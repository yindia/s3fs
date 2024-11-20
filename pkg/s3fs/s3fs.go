package s3fs

import (
	"context"
	"log"
	"math/rand"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
)

type DataNodeInstance struct {
	Host        string
	ServicePort string
}

type S3fs struct {
	validator          *protovalidate.Validator
	Port               uint16
	BlockSize          uint64
	ReplicationFactor  uint64
	IdToDataNodes      map[uint64]DataNodeInstance
	FileNameToBlocks   map[string][]string
	BlockToDataNodeIds map[string][]uint64
}

func NewS3FSServer(blockSize uint64, replicationFactor uint64, serverPort uint16) cloudv1connect.StorageServiceHandler {
	validator, err := protovalidate.New()
	if err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
	}

	return &S3fs{
		validator:          validator,
		Port:               serverPort,
		BlockSize:          blockSize,
		ReplicationFactor:  replicationFactor,
		FileNameToBlocks:   make(map[string][]string),
		IdToDataNodes:      make(map[uint64]DataNodeInstance),
		BlockToDataNodeIds: make(map[string][]uint64),
	}
}

func (s *S3fs) Upload(ctx context.Context, req *connect.Request[v1.UploadRequest]) (*connect.Response[v1.UploadResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *S3fs) Download(ctx context.Context, req *connect.Request[v1.DownloadRequest]) (*connect.Response[v1.DownloadResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *S3fs) Delete(ctx context.Context, req *connect.Request[v1.DeleteRequest]) (*connect.Response[v1.DeleteResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *S3fs) ListObjects(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *S3fs) Ping(ctx context.Context, req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	// Implement the logic for Ping
	return connect.NewResponse(&v1.PingResponse{}), nil
}

func (s *S3fs) Heartbeat(ctx context.Context, req *connect.Request[v1.HeartbeatRequest]) (*connect.Response[v1.HeartbeatResponse], error) {
	// Implement the logic for Heartbeat
	return connect.NewResponse(&v1.HeartbeatResponse{}), nil
}

func selectRandomNumbers(availableItems []uint64, count uint64) (randomNumberSet []uint64) {
	numberPresentMap := make(map[uint64]bool)
	for i := uint64(0); i < count; {
		chosenItem := availableItems[rand.Intn(len(availableItems))]
		if _, ok := numberPresentMap[chosenItem]; !ok {
			numberPresentMap[chosenItem] = true
			randomNumberSet = append(randomNumberSet, chosenItem)
			i++
		}
	}
	return
}
