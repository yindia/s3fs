package s3fs

import (
	"context"
	v1 "s3fs/pkg/gen/cloud/v1"

	"connectrpc.com/connect"
)

//go:generate mockery --output=./mocks --case=underscore --all --with-expecter
type StorageServiceHandler interface {
	Upload(ctx context.Context, req *connect.Request[v1.UploadRequest]) (*v1.UploadResponse, error)
	Delete(ctx context.Context, req *connect.Request[v1.DeleteRequest]) (*connect.Response[v1.DeleteResponse], error)
	ListObjects(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error)
	Ping(ctx context.Context, req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error)
	Heartbeat(ctx context.Context, req *connect.Request[v1.HeartbeatRequest]) (*connect.Response[v1.HeartbeatResponse], error)
}
