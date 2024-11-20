package datanode

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"
	"strconv"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/bufbuild/protovalidate-go"
)

type S3fs struct {
	validator     *protovalidate.Validator
	DataDirectory string
	ServicePort   uint16
	Host          string
	Port          uint16
}

func NewDataNodeServer() cloudv1connect.DataNodeServiceHandler {
	validator, err := protovalidate.New()
	if err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
	}

	return &S3fs{
		validator: validator,
	}
}

func (s *S3fs) PutData(ctx context.Context, req *connect.Request[v1.DataNodePutRequest]) (*connect.Response[v1.DataNodeWriteStatus], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}

	fileWriteHandler, err := os.Create(s.DataDirectory + req.Msg.BlockId)
	if err != nil {
		return nil, err
	}
	defer fileWriteHandler.Close()

	fileWriter := bufio.NewWriter(fileWriteHandler)
	_, err = fileWriter.WriteString(req.Msg.Data)
	if err != nil {
		return nil, err
	}
	fileWriter.Flush()

	s.forwardForReplication(req)

	return connect.NewResponse(&v1.DataNodeWriteStatus{
		Status: true,
	}), nil
}

func (s *S3fs) GetData(ctx context.Context, req *connect.Request[v1.DataNodeGetRequest]) (*connect.Response[v1.DataNodeData], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}

	dataBytes, err := ioutil.ReadFile(s.DataDirectory + req.Msg.BlockId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.DataNodeData{
		Data: string(dataBytes),
	}), nil
}

func (s *S3fs) Ping(ctx context.Context, req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	s.Host = req.Msg.Host
	port, err := strconv.ParseUint(req.Msg.Port, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %v", err)
	}
	s.Port = uint16(port)
	return connect.NewResponse(&v1.PingResponse{
		Ack: true,
	}), nil
}

func (s *S3fs) Heartbeat(ctx context.Context, req *connect.Request[v1.HeartbeatRequest]) (*connect.Response[v1.HeartbeatResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.HeartbeatResponse{
		Status: "ok",
	}), nil
}

func (s *S3fs) forwardForReplication(request *connect.Request[v1.DataNodePutRequest]) error {
	blockId := request.Msg.BlockId
	blockAddresses := request.Msg.ReplicationNodes

	if len(blockAddresses) == 0 {
		return nil
	}

	startingDataNode := blockAddresses[0]
	remainingDataNodes := blockAddresses[1:]

	interceptor, err := otelconnect.NewInterceptor()
	if err != nil {
		return fmt.Errorf("error creating interceptor: %w", err)
	}

	client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%s", startingDataNode.Host, startingDataNode.ServicePort), connect.WithInterceptors(interceptor))
	payloadRequest := connect.NewRequest(&v1.DataNodePutRequest{
		BlockId:          blockId,
		Data:             request.Msg.Data,
		ReplicationNodes: remainingDataNodes,
	})

	_, err = client.PutData(context.Background(), payloadRequest)
	if err != nil {
		return err
	}
	return nil
}
