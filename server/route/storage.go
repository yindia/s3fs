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

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
)

// Storage represents the S3 file system service.
type Storage struct {
	validator     *protovalidate.Validator
	dataDirectory string
	cache         cache.Cache
	filesystem    filesystem.Filesystem
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
			break
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

	// Create metadata for the uploaded file
	metadata := v1.FileMetadata{
		ObjectKey: objectKey,
		FileSize:  fileSize,
		Extension: filepath.Ext(objectKey),
	}

	if err := s.updateBlockIdsCache(metadata); err != nil {
		log.Println("Error caching file metadata:", err)
	}

	if err := stream.Err(); err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
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
			Status: true,
		}), nil
	}

	if err := s.filesystem.DeleteFile(fullPath); err != nil {
		return nil, s.logError("Failed to delete file", err)
	}

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

	cachedMetadata, _ := s.cache.GetAll()
	var fileMetadataList []*v1.FileMetadata

	for _, md := range cachedMetadata {
		var mdData v1.FileMetadata
		// Convert cachedMetadata from []byte to FileMetadata
		if err := json.Unmarshal(md, &mdData); err != nil {
			log.Println("Error unmarshalling cached metadata:", err)
			return nil, err
		}

		fileMetadataList = append(fileMetadataList, &mdData)
	}

	// Ensure the directory exists
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
func (s *Storage) updateBlockIdsCache(metadata v1.FileMetadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return s.logError("Error marshalling metadata", err)
	}
	return s.cache.Set(metadata.ObjectKey, metadataBytes)
}

// logError centralizes error logging for better maintainability.
func (s *Storage) logError(message string, err error) error {
	log.Println(message, err)
	return err
}

// updateBlockIdsCacheAfterDelete updates the BlockIds cache after a delete operation.
func (s *Storage) updateBlockIdsCacheAfterDelete(deletedBlockId string) error {
	return s.cache.Delete(deletedBlockId)
}
