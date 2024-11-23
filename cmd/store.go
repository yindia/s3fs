package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/schollz/progressbar/v3"
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
	Use:   "get [key] [output_file]",
	Short: "Get an item from the store",
	Long:  "Retrieves an item from the store using the specified key and writes the result to the specified output file.",
	Args:  cobra.ExactArgs(2), // Changed to expect 2 arguments
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createStorageClient()
		log.Printf("Attempting to retrieve item with key: '%s'", args[0])
		response, err := client.Get(context.Background(), connect.NewRequest(&v1.GetObjectRequest{
			ObjectKey: args[0],
		}))
		if err != nil {
			log.Printf("Error retrieving item: %s", err)
			return fmt.Errorf("failed to get item with key '%s': %w", args[0], err)
		}

		// Create a progress bar
		bar := progressbar.NewOptions64(-1,
			progressbar.OptionSetDescription("Downloading..."), // Initial description
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(15),
		)
		file, err := os.OpenFile(args[1], os.O_CREATE|os.O_WRONLY, 0644) // Use args[1] for output file
		if err != nil {
			return fmt.Errorf("failed to open file '%s': %w", args[1], err)
		}
		defer file.Close()

		for response.Receive() {
			message := response.Msg()
			if _, err := file.Write(message.Data); err != nil {
				return fmt.Errorf("failed to write to file '%s': %w", args[1], err)
			}
			bar.Add(len(message.Data))
		}

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
		log.Printf("Attempting to delete item with key: '%s'", args[0])
		deleteResponse, err := client.Delete(context.Background(), connect.NewRequest(&v1.DeleteRequestMsg{
			ObjectKey: args[0],
		}))
		if err != nil {
			log.Printf("Error deleting item: %s", err)
			return fmt.Errorf("failed to delete item with key '%s': %w", args[0], err)
		}
		fmt.Println(deleteResponse.Msg.Status)
		return nil
	},
}

// uploadCmd uploads an item to the store
var uploadCmd = &cobra.Command{
	Use:   "upload [key] [file]",
	Short: "Upload an item to the store",
	Long:  "Uploads an item to the store with the specified key and file. The file's content is sent to the storage service in smaller chunks.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createStorageClient()
		stream := client.Upload(context.Background())
		file, err := os.Open(args[1])
		if err != nil {
			return fmt.Errorf("failed to open file '%s': %w", args[1], err)
		}
		defer file.Close()

		// Check if the file is empty
		fileInfo, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file info for '%s': %w", args[1], err)
		}
		if fileInfo.Size() == 0 {
			return fmt.Errorf("file '%s' is empty", args[1])
		}

		const chunkSize = 1024 * 1024
		buffer := make([]byte, chunkSize)
		totalChunks := 0

		bar := progressbar.NewOptions64(fileInfo.Size()/int64(chunkSize),
			progressbar.OptionSetDescription("Uploading..."),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(15),
		)
		for {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to read file '%s': %w", args[1], err)
			}
			if n == 0 {
				break
			}

			totalChunks++
			bar.Add(1)
			if err := stream.Send(&v1.UploadRequestMsg{
				ObjectKey: args[0],
				Data:      buffer[:n],
			}); err != nil {
				return fmt.Errorf("failed to send upload request: %w", err)
			}
		}

		fmt.Printf("Uploaded %d chunks successfully for key '%s'\n", totalChunks, args[0])
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
		response, err := client.List(context.Background(), connect.NewRequest(&v1.ListObjectsRequest{}))
		if err != nil {
			return fmt.Errorf("failed to list items: %w", err)
		}
		// Create a slice to hold the item metadata
		var metadataList []string
		for _, data := range response.Msg.Metadata {
			metadataList = append(metadataList, fmt.Sprintf("%s\t%s\t%d", data.ObjectKey, data.Extension, data.FileSize)) // Collect metadata
		}

		// Print the metadata in a table format
		w := new(tabwriter.Writer)
		w.Init(os.Stdout, 0, 8, 0, '\t', 0)      // Initialize tab writer
		fmt.Fprintln(w, "Name\tExtension\tSize") // Print header
		for _, entry := range metadataList {
			fmt.Fprintln(w, entry) // Print each metadata entry
		}
		w.Flush() // Flush the writer to output

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
