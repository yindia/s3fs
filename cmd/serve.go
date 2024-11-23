package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"s3fs/pkg/gen/cloud/v1/cloudv1connect"
	dsroute "s3fs/server/route"

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
	port uint16
	dir  string
)

// newCORS initializes CORS settings for the server
// It allows all origins and methods, and exposes necessary headers for gRPC-Web
func newCORS() *cors.Cors {
	slog.Debug("Initializing CORS settings")
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

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"s", "server"},
	Short:   "Start the S3 server",
	Long:    "This command starts the S3 server with the specified configurations.",
	Example: `  `,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mux := createMux(createServiceHandler())
		return runServer(mux)
	},
}

// createServiceHandler creates a service handler for the storage service
func createServiceHandler() func() (string, http.Handler) {
	return func() (string, http.Handler) {
		otelInterceptor, err := otelconnect.NewInterceptor()
		if err != nil {
			return "/", http.NotFoundHandler()
		}

		return cloudv1connect.NewStorageServiceHandler(
			dsroute.NewStorage(dir),
			connect.WithInterceptors(otelInterceptor),
			connect.WithCompressMinBytes(CompressMinByte),
			connect.WithSendMaxBytes(math.MaxInt32),
			connect.WithReadMaxBytes(math.MaxInt32),
		)
	}
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
		grpchealth.NewStaticChecker(cloudv1connect.StorageServiceName), // Adjust this for s3fs
	))
	mux.Handle(grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(cloudv1connect.StorageServiceName), // Adjust this for s3fs
	))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(cloudv1connect.StorageServiceName), // Adjust this for s3fs
	))

	return mux
}

// runServer starts the HTTP server and handles shutdown
func runServer(mux *http.ServeMux) error {
	srv := initializeServer(mux)

	slog.Debug("Server initialized", "address", srv.Addr)

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
		slog.Error("Server error occurred", "error", err)
		return err
	}

	// Graceful shutdown
	if err := shutdownServer(srv); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
		return fmt.Errorf("HTTP server shutdown failed: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().Uint16VarP(&port, "port", "p", 8080, "Port to run the datanode server on")
	serveCmd.Flags().StringVarP(&dir, "dir", "d", "./", "")

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
