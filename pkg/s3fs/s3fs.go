package s3fs

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
	"github.com/google/uuid"
)

type NameNodeMetaData struct {
	BlockId        string
	BlockAddresses []DataNodeInstance
}

type DataNodeInstance struct {
	Host        string
	ServicePort int
}

type UnderReplicatedBlocks struct {
	BlockId           string
	HealthyDataNodeId uint64
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

func NewS3FSServer(blockSize uint64, replicationFactor uint64, serverPort uint16) *S3fs {
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

func NewS3FSServerHandler(blockSize uint64, replicationFactor uint64, serverPort uint16) cloudv1connect.StorageServiceHandler {
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

func (s *S3fs) GetIdToDataNodes() map[uint64]DataNodeInstance {
	return s.IdToDataNodes
}

func (s *S3fs) DeleteIdToDataNodes(id uint64) {
	delete(s.IdToDataNodes, id)
}

func (s *S3fs) Upload(ctx context.Context, req *connect.Request[v1.UploadRequest]) (*connect.Response[v1.UploadResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	s.FileNameToBlocks[req.Msg.ObjectKey] = []string{}

	numberOfBlocksToAllocate := uint64(math.Ceil(float64(req.Msg.FileSize) / float64(s.BlockSize)))
	md := s.allocateBlocks(req.Msg.ObjectKey, numberOfBlocksToAllocate)
	for _, metadata := range md {
		node := metadata.BlockAddresses[0]
		replicateNode := metadata.BlockAddresses[1:]
		replicationNodes := []*v1.NodeAddress{}
		for _, data := range replicateNode {
			replicationNodes = append(replicationNodes, &v1.NodeAddress{
				Host:        data.Host,
				ServicePort: uint32(data.ServicePort),
			})
		}

		client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", node.Host, node.ServicePort))

		response, err := client.PutData(context.Background(), connect.NewRequest(&v1.DataNodePutRequest{
			BlockId:          metadata.BlockId,
			Data:             string(req.Msg.Data),
			ReplicationNodes: replicationNodes,
		}))
		if err != nil {
			continue
		}
		fmt.Println(response.Msg.Status)
	}

	return connect.NewResponse(&v1.UploadResponse{
		Message: "ok",
	}), nil
}

func (s *S3fs) ListObjects(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error) {
	if err := s.validator.Validate(req.Msg); err != nil {
		return nil, err
	}
	result := v1.ListObjectsResponse{}
	for file_name, _ := range s.FileNameToBlocks {
		result.ObjectKeys = append(result.ObjectKeys, file_name)

	}
	return connect.NewResponse(&result), nil
}

func (s *S3fs) SetIdToDataNodes(node DataNodeInstance, i uint64) {
	s.IdToDataNodes[uint64(i)] = node
}

func (s *S3fs) allocateBlocks(fileName string, numberOfBlocks uint64) (metadata []NameNodeMetaData) {
	s.FileNameToBlocks[fileName] = []string{}
	var dataNodesAvailable []uint64
	for k, _ := range s.IdToDataNodes {
		dataNodesAvailable = append(dataNodesAvailable, k)
	}
	dataNodesAvailableCount := uint64(len(dataNodesAvailable))

	for i := uint64(0); i < numberOfBlocks; i++ {
		blockId := uuid.New().String()
		s.FileNameToBlocks[fileName] = append(s.FileNameToBlocks[fileName], blockId)

		var blockAddresses []DataNodeInstance
		var replicationFactor uint64
		if s.ReplicationFactor > dataNodesAvailableCount {
			replicationFactor = dataNodesAvailableCount
		} else {
			replicationFactor = s.ReplicationFactor
		}

		targetDataNodeIds := s.assignDataNodes(blockId, dataNodesAvailable, replicationFactor)
		for _, dataNodeId := range targetDataNodeIds {
			blockAddresses = append(blockAddresses, s.IdToDataNodes[dataNodeId])
		}

		metadata = append(metadata, NameNodeMetaData{BlockId: blockId, BlockAddresses: blockAddresses})
	}
	return
}

func (s *S3fs) assignDataNodes(blockId string, dataNodesAvailable []uint64, replicationFactor uint64) []uint64 {
	targetDataNodeIds := selectRandomNumbers(dataNodesAvailable, replicationFactor)
	s.BlockToDataNodeIds[blockId] = targetDataNodeIds
	return targetDataNodeIds
}

func (s *S3fs) GetBlockSize(request bool, reply *uint64) error {
	if request {
		*reply = s.BlockSize
	}
	return nil
}

func (s *S3fs) ReDistribute(ctx context.Context, req *connect.Request[v1.ReDistributeRequest]) (*connect.Response[v1.ReDistributeResponse], error) {
	deadDataNodeSlice := strings.Split(req.Msg.DataNodeUri, ":")
	var deadDataNodeId uint64

	port, err := strconv.Atoi(deadDataNodeSlice[1])
	if err != nil {
		return nil, err
	}
	// de-register the dead DataNode from IdToDataNodes meta
	for id, dn := range s.IdToDataNodes {
		if dn.Host == deadDataNodeSlice[0] && dn.ServicePort == port {
			deadDataNodeId = id
			break
		}
	}
	delete(s.IdToDataNodes, deadDataNodeId)

	// construct under-replicated blocks list and
	// de-register the block entirely in favour of re-creation
	var underReplicatedBlocksList []UnderReplicatedBlocks
	for blockId, dnIds := range s.BlockToDataNodeIds {
		for i, dnId := range dnIds {
			if dnId == deadDataNodeId {
				healthyDataNodeId := s.BlockToDataNodeIds[blockId][(i+1)%len(dnIds)]
				underReplicatedBlocksList = append(
					underReplicatedBlocksList,
					UnderReplicatedBlocks{blockId, healthyDataNodeId},
				)
				delete(s.BlockToDataNodeIds, blockId)
				// TODO: trigger data deletion on the existing data nodes
				break
			}
		}
	}

	// verify if re-replication would be possible
	if len(s.IdToDataNodes) < int(s.ReplicationFactor) {
		log.Println("Replication not possible due to unavailability of sufficient DataNode(s)")
		return nil, nil
	}

	var availableNodes []uint64
	for k, _ := range s.IdToDataNodes {
		availableNodes = append(availableNodes, k)
	}

	// attempt re-replication of under-replicated blocks
	for _, blockToReplicate := range underReplicatedBlocksList {

		// fetch the data from the healthy DataNode
		healthyDataNode := s.IdToDataNodes[blockToReplicate.HealthyDataNodeId]
		client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", healthyDataNode.Host, healthyDataNode.ServicePort))

		response, err := client.GetData(context.Background(), connect.NewRequest(&v1.DataNodeGetRequest{
			BlockId: blockToReplicate.BlockId,
		}))

		if err != nil {
			continue
		}
		fmt.Println(response.Msg.Data)
		// initiate the replication of the block contents
		targetDataNodeIds := s.assignDataNodes(blockToReplicate.BlockId, availableNodes, s.ReplicationFactor)
		var blockAddresses []DataNodeInstance
		for _, dataNodeId := range targetDataNodeIds {
			blockAddresses = append(blockAddresses, s.IdToDataNodes[dataNodeId])
		}
		startingDataNode := blockAddresses[0]
		remainingDataNodes := blockAddresses[1:]

		replicationNodes := []*v1.NodeAddress{}
		for _, node := range remainingDataNodes {
			replicationNodes = append(replicationNodes, &v1.NodeAddress{
				Host:        node.Host,
				ServicePort: uint32(node.ServicePort),
			})
		}
		client = cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", startingDataNode.Host, startingDataNode.ServicePort))

		result, err := client.PutData(context.Background(), connect.NewRequest(&v1.DataNodePutRequest{
			BlockId:          blockToReplicate.BlockId,
			Data:             response.Msg.Data,
			ReplicationNodes: replicationNodes,
		}))
		fmt.Println(result.Msg.Status)
		log.Printf("Block %s replication completed for %+v\n", blockToReplicate.BlockId, targetDataNodeIds)
	}

	return connect.NewResponse(&v1.ReDistributeResponse{}), nil
}

func (s *S3fs) Get(ctx context.Context, req *connect.Request[v1.GetRequest]) (*connect.Response[v1.GetResponse], error) {
	fileBlocks := s.FileNameToBlocks[req.Msg.ObjectKey]

	for _, block := range fileBlocks {
		targetDataNodeIds := s.BlockToDataNodeIds[block]
		for _, dataNodeId := range targetDataNodeIds {
			client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", s.IdToDataNodes[dataNodeId].Host, s.IdToDataNodes[dataNodeId].ServicePort))

			result, err := client.GetData(context.Background(), connect.NewRequest(&v1.DataNodeGetRequest{
				BlockId: block,
			}))
			if err != nil {
				continue
			}
			return connect.NewResponse(&v1.GetResponse{Data: []byte(result.Msg.Data)}), nil
		}
	}

	return connect.NewResponse(&v1.GetResponse{Data: []byte("")}), nil
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
