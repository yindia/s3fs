package datanode

import (
	v1 "s3fs/pkg/gen/cloud/v1"

	"connectrpc.com/connect"
)

//go:generate mockery --output=./mocks --case=underscore --all --with-expecter
type DataNodeService interface {
	Ping(req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error)                  // Method for Ping
	Heartbeat(req *connect.Request[v1.HeartbeatRequest]) (*connect.Response[v1.HeartbeatResponse], error)   // Updated method for Heartbeat
	PutData(req *connect.Request[v1.DataNodePutRequest]) (*connect.Response[v1.DataNodeWriteStatus], error) // Updated method for PutData
	GetData(req *connect.Request[v1.DataNodeGetRequest]) (*connect.Response[v1.DataNodeData], error)        // Updated method for GetData
}
