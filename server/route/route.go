package route

import (
	"context"
	v1 "s3fs/pkg/gen/cloud/v1"

	"connectrpc.com/connect"
)

// StorageService defines the methods for interacting with the storage system.
//
//go:generate mockery --output=./mocks --case=underscore --all --with-expecter
type StorageService interface {
	// Heartbeat checks the health of the storage service.
	Heartbeat(ctx context.Context, req *connect.Request[v1.HeartbeatRequestMsg]) (*connect.Response[v1.HeartbeatResponseMsg], error)

	// Upload uploads a file to the storage service.
	Upload(ctx context.Context, req *connect.Request[v1.UploadRequestMsg]) (*connect.Response[v1.UploadResponseMsg], error)

	// Get retrieves an object from the storage service.
	Get(ctx context.Context, req *connect.Request[v1.GetObjectRequest]) (*connect.Response[v1.GetObjectResponse], error)

	// Delete removes an object from the storage service.
	Delete(ctx context.Context, req *connect.Request[v1.DeleteRequestMsg]) (*connect.Response[v1.DeleteStatusMsg], error)

	// List retrieves a list of objects from the storage service.
	List(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error)
}
