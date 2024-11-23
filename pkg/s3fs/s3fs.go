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
	log.Println("Entering NewS3FSServer")
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
	log.Println("Entering NewS3FSServerHandler")
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
	log.Println("Entering GetIdToDataNodes")
	defer log.Println("Exiting GetIdToDataNodes")
	return s.IdToDataNodes
}

func (s *S3fs) DeleteIdToDataNodes(id uint64) {
	log.Printf("Entering DeleteIdToDataNodes with id: %d", id)
	defer log.Println("Exiting DeleteIdToDataNodes")
	delete(s.IdToDataNodes, id)
}

func (s *S3fs) Upload(ctx context.Context, req *connect.Request[v1.UploadRequest]) (*connect.Response[v1.UploadResponse], error) {
	log.Println("Entering Upload")
	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}
	s.FileNameToBlocks[req.Msg.ObjectKey] = []string{}

	numberOfBlocksToAllocate := uint64(math.Ceil(float64(req.Msg.FileSize) / float64(s.BlockSize)))
	md := s.allocateBlocks(req.Msg.ObjectKey, numberOfBlocksToAllocate)
	for _, metadata := range md {
		log.Println("Uploading block with ID:", metadata.BlockId)
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
	log.Println("Exiting Upload")
	return connect.NewResponse(&v1.UploadResponse{
		Message: "ok",
	}), nil
}

func (s *S3fs) ListObjects(ctx context.Context, req *connect.Request[v1.ListObjectsRequest]) (*connect.Response[v1.ListObjectsResponse], error) {
	log.Println("Entering ListObjects")
	if err := s.validator.Validate(req.Msg); err != nil {
		log.Println("Validation error:", err)
		return nil, err
	}
	result := v1.ListObjectsResponse{}
	for file_name, _ := range s.FileNameToBlocks {
		result.ObjectKeys = append(result.ObjectKeys, file_name)

	}
	log.Println("ListObjects response prepared with ObjectKeys:", result.ObjectKeys)
	log.Println("Exiting ListObjects")
	return connect.NewResponse(&result), nil
}

func (s *S3fs) SetIdToDataNodes(node DataNodeInstance, i uint64) {
	log.Printf("Entering SetIdToDataNodes with node: %+v, index: %d", node, i)
	defer log.Println("Exiting SetIdToDataNodes")
	s.IdToDataNodes[uint64(i)] = node
}

func (s *S3fs) allocateBlocks(fileName string, numberOfBlocks uint64) (metadata []NameNodeMetaData) {
	log.Printf("Entering allocateBlocks with fileName: %s, numberOfBlocks: %d", fileName, numberOfBlocks)
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
	log.Println("Exiting allocateBlocks")
	return
}

func (s *S3fs) assignDataNodes(blockId string, dataNodesAvailable []uint64, replicationFactor uint64) []uint64 {
	log.Printf("Entering assignDataNodes with blockId: %s", blockId)
	targetDataNodeIds := selectRandomNumbers(dataNodesAvailable, replicationFactor)
	s.BlockToDataNodeIds[blockId] = targetDataNodeIds
	log.Println("Exiting assignDataNodes")
	return targetDataNodeIds
}

func (s *S3fs) GetBlockSize() *uint64 {
	log.Println("Entering GetBlockSize")
	defer log.Println("Exiting GetBlockSize")
	return &s.BlockSize
}

