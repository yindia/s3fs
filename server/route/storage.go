package route

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"s3fs/pkg/cache"
	"s3fs/pkg/filesystem"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"
	"sync"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
)

const BlockIdsCacheKey = "blockIds"

// FileMetadata holds information about a file in storage.
type FileMetadata struct {
	ObjectKey string // The key of the object
	FileSize  uint32 // The size of the file
	Extension string // The file extension
}

// Storage represents the S3 file system service.
type Storage struct {
	validator     *protovalidate.Validator
	dataDirectory string
	cache         cache.Cache
	filesystem    filesystem.Filesystem
	mu            sync.Mutex // Mutex for thread safety
}

// NewStorage initializes a new S3fs server.
func NewStorage(dir string) cloudv1connect.StorageServiceHandler {
	// Ensure the directory exists, create it if it doesn't
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	validator, err := protovalidate.New()
	if err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
	}

	return &Storage{
		validator:     validator,
		dataDirectory: filepath.Base(dir),
		cache:         cache.NewCache("memory", nil),
		filesystem:    &filesystem.FilesystemImpl{},
	}
}

// Upload handles the file upload logic.
func (s *Storage) Upload(ctx context.Context, stream *connect.ClientStream[v1.UploadRequestMsg]) (*connect.Response[v1.UploadResponseMsg], error) {
	log.Println("Debug: Starting Upload method")

	var objectKey string
	var fileSize uint32
	var file *os.File

	// Create a new file before receiving chunks
	for stream.Receive() {
		req := stream.Msg()
		if err := s.validator.Validate(req); err != nil {
			return nil, s.logError("Validation error", err)
		}
		objectKey = req.ObjectKey
		data := req.GetData()

		// Handle blank file (no data)
		if len(data) == 0 {
			log.Printf("Received blank file for object key: %s", objectKey)
			fullPath := filepath.Join(s.dataDirectory, objectKey)
			if err := os.WriteFile(fullPath, []byte{}, 0644); err != nil {
				return nil, s.logError("Failed to create blank file", err)
			}
			break // Exit the loop for blank file
		}

		fileSize += uint32(len(data))
		fullPath := filepath.Join(s.dataDirectory, objectKey)

		// Initialize file only once
		if file == nil {
			var err error
			file, err = os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, 0644) // Open file for writing
			if err != nil {
				return nil, s.logError("Failed to open file for writing", err)
			}
		}

		// Write the chunk to the file
		if _, err := file.Write(data); err != nil {
			return nil, s.logError("Failed to write data to file", err)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	// Locking to prevent race conditions when updating cache
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create metadata for the uploaded file
	metadata := FileMetadata{
		ObjectKey: objectKey,
		FileSize:  fileSize,
		Extension: filepath.Ext(objectKey), // Store the file extension
	}

	if err := s.updateBlockIdsCache(metadata); err != nil {
		log.Println("Error caching file metadata:", err)
	}

	return connect.NewResponse(&v1.UploadResponseMsg{
		Status: true,
	}), nil
}

// Get retrieves the file data for the specified object key.
func (s *Storage) Get(ctx context.Context, req *connect.Request[v1.GetObjectRequest], stream *connect.ServerStream[v1.GetObjectResponse]) error {
	log.Println("Debug: Starting Get method")
	log.Printf("Get called with ObjectKey: %s", req.Msg.ObjectKey)

	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return err
	}

	fullPath := filepath.Join(s.dataDirectory, req.Msg.ObjectKey)
	data, err := s.filesystem.ReadFile(fullPath)
	if err != nil {
		return s.logError("Failed to read file", err)
	}

	chunkSize := 1024
	for offset := 0; offset < len(data); offset += chunkSize {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&v1.GetObjectResponse{
			Data: data[offset:end],
		}); err != nil {
			return s.logError("Failed to send data chunk", err)
		}
	}
	log.Printf("Data retrieved for ObjectKey: %s", req.Msg.ObjectKey)
	return nil
}

// Delete removes the specified object from storage.
func (s *Storage) Delete(ctx context.Context, req *connect.Request[v1.DeleteRequestMsg]) (*connect.Response[v1.DeleteStatusMsg], error) {
	log.Printf("Debug: Starting Delete method with ObjectKey: %s", req.Msg.ObjectKey)

	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, s.logError("Validation error", err)
	}

	fullPath := filepath.Join(s.dataDirectory, req.Msg.ObjectKey)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Printf("File does not exist, cannot delete: %s", fullPath)
		return connect.NewResponse(&v1.DeleteStatusMsg{
			Status: false,
		}), nil
	}

	if err := s.filesystem.DeleteFile(fullPath); err != nil {
		return nil, s.logError("Failed to delete file", err)
	}

	// Locking to prevent race conditions when updating cache
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.updateBlockIdsCacheAfterDelete(req.Msg.ObjectKey); err != nil {
		log.Println("Error updating BlockIds cache after delete:", err)
	}

	log.Printf("File deleted successfully: %s", fullPath)

	return connect.NewResponse(&v1.DeleteStatusMsg{
		Status: true,
	}), nil
}

