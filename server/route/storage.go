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

const BlockIdsCacheKey = "blockIds"

// Storage represents the S3 file system service.
type Storage struct {
	validator     *protovalidate.Validator
	DataDirectory string
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
		DataDirectory: filepath.Base(dir),
		cache:         cache.NewCache("memory", nil),
		filesystem:    &filesystem.FilesystemImpl{},
	}
}

// Upload handles the file upload logic.
func (s *Storage) Upload(ctx context.Context, req *connect.Request[v1.UploadRequestMsg]) (*connect.Response[v1.UploadResponseMsg], error) {
	log.Println("Debug: Starting Upload method with BlockId:", req.Msg.ObjectKey)

	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, s.logError("Validation error", err)
	}

	fullPath := filepath.Join(s.DataDirectory, req.Msg.ObjectKey)

	if err := s.uploadFile(fullPath, req.Msg.Data); err != nil {
		return nil, err
	}

	if err := s.updateBlockIdsCache(req.Msg.ObjectKey); err != nil {
		log.Println("Error caching BlockIds:", err)
	}

	return connect.NewResponse(&v1.UploadResponseMsg{
		Status: true,
	}), nil
}

func (s *Storage) Get(ctx context.Context, req *connect.Request[v1.GetObjectRequest]) (*connect.Response[v1.GetObjectResponse], error) {
	log.Println("Debug: Starting GetData method")
	log.Println("GetData called with BlockId:", req.Msg.ObjectKey)

	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}

	fullPath := filepath.Join(s.DataDirectory, req.Msg.ObjectKey)
	data, err := s.filesystem.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	log.Println("Data retrieved for BlockId:", req.Msg.ObjectKey)
	return connect.NewResponse(&v1.GetObjectResponse{
		Data: []byte(data),
	}), nil
}

func (s *Storage) Delete(ctx context.Context, req *connect.Request[v1.DeleteRequestMsg]) (*connect.Response[v1.DeleteStatusMsg], error) {
	log.Println("Debug: Starting Delete method with BlockId:", req.Msg.ObjectKey)

	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, s.logError("Validation error", err)
	}

	fullPath := filepath.Join(s.DataDirectory, req.Msg.ObjectKey)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Println("File does not exist, cannot delete:", fullPath)
		return connect.NewResponse(&v1.DeleteStatusMsg{
			Status: false,
		}), nil
	}

	if err := s.filesystem.DeleteFile(fullPath); err != nil {
		return nil, err
	}

	if err := s.updateBlockIdsCacheAfterDelete(req.Msg.ObjectKey); err != nil {
		log.Println("Error updating BlockIds cache after delete:", err)
	}

	log.Println("File deleted successfully:", fullPath)

	return connect.NewResponse(&v1.DeleteStatusMsg{
		Status: true,
	}), nil
}

func (s *Storage) List(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error) {
	log.Println("Debug: Starting ListData method")

	// Check if BlockIds are cached
	cachedBlockIds, exists := s.cache.Get("blockIds") // Use a key for BlockIds
	var blockIds []string

	if exists {
		// Convert cachedBlockIds from []byte to []string
		if err := json.Unmarshal(cachedBlockIds, &blockIds); err != nil {
			log.Println("Error unmarshalling cached BlockIds:", err)
			return nil, err
		}
		log.Println("Returning cached BlockIds")
		return connect.NewResponse(&v1.ListObjectsResponse{
			ObjectKeys: blockIds, // Return the cached BlockIds
		}), nil
	}
	// Ensure the directory exists
	if _, err := os.Stat(s.DataDirectory); os.IsNotExist(err) {
		log.Println("Data directory does not exist:", s.DataDirectory)
		return connect.NewResponse(&v1.ListObjectsResponse{
			ObjectKeys: []string{},
		}), nil
	}

	// List files using the filesystem package
	blockIds, err := s.filesystem.ListFiles(s.DataDirectory)
	if err != nil {
		return nil, err
	}

	log.Println("List of BlockIds retrieved:", blockIds)
	return connect.NewResponse(&v1.ListObjectsResponse{
		ObjectKeys: blockIds,
	}), nil
}

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

// updateBlockIdsCache updates the BlockIds cache with the new BlockId.
func (s *Storage) updateBlockIdsCache(newBlockId string) error {
	cachedBlockIds, exists := s.cache.Get(BlockIdsCacheKey)
	var blockIds []string

	if exists {
		if err := json.Unmarshal(cachedBlockIds, &blockIds); err != nil {
			return s.logError("Error unmarshalling cached BlockIds", err)
		}
	}

	blockIds = append(blockIds, newBlockId)

	blockIdsBytes, err := json.Marshal(blockIds)
	if err != nil {
		return s.logError("Error marshalling BlockIds", err)
	}

	return s.cache.Set(BlockIdsCacheKey, blockIdsBytes)
}

// logError centralizes error logging for better maintainability.
func (s *Storage) logError(message string, err error) error {
	log.Println(message, err)
	return err
}

// updateBlockIdsCacheAfterDelete updates the BlockIds cache after a delete operation.
func (s *Storage) updateBlockIdsCacheAfterDelete(deletedBlockId string) error {
	cachedBlockIds, exists := s.cache.Get(BlockIdsCacheKey)
	var blockIds []string

	if exists {
		if err := json.Unmarshal(cachedBlockIds, &blockIds); err != nil {
			return s.logError("Error unmarshalling cached BlockIds", err)
		}
	}

	// Remove the BlockId from the cached list
	for i, blockId := range blockIds {
		if blockId == deletedBlockId {
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
