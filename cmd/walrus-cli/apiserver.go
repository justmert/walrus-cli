package main

import (
	"fmt"
	"net/http"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newAPIServerCommand() *cobra.Command {
	var port string

	cmd := &cobra.Command{
		Use:   "api-server",
		Short: "Start the Walrus API server",
		Long:  "Launch the API server that provides S3 proxy and other endpoints for the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			mux := http.NewServeMux()

			// Setup S3 proxy routes
			setupS3ProxyRoutes(mux)

			// Setup blob indexer routes
			setupBlobIndexerRoutes(mux)

			// Health check endpoint
			mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Write([]byte(`{"status":"ok"}`))
			})

			addr := ":" + port
			fmt.Println(color.CyanString("🚀 Walrus API Server"))
			fmt.Printf("Starting API server on http://localhost%s\n", addr)
			fmt.Println(color.YellowString("\nAvailable endpoints:"))
			fmt.Println("  • POST /api/s3/proxy      - S3 operations proxy")
			fmt.Println("  • POST /api/s3/transfer   - S3 to Walrus transfer")
			fmt.Println("  • POST /api/blobs/list    - List user's Walrus blobs")
			fmt.Println("  • GET  /api/blobs/search  - Search user's blobs")
			fmt.Println("  • GET  /api/health        - Health check")
			fmt.Println("\nPress Ctrl+C to stop the server")

			return http.ListenAndServe(addr, mux)
		},
	}

	cmd.Flags().StringVarP(&port, "port", "p", "3002", "Port to run the API server on")

	return cmd
}