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

	dsroute "s3fs/pkg/datanode"
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

var datanodeCmd = &cobra.Command{
	Use:   "datanode",
	Short: "Start the datanode server",
	RunE: func(cmd *cobra.Command, args []string) error {
		mux := http.NewServeMux()
		otelInterceptor, err := otelconnect.NewInterceptor()
		if err != nil {
			return fmt.Errorf("failed to create interceptor: %w", err)
		}

		pattern, handler := cloudv1connect.NewDataNodeServiceHandler(
			dsroute.NewDataNodeServer(),
			connect.WithInterceptors(otelInterceptor),
			connect.WithCompressMinBytes(CompressMinByte),
			connect.WithSendMaxBytes(math.MaxInt32),
			connect.WithReadMaxBytes(math.MaxInt32),
		)
		mux.Handle(pattern, handler)

		// Health check and reflection handlers
		mux.Handle(grpchealth.NewHandler(
			grpchealth.NewStaticChecker(cloudv1connect.DataNodeServiceName),
		))
		mux.Handle(grpcreflect.NewHandlerV1(
			grpcreflect.NewStaticReflector(cloudv1connect.DataNodeServiceName),
		))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(
			grpcreflect.NewStaticReflector(cloudv1connect.DataNodeServiceName),
		))

		exitChan := make(chan os.Signal, 1)
		signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

		// Initialize HTTP server
		srv := &http.Server{
			Addr: fmt.Sprintf("0.0.0.0:%v", ""),
			Handler: h2c.NewHandler(
				newCORS().Handler(mux),
				&http2.Server{},
			),
			MaxHeaderBytes:    1 << 20, // 1 MB
			ReadHeaderTimeout: 60 * time.Minute,
			ReadTimeout:       60 * time.Minute,
			WriteTimeout:      60 * time.Minute,
		}

		// Start the server in a goroutine
		serverErrChan := make(chan error, 1)
		go func() {
			slog.Info("HTTP server starting", "address", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrChan <- fmt.Errorf("HTTP server failed: %w", err)
			}
		}()

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
	},
}

var s3fsCmd = &cobra.Command{
	Use:   "s3fs",
	Short: "Start the s3fs server",
	RunE: func(cmd *cobra.Command, args []string) error {
		mux := http.NewServeMux()

		otelInterceptor, err := otelconnect.NewInterceptor()
		if err != nil {
			return fmt.Errorf("failed to create interceptor: %w", err)
		}

		pattern, handler := cloudv1connect.NewStorageServiceHandler(
			s3fs.NewS3FSServer(),
			connect.WithInterceptors(otelInterceptor),
			connect.WithCompressMinBytes(CompressMinByte),
			connect.WithSendMaxBytes(math.MaxInt32),
			connect.WithReadMaxBytes(math.MaxInt32),
		)
		mux.Handle(pattern, handler)

		// Health check and reflection handlers
		mux.Handle(grpchealth.NewHandler(
			grpchealth.NewStaticChecker(cloudv1connect.StorageServiceName),
		))
		mux.Handle(grpcreflect.NewHandlerV1(
			grpcreflect.NewStaticReflector(cloudv1connect.StorageServiceName),
		))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(
			grpcreflect.NewStaticReflector(cloudv1connect.StorageServiceName),
		))

		exitChan := make(chan os.Signal, 1)
		signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

		// Initialize HTTP server
		srv := &http.Server{
			Addr: fmt.Sprintf("0.0.0.0:%v", ""),
			Handler: h2c.NewHandler(
				newCORS().Handler(mux),
				&http2.Server{},
			),
			MaxHeaderBytes:    1 << 20, // 1 MB
			ReadHeaderTimeout: 60 * time.Minute,
			ReadTimeout:       60 * time.Minute,
			WriteTimeout:      60 * time.Minute,
		}

		// Start the server in a goroutine
		serverErrChan := make(chan error, 1)
		go func() {
			slog.Info("HTTP server starting", "address", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrChan <- fmt.Errorf("HTTP server failed: %w", err)
			}
		}()

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
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.AddCommand(datanodeCmd) // Add datanode command
	serveCmd.AddCommand(s3fsCmd)     // Add s3fs command
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
