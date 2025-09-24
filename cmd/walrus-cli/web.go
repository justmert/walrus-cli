package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newWebCommand() *cobra.Command {
	var background bool
	var port string

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the Walrus web UI",
		Long:  "Launch the Walrus web UI and API server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Start API server in background first
			fmt.Println(color.CyanString("🚀 Starting Walrus services..."))

			// Always start API server on port 3002
			apiPort := "3002"
			if !isPortInUse(apiPort) {
				fmt.Printf("Starting API server on port %s...\n", apiPort)

				// Start API server as a goroutine
				go func() {
					mux := http.NewServeMux()
					setupS3ProxyRoutes(mux)
					setupBlobIndexerRoutes(mux)

					mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte(`{"status":"ok"}`))
					})

					if err := http.ListenAndServe(":"+apiPort, corsMiddleware(mux)); err != nil {
						fmt.Printf("API server error: %v\n", err)
					}
				}()

				// Wait for API server to start
				time.Sleep(500 * time.Millisecond)
				fmt.Println(color.GreenString("✓ API server running"))
			} else {
				fmt.Println(color.YellowString("API server already running on port " + apiPort))
			}

			// Check if we have embedded web UI
			if IsEmbedded() {
				// Use embedded web UI
				webFS, err := GetWebUIFS()
				if err != nil {
					return fmt.Errorf("failed to get embedded web UI: %w", err)
				}

				// Start web server for embedded UI
				fmt.Printf("Starting web UI on port %s...\n", port)

				mux := http.NewServeMux()

				// Serve static files from embedded FS
				fileServer := http.FileServer(webFS)
				mux.Handle("/", fileServer)

				webAddr := ":" + port

				if background {
					// Start in background
					go func() {
						if err := http.ListenAndServe(webAddr, mux); err != nil {
							fmt.Printf("Web UI server error: %v\n", err)
						}
					}()

					fmt.Println(color.GreenString("✓ Web UI started in background"))
					fmt.Printf("\n📋 Web UI: http://localhost:%s\n", port)
					fmt.Printf("📋 API Server: http://localhost:%s\n", apiPort)
					return nil
				} else {
					// Open browser
					url := fmt.Sprintf("http://localhost:%s", port)
					time.Sleep(1 * time.Second)
					openBrowser(url)

					fmt.Println(color.GreenString("✓ Web UI running"))
					fmt.Printf("\n📋 Web UI: %s\n", url)
					fmt.Printf("📋 API Server: http://localhost:%s\n", apiPort)
					fmt.Println(color.CyanString("\nPress Ctrl+C to stop"))

					// Start server (blocking)
					return http.ListenAndServe(webAddr, mux)
				}
			} else {
				// Development mode - try to run npm dev
				return fmt.Errorf("web UI not embedded in this binary. Run from repository with source code or use a release build")
			}
		},
	}

	cmd.Flags().BoolVarP(&background, "background", "b", false, "Run in background")
	cmd.Flags().StringVarP(&port, "port", "p", "5173", "Port for the web UI")

	return cmd
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Could not open browser: %v\n", err)
		fmt.Printf("Please open %s manually\n", url)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

