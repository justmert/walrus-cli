package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newWebCommand() *cobra.Command {
	var background bool

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the Walrus web UI",
		Long:  "Launch the Walrus web UI development server from this repository.",
		RunE: func(cmd *cobra.Command, args []string) error {
			uiPath, err := resolveWebUIPath()
			if err != nil {
				return err
			}

			if _, err := exec.LookPath("npm"); err != nil {
				return errors.New("npm not found in PATH. Install Node.js to start the web UI")
			}

			pkgPath := filepath.Join(uiPath, "package.json")
			if _, err := os.Stat(pkgPath); err != nil {
				return fmt.Errorf("missing package.json at %s (expected Walrus web UI project)", pkgPath)
			}

			// Start API server in background first
			fmt.Println(color.CyanString("🚀 Starting Walrus services..."))

			if !isPortInUse("3002") {
				fmt.Printf("Starting API server on port 3002...")

				// Always start API server as a goroutine
				go func() {
					mux := http.NewServeMux()
					setupS3ProxyRoutes(mux)
					mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Header().Set("Access-Control-Allow-Origin", "*")
						w.Write([]byte(`{"status":"ok"}`))
					})
					http.ListenAndServe(":3002", mux)
				}()

				// Wait for server to start
				time.Sleep(2 * time.Second)

				// Verify it started
				if isPortInUse("3002") {
					fmt.Println(color.GreenString(" ✓"))
				} else {
					fmt.Println(color.RedString(" ✗"))
					fmt.Println(color.YellowString("Warning: API server may not have started properly"))
				}
			} else {
				fmt.Println(color.YellowString("API server already running on port 3002"))
			}

			npmArgs := []string{"run", "dev", "--", "--host"}
			npmCmd := exec.CommandContext(cmd.Context(), "npm", npmArgs...)
			npmCmd.Dir = uiPath
			npmCmd.Env = os.Environ()

			if background {
				// Start npm in background
				npmCmd.Stdout = io.Discard
				npmCmd.Stderr = io.Discard

				if err := npmCmd.Start(); err != nil {
					return fmt.Errorf("starting web UI in background: %w", err)
				}

				npmPID := npmCmd.Process.Pid

				fmt.Fprintf(cmd.OutOrStdout(), "✓ Web UI running in background (PID %d). Open http://localhost:5173\n", npmPID)
				fmt.Fprintf(cmd.OutOrStdout(), "✓ API server running on http://localhost:3002\n")
				fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'walrus-cli stop' to stop all background services\n")

				// Important: Wait for npm process to keep the API server goroutine alive
				// This blocks but since we're in background mode, the shell returns control
				npmCmd.Wait()
				return nil
			}

			npmCmd.Stdout = cmd.OutOrStdout()
			npmCmd.Stderr = cmd.ErrOrStderr()
			npmCmd.Stdin = os.Stdin

			fmt.Printf("Starting web UI from %s...\n", uiPath)
			fmt.Println(color.GreenString("✓ API server running on http://localhost:3002"))
			fmt.Println(color.GreenString("✓ Web UI will be available at http://localhost:5173"))
			fmt.Println(color.YellowString("\nBoth services are now ready for S3 to Walrus transfers!"))
			if err := npmCmd.Run(); err != nil {
				return fmt.Errorf("web UI exited with error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&background, "background", "b", false, "run the web UI in the background")
	return cmd
}

func resolveWebUIPath() (string, error) {
	if env := os.Getenv("WALRUS_WEB_UI_PATH"); env != "" {
		path := filepath.Clean(env)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
		return "", fmt.Errorf("WALRUS_WEB_UI_PATH=%s does not point to a valid directory", env)
	}

	candidates := []string{
		filepath.Join(".", "web", "walrus-ui"),
		filepath.Join("web", "walrus-ui"),
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "web", "walrus-ui"))
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "web", "walrus-ui"),
			filepath.Join(exeDir, "..", "web", "walrus-ui"),
		)
	}

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}

		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs, nil
		}
	}

	return "", errors.New("could not locate the web UI directory. Set WALRUS_WEB_UI_PATH or run the CLI from the repository root")
}

