package datanode

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/bufbuild/protovalidate-go"
)

type S3fs struct {
	validator     *protovalidate.Validator
	DataDirectory string
	Host          string
	Port          uint16
}

func NewDataNodeServer(dir string, port uint16, host string) cloudv1connect.DataNodeServiceHandler {
	validator, err := protovalidate.New()
	if err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
	}

	return &S3fs{
		validator:     validator,
		DataDirectory: filepath.Base(dir),
		Host:          host,
		Port:          port,
	}
}

func (s *S3fs) PutData(ctx context.Context, req *connect.Request[v1.DataNodePutRequest]) (*connect.Response[v1.DataNodeWriteStatus], error) {
	log.Println("Debug: Starting PutData method")
	log.Println("PutData called with BlockId:", req.Msg.BlockId)

	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}

	// Ensure the directory exists
	fullPath := filepath.Join(s.DataDirectory, req.Msg.BlockId)
	if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
		log.Println("Error creating directory:", err)
		return nil, err
	}

	fileWriteHandler, err := os.Create(fullPath)
	if err != nil {
		log.Println("Error creating file:", err)
		return nil, err
	}
	defer fileWriteHandler.Close()

	// Log the data being written
	log.Printf("Writing data to file: %s\n", req.Msg.Data)

	_, err = fileWriteHandler.WriteString(req.Msg.Data)
	if err != nil {
		log.Println("Error writing to file:", err)
		return nil, err
	}
	fileWriteHandler.Sync() // Ensure data is flushed to disk

	log.Println("Forwarding for replication for BlockId:", req.Msg.BlockId)
	s.forwardForReplication(req)

	return connect.NewResponse(&v1.DataNodeWriteStatus{
		Status: true,
	}), nil
}

func (s *S3fs) GetData(ctx context.Context, req *connect.Request[v1.DataNodeGetRequest]) (*connect.Response[v1.DataNodeData], error) {
	log.Println("Debug: Starting GetData method")
	log.Println("GetData called with BlockId:", req.Msg.BlockId)
	log.Printf("Request Message: %+v\n", req.Msg)

	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}
	fullPath := filepath.Join(s.DataDirectory, req.Msg.BlockId)
	dataBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Println("Error reading file:", err)
		return nil, err
	}
	log.Println("Data retrieved for BlockId:", req.Msg.BlockId)
	return connect.NewResponse(&v1.DataNodeData{
		Data: string(dataBytes),
	}), nil
}

func (s *S3fs) DeleteData(ctx context.Context, req *connect.Request[v1.DataNodeDeleteRequest]) (*connect.Response[v1.DataNodeDeleteStatus], error) {
	log.Println("Debug: Starting DeleteData method")
	log.Println("DeleteData called with BlockId:", req.Msg.BlockId)

	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}
	fullPath := filepath.Join(s.DataDirectory, req.Msg.BlockId)

	// Check if the file exists before attempting to delete
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Println("File does not exist, cannot delete:", fullPath)
		return connect.NewResponse(&v1.DataNodeDeleteStatus{
			Status: false,
		}), nil
	}

	// Delete the file after reading
	if err := os.Remove(fullPath); err != nil {
		log.Println("Error deleting file:", err)
		return nil, err
	}
	s.forwardForReplicationDelete(req)
	log.Println("File deleted successfully:", fullPath)

	return connect.NewResponse(&v1.DataNodeDeleteStatus{
		Status: true,
	}), nil
}

func (s *S3fs) Ping(ctx context.Context, req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	log.Println("Debug: Starting Ping method")
	log.Println("Ping called from Host:", req.Msg.Host)
	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}
	s.Host = req.Msg.Host
	s.Port = uint16(req.Msg.Port)
	log.Println("Ping updated Host and Port:", s.Host, s.Port)
	return connect.NewResponse(&v1.PingResponse{
		Ack: true,
	}), nil
}

func (s *S3fs) Heartbeat(ctx context.Context, req *connect.Request[v1.HeartbeatRequest]) (*connect.Response[v1.HeartbeatResponse], error) {
	log.Println("Debug: Starting Heartbeat method")
	log.Println("Heartbeat called")
	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
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

	client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", startingDataNode.Host, startingDataNode.ServicePort), connect.WithInterceptors(interceptor))

	res, err := client.PutData(context.Background(), connect.NewRequest(&v1.DataNodePutRequest{
		BlockId:          blockId,
		Data:             request.Msg.Data,
		ReplicationNodes: remainingDataNodes,
	}))
	if err != nil {

		return err
	}
	fmt.Println(res.Msg.Status)

	log.Println("Debug: Forwarding for replication with BlockId:", blockId)

	return nil
}

func (s *S3fs) forwardForReplicationDelete(request *connect.Request[v1.DataNodeDeleteRequest]) error {
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

	client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", startingDataNode.Host, startingDataNode.ServicePort), connect.WithInterceptors(interceptor))

	res, err := client.DeleteData(context.Background(), connect.NewRequest(&v1.DataNodeDeleteRequest{
		BlockId:          blockId,
		ReplicationNodes: remainingDataNodes,
	}))
	if err != nil {

		return err
	}
	fmt.Println(res.Msg.Status)

	log.Println("Debug: Forwarding for deletion with BlockId:", blockId)

	return nil
}
