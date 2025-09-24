package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/justmert/walrus-cli/backend"
)

var (
	// Color definitions
	red      = color.New(color.FgRed).SprintFunc()
	green    = color.New(color.FgGreen).SprintFunc()
	yellow   = color.New(color.FgYellow).SprintFunc()
	blue     = color.New(color.FgHiBlue).SprintFunc()
	magenta  = color.New(color.FgMagenta).SprintFunc()
	cyan     = color.New(color.FgHiCyan).SprintFunc()
	cyanBold = color.New(color.FgHiCyan, color.Bold).SprintFunc()
	blueBold = color.New(color.FgHiBlue, color.Bold).SprintFunc()
)

var (
	epochsFlag int
	dryRunFlag bool
	outputFlag string
	sizeFlag   int64
)

func createRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "walrus-cli",
		Short: "Walrus Storage CLI - Decentralized file storage",
		Long: color.CyanString(`
██╗    ██╗ █████╗ ██╗     ██████╗ ██╗   ██╗███████╗
██║    ██║██╔══██╗██║     ██╔══██╗██║   ██║██╔════╝
██║ █╗ ██║███████║██║     ██████╔╝██║   ██║███████╗
██║███╗██║██╔══██║██║     ██╔══██╗██║   ██║╚════██║
╚███╔███╔╝██║  ██║███████╗██║  ██║╚██████╔╝███████║
 ╚══╝╚══╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝
`) + color.HiBlueString(`            Decentralized Storage CLI`),
		SilenceUsage: true,
	}

	// Setup command
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard",
		Long:  "Launch the interactive setup wizard to configure network, wallet, and storage preferences",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ModernInteractiveSetup()
		},
	}

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show configuration status",
		Long:  "Display current configuration, wallet status, and storage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := backend.LoadConfig("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ModernStatusDisplay(config)
			return nil
		},
	}

	// Upload command
	uploadCmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload a file to Walrus",
		Long:  "Upload a file to Walrus decentralized storage with cost estimation and progress tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := backend.LoadConfig("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client := backend.NewWalrusClient(
				config.Walrus.AggregatorURL,
				config.Walrus.PublisherURL,
			)

			index := loadIndex()
			epochs := epochsFlag
			if epochs == 0 {
				epochs = config.Walrus.Epochs
			}

			handleUpload(client, index, args[0], epochs, dryRunFlag)
			return nil
		},
	}
	uploadCmd.Flags().IntVarP(&epochsFlag, "epochs", "e", 0, "Number of epochs to store (default from config)")
	uploadCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Estimate cost without uploading")

	// Download command
	downloadCmd := &cobra.Command{
		Use:   "download <filename>",
		Short: "Download a file from Walrus",
		Long:  "Download a previously uploaded file from Walrus storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := backend.LoadConfig("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client := backend.NewWalrusClient(
				config.Walrus.AggregatorURL,
				config.Walrus.PublisherURL,
			)

			index := loadIndex()
			handleDownload(client, index, args[0], outputFlag)
			return nil
		},
	}
	downloadCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path")

	// List command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List stored files",
		Long:  "Show all files stored in Walrus with metadata and Walruscan links",
		RunE: func(cmd *cobra.Command, args []string) error {
			index := loadIndex()
			handleListModern(index)
			return nil
		},
	}

	// Info command
	infoCmd := &cobra.Command{
		Use:   "info <filename|blob-id>",
		Short: "Show detailed file information",
		Long:  "Display detailed information about a stored file including Walruscan link",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			index := loadIndex()
			handleInfoModern(index, args[0])
			return nil
		},
	}

	// Cost command
	costCmd := &cobra.Command{
		Use:   "cost",
		Short: "Estimate storage cost",
		Long:  "Calculate the estimated cost in WAL tokens for storing data",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sizeFlag == 0 {
				return fmt.Errorf("please provide file size with --size flag")
			}

			config, err := backend.LoadConfig("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client := backend.NewWalrusClient(
				config.Walrus.AggregatorURL,
				config.Walrus.PublisherURL,
			)

			epochs := epochsFlag
			if epochs == 0 {
				epochs = config.Walrus.Epochs
			}

			return handleCostModern(client, sizeFlag, epochs)
		},
	}
	costCmd.Flags().Int64VarP(&sizeFlag, "size", "s", 0, "File size in bytes (required)")
	costCmd.Flags().IntVarP(&epochsFlag, "epochs", "e", 0, "Number of epochs (default from config)")
	costCmd.MarkFlagRequired("size")

	// Web command
	webCmd := newWebCommand()

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Walrus CLI %s\n", cyanBold(version))
			fmt.Printf("Commit: %s\n", blue(commit))
			fmt.Printf("Built: %s\n", blue(date))
			fmt.Printf("Built by: %s\n", blue(builtBy))
		},
	}

	// Stop command to kill background processes
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop background Walrus services",
		Long:  "Stop all background Walrus services (web UI and API server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			stopped := false

			// Kill processes on port 5173 (Vite dev server)
			if isPortInUse("5173") {
				fmt.Print("Stopping web UI on port 5173...")
				killCmd := exec.Command("sh", "-c", "lsof -ti:5173 | xargs kill -9 2>/dev/null")
				killCmd.Run()
				time.Sleep(1 * time.Second)
				if !isPortInUse("5173") {
					fmt.Println(green(" ✓"))
					stopped = true
				} else {
					fmt.Println(red(" ✗ (failed)"))
				}
			}

			// Kill processes on port 3002 (API server)
			if isPortInUse("3002") {
				fmt.Print("Stopping API server on port 3002...")
				killCmd := exec.Command("sh", "-c", "lsof -ti:3002 | xargs kill -9 2>/dev/null")
				killCmd.Run()
				time.Sleep(1 * time.Second)
				if !isPortInUse("3002") {
					fmt.Println(green(" ✓"))
					stopped = true
				} else {
					fmt.Println(red(" ✗ (failed)"))
				}
			}

			if !stopped {
				fmt.Println(yellow("No background services found running"))
			} else {
				fmt.Println(green("\n✓ Background services stopped"))
			}

			return nil
		},
	}

	// Hidden internal API server command for background mode
	apiServerInternalCmd := &cobra.Command{
		Use:    "api-server-internal",
		Hidden: true, // Hide from help
		RunE: func(cmd *cobra.Command, args []string) error {
			mux := http.NewServeMux()
			setupS3ProxyRoutes(mux)

			// Health check endpoint
			mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Write([]byte(`{"status":"ok"}`))
			})

			return http.ListenAndServe(":3002", mux)
		},
	}

	// Add all commands
	rootCmd.AddCommand(setupCmd, statusCmd, uploadCmd, downloadCmd, listCmd, infoCmd, costCmd, webCmd, stopCmd, versionCmd, s3Cmd, indexerCmd, apiServerInternalCmd)

	return rootCmd
}

