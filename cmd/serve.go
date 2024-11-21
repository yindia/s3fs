package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	dsroute "s3fs/pkg/datanode"
	v1 "s3fs/pkg/gen/cloud/v1"
	"s3fs/pkg/gen/cloud/v1/cloudv1connect"
	s3fs "s3fs/pkg/s3fs"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	CompressMinByte = 1024 // Minimum byte size for compression

)

var (
	port              uint16
	server            string
	dir               string
	blockSize         uint64
	replicationFactor uint64
	nodes             []string
)

// newCORS initializes CORS settings for the server
// It allows all origins and methods, and exposes necessary headers for gRPC-Web
func newCORS() *cors.Cors {
	return cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(origin string) bool {
			return true // Allow all origins
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
		},
	})
}

var serveCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"s", "server"},
	Short:   "Server the s3 server",
	Long:    ``,
	Example: `  `,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

// initializeServer initializes and returns a new HTTP server
func initializeServer(mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", port),
		Handler: h2c.NewHandler(
			newCORS().Handler(mux),
			&http2.Server{},
		),
		MaxHeaderBytes:    1 << 20, // 1 MB
		ReadHeaderTimeout: 60 * time.Minute,
		ReadTimeout:       60 * time.Minute,
		WriteTimeout:      60 * time.Minute,
	}
}

// createMux initializes a new ServeMux and sets up the handlers
func createMux(serviceHandler func() (string, http.Handler)) *http.ServeMux {
	mux := http.NewServeMux()
	pattern, handler := serviceHandler()
	mux.Handle(pattern, handler)

	// Health check and reflection handlers
	mux.Handle(grpchealth.NewHandler(
		grpchealth.NewStaticChecker(cloudv1connect.DataNodeServiceName), // Adjust this for s3fs
	))
	mux.Handle(grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(cloudv1connect.DataNodeServiceName), // Adjust this for s3fs
	))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(cloudv1connect.DataNodeServiceName), // Adjust this for s3fs
	))

	return mux
}

// runServer starts the HTTP server and handles shutdown
func runServer(mux *http.ServeMux) error {
	srv := initializeServer(mux)

	// Start the server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		slog.Info("HTTP server starting", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- fmt.Errorf("HTTP server failed: %w", err)
		}
	}()

	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for exit signal or server error
	select {
	case <-exitChan:
		slog.Info("Shutdown signal received, shutting down server...")
	case err := <-serverErrChan:
		return err
	}

	// Graceful shutdown
	if err := shutdownServer(srv); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}
	return nil
}

var datanodeCmd = &cobra.Command{
	Use:   "datanode",
	Short: "Start the datanode server",
	RunE: func(cmd *cobra.Command, args []string) error {
		mux := createMux(func() (string, http.Handler) {
			otelInterceptor, err := otelconnect.NewInterceptor()
			if err != nil {
				return "/", http.NotFoundHandler()
			}

			return cloudv1connect.NewDataNodeServiceHandler(
				dsroute.NewDataNodeServer(dir, port, server),
				connect.WithInterceptors(otelInterceptor),
				connect.WithCompressMinBytes(CompressMinByte),
				connect.WithSendMaxBytes(math.MaxInt32),
				connect.WithReadMaxBytes(math.MaxInt32),
			)
		})

		return runServer(mux)
	},
}