// List retrieves a list of object keys and their metadata stored in the directory.
func (s *Storage) List(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error) {
	log.Println("Debug: Starting List method")

	// Check if BlockIds are cached
	cachedMetadata, exists := s.cache.Get(BlockIdsCacheKey) // Use a key for BlockIds
	var fileMetadataList []*v1.FileMetadata

	if exists {
		// Convert cachedMetadata from []byte to []FileMetadata
		if err := json.Unmarshal(cachedMetadata, &fileMetadataList); err != nil {
			log.Println("Error unmarshalling cached metadata:", err)
			return nil, err
		}
		log.Println("Returning cached metadata")
		return connect.NewResponse(&v1.ListObjectsResponse{
			Metadata: fileMetadataList, // Return the cached metadata
		}), nil
	}
	// Ensure the directory exists
	if _, err := os.Stat(s.dataDirectory); os.IsNotExist(err) {
		log.Printf("Data directory does not exist: %s", s.dataDirectory)
		return connect.NewResponse(&v1.ListObjectsResponse{
			Metadata: []*v1.FileMetadata{}, // Return empty metadata
		}), nil
	}

	// List files using the filesystem package
	blockIds, err := s.filesystem.ListFiles(s.dataDirectory)
	if err != nil {
		return nil, err
	}

	// Create metadata for each blockId
	for _, blockId := range blockIds {
		metadata := &v1.FileMetadata{
			ObjectKey: blockId,
			Extension: filepath.Ext(blockId), // Extracting the file extension from blockId
		}
		fileMetadataList = append(fileMetadataList, metadata)
	}

	log.Printf("List of BlockIds retrieved: %v", blockIds)
	return connect.NewResponse(&v1.ListObjectsResponse{
		Metadata: fileMetadataList,
	}), nil
}

// Heartbeat checks the health of the service.
func (s *Storage) Heartbeat(ctx context.Context, req *connect.Request[v1.HeartbeatRequestMsg]) (*connect.Response[v1.HeartbeatResponseMsg], error) {
	log.Println("Debug: Starting Heartbeat method")
	log.Println("Heartbeat called")
	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}
	return connect.NewResponse(&v1.HeartbeatResponseMsg{
		Status: "ok",
	}), nil
}

// uploadFile handles the file upload logic.
func (s *Storage) uploadFile(fullPath string, data []byte) error {
	if err := s.filesystem.EnsureDirectory(filepath.Dir(fullPath)); err != nil {
		return err
	}
	return s.filesystem.WriteFile(fullPath, string(data))
}

// updateBlockIdsCache updates the BlockIds cache to store metadata instead of just object keys
func (s *Storage) updateBlockIdsCache(metadata FileMetadata) error {
	s.mu.Lock() // Locking to prevent race conditions
	defer s.mu.Unlock()

	cachedMetadata, exists := s.cache.Get(BlockIdsCacheKey)
	var fileMetadataList []FileMetadata

	if exists {
		if err := json.Unmarshal(cachedMetadata, &fileMetadataList); err != nil {
			return s.logError("Error unmarshalling cached metadata", err)
		}
	}

	fileMetadataList = append(fileMetadataList, metadata)

	metadataBytes, err := json.Marshal(fileMetadataList)
	if err != nil {
		return s.logError("Error marshalling metadata", err)
	}

	return s.cache.Set(BlockIdsCacheKey, metadataBytes)
}

// logError centralizes error logging for better maintainability.
func (s *Storage) logError(message string, err error) error {
	log.Println(message, err)
	return err
}

// updateBlockIdsCacheAfterDelete updates the BlockIds cache after a delete operation.
func (s *Storage) updateBlockIdsCacheAfterDelete(deletedBlockId string) error {
	s.mu.Lock() // Locking to prevent race conditions
	defer s.mu.Unlock()

	cachedBlockIds, exists := s.cache.Get(BlockIdsCacheKey)
	var blockIds []FileMetadata

	if exists {
		if err := json.Unmarshal(cachedBlockIds, &blockIds); err != nil {
			return s.logError("Error unmarshalling cached BlockIds", err)
		}
	}

	// Remove the BlockId from the cached list
	for i, blockId := range blockIds {
		if blockId.ObjectKey == deletedBlockId {
			blockIds = append(blockIds[:i], blockIds[i+1:]...) // Remove the BlockId
			break
		}
	}

	blockIdsBytes, err := json.Marshal(blockIds)
	if err != nil {
		return s.logError("Error marshalling BlockIds", err)
	}

	return s.cache.Set(BlockIdsCacheKey, blockIdsBytes)
}