// isPortInUse checks if a port is already in use
func isPortInUse(port string) bool {
	conn, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

// Modern colored versions of handlers
func handleListModern(index *FileIndex) {
	if len(index.Files) == 0 {
		fmt.Println("No files stored in Walrus")
		fmt.Println(blue("\nTip: Upload your first file with:"))
		fmt.Println("  walrus-cli upload myfile.pdf")
		return
	}

	fmt.Println()
	fmt.Println(cyanBold("Your Walrus Files"))
	fmt.Println(strings.Repeat("=", 20))
	fmt.Println()

	// Sort files by upload time (most recent first)
	type fileWithName struct {
		name  string
		entry *FileEntry
	}
	var sortedFiles []fileWithName
	for name, entry := range index.Files {
		sortedFiles = append(sortedFiles, fileWithName{name, entry})
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].entry.ModTime.After(sortedFiles[j].entry.ModTime)
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, color.BlueString("NAME\tSIZE\tBLOB ID\tEXPIRY\tUPLOADED\tWALRUSCAN"))

	for _, file := range sortedFiles {
		name := file.name
		entry := file.entry
		blobIDDisplay := entry.BlobID
		walruscanLink := "—"

		if len(entry.BlobID) > 12 {
			blobIDDisplay = cyan(entry.BlobID[:12] + "...")
			walruscanLink = green("Available")
		} else if entry.BlobID != "" {
			blobIDDisplay = cyan(entry.BlobID)
			walruscanLink = green("Available")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			name,
			formatBytes(entry.Size),
			blobIDDisplay,
			fmt.Sprintf("Epoch %d", entry.ExpiryEpoch),
			entry.ModTime.Format("2006-01-02 15:04"),
			walruscanLink,
		)
	}

	w.Flush()
	fmt.Println()
	fmt.Println(blue("Tip: Use 'walrus-cli info <filename>' for detailed information"))
}