var s3fsCmd = &cobra.Command{
	Use:   "s3fs",
	Short: "Start the s3fs server",
	RunE: func(cmd *cobra.Command, args []string) error {
		mux := createMux(func() (string, http.Handler) {
			otelInterceptor, err := otelconnect.NewInterceptor()
			if err != nil {
				return "/", http.NotFoundHandler()
			}

			s3 := s3fs.NewS3FSServer(blockSize, replicationFactor, port)
			if err := discoverDataNodes(nodes, s3); err != nil {
				return "/", http.NotFoundHandler()
			}

			go heartbeatToDataNodes(nodes, s3)

			return cloudv1connect.NewStorageServiceHandler(
				s3,
				connect.WithInterceptors(otelInterceptor),
				connect.WithCompressMinBytes(CompressMinByte),
				connect.WithSendMaxBytes(math.MaxInt32),
				connect.WithReadMaxBytes(math.MaxInt32),
			)
		})

		return runServer(mux)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.AddCommand(datanodeCmd) // Add datanode command
	serveCmd.AddCommand(s3fsCmd)     // Add s3fs command

	datanodeCmd.Flags().Uint16VarP(&port, "port", "p", 8080, "Port to run the datanode server on")
	datanodeCmd.Flags().StringVarP(&server, "server-url", "u", "http://localhost", "Server URL for the datanode server")
	datanodeCmd.Flags().StringVarP(&dir, "dir", "d", "./", "")

	s3fsCmd.Flags().Uint16VarP(&port, "port", "p", 8080, "Port to run the datanode server on")
	s3fsCmd.Flags().Uint64VarP(&blockSize, "block-size", "b", 10, "Server URL for the datanode server")
	s3fsCmd.Flags().Uint64VarP(&replicationFactor, "replication-factor", "r", 2, "Server URL for the datanode server")
	s3fsCmd.Flags().StringArrayVarP(&nodes, "", "n", []string{}, "Server URL for the datanode server")
}

// shutdownServer gracefully shuts down the HTTP server
// It waits for ongoing requests to complete before shutting down
func shutdownServer(srv *http.Server) error {
	slog.Info("Initiating graceful shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}
	slog.Info("Server shutdown completed")
	return nil
}

func discoverDataNodes(listOfDataNodes []string, s3 *s3fs.S3fs) error {
	for i, node := range listOfDataNodes {
		log.Printf("Discovering DataNodes ...\n")
		uri := strings.Split(node, ":")
		if len(uri) != 2 {
			log.Printf("Invalid node format: %s\n", node)
			continue
		}
		port, err := strconv.Atoi(uri[1])
		if err != nil {
			log.Printf("Error converting port: %s\n", err)
			continue
		}
		client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", uri[0], port))

		pingResponse, err := client.Ping(context.Background(), connect.NewRequest(&v1.PingRequest{
			Host: uri[0],
			Port: uint32(port),
		}))

		if err != nil {
			log.Printf("No ack received from %s:%s\n", uri[0], uri[1])
			continue
		}

		if pingResponse.Msg.Ack {
			s3.SetIdToDataNodes(s3fs.DataNodeInstance{
				Host:        uri[0],
				ServicePort: port,
			}, uint64(i))
			log.Printf("Ack received from %s:%s\n", uri[0], uri[1])
		} else {
			log.Printf("No ack received from %s:%s\n", uri[0], uri[1])
		}
	}
	return nil
}

// heartbeatToDataNodes tracks heartbeats from data nodes and handles failures
func heartbeatToDataNodes(listOfDataNodes []string, s3 *s3fs.S3fs) {
	for range time.Tick(time.Second * 5) {
		for i, node := range listOfDataNodes {
			uri := strings.Split(node, ":")
			if len(uri) != 2 {
				log.Printf("Invalid node format: %s\n", node)
				continue
			}
			n, err := strconv.Atoi(uri[1])
			if len(uri) != 2 {
				log.Printf("Invalid node format: %s\n", node)
				continue
			}
			client := cloudv1connect.NewDataNodeServiceClient(http.DefaultClient, fmt.Sprintf("http://%s:%d", uri[0], n))
			resp, err := client.Heartbeat(context.Background(), connect.NewRequest(&v1.HeartbeatRequest{}))
			if err != nil {
				log.Printf("Error converting port: %s\n", err)
				// Redistribute the Data
				result, err := s3.ReDistribute(context.Background(), connect.NewRequest(&v1.ReDistributeRequest{
					DataNodeUri: fmt.Sprintf("http://%s:%d", uri[0], n),
				}))

				if err != nil {
					log.Printf("Invalid node format: %s\n", node)
					continue
				}
				fmt.Println(result.Msg.Message)
				s3.DeleteIdToDataNodes(uint64(i))
				continue
			}

			if resp.Msg.Status == "ok" {
				s3.SetIdToDataNodes(s3fs.DataNodeInstance{
					Host:        uri[0],
					ServicePort: n,
				}, uint64(i))
			} else {
				s3.DeleteIdToDataNodes(uint64(i))
			}

		}
	}
}