func (s *S3fs) ReDistribute(ctx context.Context, req *connect.Request[v1.ReDistributeRequest]) (*connect.Response[v1.ReDistributeResponse], error) {
	log.Println("Entering ReDistribute")
	log.Println("ReDistribute request received for DataNodeUri:", req.Msg.DataNodeUri)
	deadDataNodeSlice := strings.Split(req.Msg.DataNodeUri, ":")
	var deadDataNodeId uint64

	// De-register the dead DataNode from IdToDataNodes meta
	for id, dn := range s.IdToDataNodes {
		if dn.Host == deadDataNodeSlice[0] && strconv.Itoa(dn.ServicePort) == deadDataNodeSlice[1] {
			deadDataNodeId = id
			break
		}
	}
	delete(s.IdToDataNodes, deadDataNodeId)

	// Construct under-replicated blocks list
	var underReplicatedBlocksList []UnderReplicatedBlocks
	for blockId, dnIds := range s.BlockToDataNodeIds {
		for _, dnId := range dnIds {
			if dnId == deadDataNodeId {
				healthyDataNodeId := s.BlockToDataNodeIds[blockId][(0)%len(dnIds)] // Get a healthy node
				underReplicatedBlocksList = append(
					underReplicatedBlocksList,
					UnderReplicatedBlocks{blockId, healthyDataNodeId},
				)
				delete(s.BlockToDataNodeIds, blockId)
				break
			}
		}
	}

	// Verify if re-replication would be possible
	if len(s.IdToDataNodes) < int(s.ReplicationFactor) {
		log.Println("Replication not possible due to unavailability of sufficient DataNode(s)")
		return nil, nil
	}

	// Attempt re-replication of under-replicated blocks
	for _, blockToReplicate := range underReplicatedBlocksList {
		// Fetch the data from the healthy DataNode
		healthyDataNode := s.IdToDataNodes[blockToReplicate.HealthyDataNodeId]
		client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", healthyDataNode.Host, healthyDataNode.ServicePort))

		response, err := client.GetData(context.Background(), connect.NewRequest(&v1.DataNodeGetRequest{
			BlockId: blockToReplicate.BlockId,
		}))
		if err != nil {
			log.Printf("Failed to get data from healthy DataNode for block %s: %v", blockToReplicate.BlockId, err)
			continue
		}
		blockContents := response.Msg.Data

		// Find a target node where the data is not available
		var targetDataNodeIds []uint64
		for id := range s.IdToDataNodes {
			if _, exists := s.BlockToDataNodeIds[blockToReplicate.BlockId]; !exists {
				targetDataNodeIds = append(targetDataNodeIds, id)
			}
		}

		if len(targetDataNodeIds) == 0 {
			log.Println("No available nodes for replication.")
			continue // Skip if no nodes are available
		}

		// Select one target node for replication
		selectedNodeId := targetDataNodeIds[0] // You can implement random selection if needed
		targetDataNode := s.IdToDataNodes[selectedNodeId]

		client = cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", targetDataNode.Host, targetDataNode.ServicePort))
		result, err := client.PutData(context.Background(), connect.NewRequest(&v1.DataNodePutRequest{
			BlockId:          blockToReplicate.BlockId,
			Data:             blockContents,
			ReplicationNodes: []*v1.NodeAddress{},
		}))
		if err != nil {
			log.Printf("Failed to replicate block %s: %v", blockToReplicate.BlockId, err)
			continue
		}
		fmt.Println(result.Msg.Status)
		log.Printf("Block %s replication completed for node %+v\n", blockToReplicate.BlockId, targetDataNodeIds)

		// After successful replication, remove blockId from FileNameToBlocks

	}

	log.Println("Exiting ReDistribute")
	return connect.NewResponse(&v1.ReDistributeResponse{}), nil
}

func (s *S3fs) Get(ctx context.Context, req *connect.Request[v1.GetRequest]) (*connect.Response[v1.GetResponse], error) {
	log.Println("Entering Get")
	fileBlocks := s.FileNameToBlocks[req.Msg.ObjectKey]
	for _, block := range fileBlocks {
		log.Printf("Getting block: %s", block)
		targetDataNodeIds := s.BlockToDataNodeIds[block]
		for _, dataNodeId := range targetDataNodeIds {
			log.Printf("Getting block: %s from node: %+v", block, s.IdToDataNodes[dataNodeId].ServicePort)
			client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", s.IdToDataNodes[dataNodeId].Host, s.IdToDataNodes[dataNodeId].ServicePort))

			result, err := client.GetData(context.Background(), connect.NewRequest(&v1.DataNodeGetRequest{
				BlockId: block,
			}))
			if err != nil {
				continue
			}
			fmt.Println(result.Msg.Data)
			return connect.NewResponse(&v1.GetResponse{Data: []byte(result.Msg.Data)}), nil
		}
	}

	log.Println("Exiting Get")
	return connect.NewResponse(&v1.GetResponse{Data: []byte("")}), nil
}

func (s *S3fs) Delete(ctx context.Context, req *connect.Request[v1.DeleteRequest]) (*connect.Response[v1.DeleteResponse], error) {
	log.Println("Entering Delete")
	fileBlocks := s.FileNameToBlocks[req.Msg.ObjectKey]

	for _, block := range fileBlocks {
		targetDataNodeIds := s.BlockToDataNodeIds[block]

		var replicationNodes []*v1.NodeAddress
		for _, dataNodeId := range targetDataNodeIds {
			replicationNodes = append(replicationNodes, &v1.NodeAddress{
				Host:        s.IdToDataNodes[dataNodeId].Host,
				ServicePort: uint32(s.IdToDataNodes[dataNodeId].ServicePort),
			})
		}
		client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", replicationNodes[0].Host, replicationNodes[0].ServicePort))

		result, err := client.DeleteData(context.Background(), connect.NewRequest(&v1.DataNodeDeleteRequest{
			BlockId:          block,
			ReplicationNodes: replicationNodes[1:],
		}))
		if err != nil {
			continue
		}
		fmt.Println(result.Msg.Status)
	}

	// Remove the file entry from FileNameToBlocks after all blocks are deleted
	delete(s.FileNameToBlocks, req.Msg.ObjectKey)

	log.Println("Exiting Delete")
	return connect.NewResponse(&v1.DeleteResponse{Message: "ok"}), nil
}

func selectRandomNumbers(availableItems []uint64, count uint64) (randomNumberSet []uint64) {
	log.Println("Entering selectRandomNumbers")
	defer log.Println("Exiting selectRandomNumbers")
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
