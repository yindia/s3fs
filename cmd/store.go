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

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Manage the store",
}

// getCmd retrieves an item from the store
var getCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get an item from the store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cloudv1connect.NewStorageServiceClient(http.DefaultClient, address)

		pingResponse, err := client.Get(context.Background(), connect.NewRequest(&v1.GetRequest{
			ObjectKey: args[0],
		}))
		if err != nil {
			return err
		}
		responseJSON, err := json.MarshalIndent(string(pingResponse.Msg.Data), "", " ") // Convert response to JSON
		if err != nil {
			return err
		}
		fmt.Println(string(responseJSON))
		return nil
	},
}

// uploadCmd uploads an item to the store
var uploadCmd = &cobra.Command{
	Use:   "upload [key] [file]",
	Short: "Upload an item to the store",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cloudv1connect.NewStorageServiceClient(http.DefaultClient, address)
		data, err := ioutil.ReadFile(args[1])
		if err != nil {
			return err
		}
		pingResponse, err := client.Upload(context.Background(), connect.NewRequest(&v1.UploadRequest{
			ObjectKey: args[0],
			FileSize:  uint32(len(data)),
			Data:      data,
		}))
		if err != nil {
			return err
		}
		fmt.Println(pingResponse.Msg.Message)
		return nil
	},
}

// listCmd lists all items in the store
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all items in the store",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cloudv1connect.NewStorageServiceClient(http.DefaultClient, address)

		pingResponse, err := client.ListObjects(context.Background(), connect.NewRequest(&v1.ListObjectsRequest{}))
		if err != nil {
			return err
		}
		responseJSON, err := json.Marshal(pingResponse.Msg.ObjectKeys) // Convert response to JSON
		if err != nil {
			return err
		}
		fmt.Println(string(responseJSON))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(storeCmd)
	storeCmd.PersistentFlags().StringVar(&address, "address", "http://127.0.0.1:8084", "Set the logging level")

	storeCmd.AddCommand(getCmd)
	storeCmd.AddCommand(uploadCmd)
	storeCmd.AddCommand(listCmd)

}