func handleInfoModern(index *FileIndex, nameOrID string) {
	// Check if it's a filename in our index
	if entry, exists := index.Files[nameOrID]; exists {
		fmt.Println()
		fmt.Println(cyanBold("File Information"))
		fmt.Println(strings.Repeat("=", 20))
		fmt.Printf("Name:       %s\n", magenta(nameOrID))
		fmt.Printf("Size:       %s\n", blue(formatBytes(entry.Size)))
		fmt.Printf("Blob ID:    %s\n", cyan(entry.BlobID))
		fmt.Printf("Uploaded:   %s\n", green(entry.ModTime.Format("2006-01-02 15:04:05")))
		fmt.Printf("Expires:    %s\n", yellow(fmt.Sprintf("Epoch %d", entry.ExpiryEpoch)))

		if entry.BlobID != "" {
			fmt.Println()
			fmt.Println(blueBold("Walruscan Explorer"))
			fmt.Printf("URL: %s\n", blue(fmt.Sprintf("https://walruscan.com/testnet/blob/%s", entry.BlobID)))
		}
		fmt.Println()
		return
	}

	// Check if it might be a blob ID
	for name, entry := range index.Files {
		if entry.BlobID == nameOrID {
			fmt.Println()
			fmt.Println(cyanBold("Blob Information"))
			fmt.Println(strings.Repeat("=", 20))
			fmt.Printf("Blob ID:    %s\n", cyan(entry.BlobID))
			fmt.Printf("File Name:  %s\n", magenta(name))
			fmt.Printf("Size:       %s\n", blue(formatBytes(entry.Size)))
			fmt.Printf("Uploaded:   %s\n", green(entry.ModTime.Format("2006-01-02 15:04:05")))
			fmt.Printf("Expires:    %s\n", yellow(fmt.Sprintf("Epoch %d", entry.ExpiryEpoch)))
			fmt.Println()
			fmt.Println(blueBold("Walruscan Explorer"))
			fmt.Printf("URL: %s\n", blue(fmt.Sprintf("https://walruscan.com/testnet/blob/%s", entry.BlobID)))
			fmt.Println()
			return
		}
	}

	fmt.Printf(red("❌ File or blob ID '%s' not found in index\n"), nameOrID)
	fmt.Println(blue("💡 Use 'walrus-cli list' to see available files"))
}

func handleCostModern(client *backend.WalrusClient, size int64, epochs int) error {
	cost, err := client.EstimateStorageCost(size, epochs)
	if err != nil {
		return fmt.Errorf("estimating cost: %w", err)
	}

	fmt.Println()
	fmt.Println(cyanBold("Storage Cost Estimation"))
	fmt.Println(strings.Repeat("=", 30))
	fmt.Printf("File Size:  %s\n", formatBytes(size))
	fmt.Printf("Duration:   %d epochs\n", epochs)
	fmt.Printf("Cost:       %s\n", green(formatWAL(cost)+" WAL"))
	fmt.Printf("USD Value:  %s\n", green(fmt.Sprintf("~$%.4f", float64(cost)/1_000_000_000*0.425)))
	fmt.Println()

	return nil
}
