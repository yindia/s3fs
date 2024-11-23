package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
)

// storeCmd represents the store command
var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Manage the store",
	Long:  "The store command allows you to manage items in the storage system, including retrieving, uploading, and listing items.",
}

// createStorageClient initializes a new storage service client
func createStorageClient() cloudv1connect.StorageServiceClient {
	return cloudv1connect.NewStorageServiceClient(http.DefaultClient, address)
}

// getCmd retrieves an item from the store
var getCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get an item from the store",
	Long:  "Retrieves an item from the store using the specified key. The result is returned in JSON format.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createStorageClient()
		pingResponse, err := client.Get(context.Background(), connect.NewRequest(&v1.GetObjectRequest{
			ObjectKey: args[0],
		}))
		if err != nil {
			return fmt.Errorf("failed to get item with key '%s': %w", args[0], err)
		}
		responseJSON, err := json.MarshalIndent(pingResponse.Msg.Data, "", " ")
		if err != nil {
			return fmt.Errorf("failed to marshal response to JSON: %w", err)
		}
		fmt.Println(string(responseJSON))
		return nil
	},
}

// deleteCmd removes an item from the store
var deleteCmd = &cobra.Command{
	Use:   "delete [key]",
	Short: "Delete an item from the store",
	Long:  "Removes an item from the store using the specified key.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createStorageClient()
		pingResponse, err := client.Delete(context.Background(), connect.NewRequest(&v1.DeleteRequestMsg{
			ObjectKey: args[0],
		}))
		if err != nil {
			return fmt.Errorf("failed to delete item with key '%s': %w", args[0], err)
		}
		fmt.Println(pingResponse.Msg.Status)
		return nil
	},
}

// uploadCmd uploads an item to the store
var uploadCmd = &cobra.Command{
	Use:   "upload [key] [file]",
	Short: "Upload an item to the store",
	Long:  "Uploads an item to the store with the specified key and file. The file's content is sent to the storage service.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createStorageClient()
		data, err := ioutil.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("failed to read file '%s': %w", args[1], err)
		}
		pingResponse, err := client.Upload(context.Background(), connect.NewRequest(&v1.UploadRequestMsg{
			ObjectKey: args[0],
			Data:      data,
		}))
		if err != nil {
			return fmt.Errorf("failed to upload item with key '%s': %w", args[0], err)
		}
		fmt.Println(pingResponse.Msg.Status)
		return nil
	},
}

// listCmd lists all items in the store
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all items in the store",
	Long:  "Lists all items currently stored in the storage system. The result is returned in JSON format.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createStorageClient()
		pingResponse, err := client.List(context.Background(), connect.NewRequest(&v1.ListObjectsRequest{}))
		if err != nil {
			return fmt.Errorf("failed to list items: %w", err)
		}
		responseJSON, err := json.Marshal(pingResponse.Msg.ObjectKeys)
		if err != nil {
			return fmt.Errorf("failed to marshal object keys to JSON: %w", err)
		}
		fmt.Println(string(responseJSON))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(storeCmd)
	storeCmd.PersistentFlags().StringVar(&address, "address", "http://127.0.0.1:8080", "Set the address of the storage service")

	storeCmd.AddCommand(getCmd)
	storeCmd.AddCommand(uploadCmd)
	storeCmd.AddCommand(listCmd)
	storeCmd.AddCommand(deleteCmd)
}
