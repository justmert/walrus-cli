package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/walrus-rclone/mvp/backend"
)

// FileIndex manages local file mappings
type FileIndex struct {
	Files map[string]*FileEntry `json:"files"`
}

type FileEntry struct {
	BlobID       string    `json:"blob_id"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	ExpiryEpoch  int       `json:"expiry_epoch"`
	OriginalPath string    `json:"original_path"`
}

func mainLegacy() {
	// Define commands
	uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
	downloadCmd := flag.NewFlagSet("download", flag.ExitOnError)
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	costCmd := flag.NewFlagSet("cost", flag.ExitOnError)
	infoCmd := flag.NewFlagSet("info", flag.ExitOnError)
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)

	// Upload flags
	uploadEpochs := uploadCmd.Int("epochs", 5, "Number of epochs to store")
	uploadDryRun := uploadCmd.Bool("dry-run", false, "Estimate cost without uploading")

	// Download flags
	downloadOutput := downloadCmd.String("output", "", "Output file path")

	// Cost flags
	costSize := costCmd.Int64("size", 0, "File size in bytes")
	costEpochs := costCmd.Int("epochs", 5, "Number of epochs")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load configuration
	config, err := backend.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create client
	client := backend.NewWalrusClient(
		config.Walrus.AggregatorURL,
		config.Walrus.PublisherURL,
	)

	// Load file index
	index := loadIndex()

	switch os.Args[1] {
	case "upload":
		uploadCmd.Parse(os.Args[2:])
		if uploadCmd.NArg() < 1 {
			fmt.Println("Error: Please provide a file to upload")
			os.Exit(1)
		}
		handleUpload(client, index, uploadCmd.Arg(0), *uploadEpochs, *uploadDryRun)

	case "download":
		downloadCmd.Parse(os.Args[2:])
		if downloadCmd.NArg() < 1 {
			fmt.Println("Error: Please provide a filename to download")
			os.Exit(1)
		}
		handleDownload(client, index, downloadCmd.Arg(0), *downloadOutput)

	case "list", "ls":
		listCmd.Parse(os.Args[2:])
		handleList(index)

	case "cost":
		costCmd.Parse(os.Args[2:])
		handleCost(client, *costSize, *costEpochs)

	case "init":
		handleInit()

	case "setup":
		if err := InteractiveSetup(); err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}

	case "info":
		infoCmd.Parse(os.Args[2:])
		if infoCmd.NArg() < 1 {
			fmt.Println("Error: Please provide a filename or blob ID")
			os.Exit(1)
		}
		handleInfo(index, infoCmd.Arg(0))

	case "status":
		statusCmd.Parse(os.Args[2:])
		handleStatus(config)

	default:
		printUsage()
		os.Exit(1)
	}
}

func handleUpload(client *backend.WalrusClient, index *FileIndex, filePath string, epochs int, dryRun bool) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	fileName := filepath.Base(filePath)
	fileSize := int64(len(data))

	// Estimate cost
	cost, err := client.EstimateStorageCost(fileSize, epochs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error estimating cost: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", fileName)
	fmt.Printf("Size: %s\n", formatBytes(fileSize))
	fmt.Printf("Epochs: %d\n", epochs)
	fmt.Printf("Estimated Cost: %s\n", formatWALWithUSD(cost))

	if dryRun {
		fmt.Println("\n✓ Dry run complete (no data uploaded)")
		return
	}

	fmt.Print("\nUploading... ")

	// Upload to Walrus
	resp, err := client.StoreBlob(data, epochs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError uploading: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓")

	// Update index
	expiryEpoch := 0
	if resp.EndEpoch != nil {
		expiryEpoch = int(*resp.EndEpoch)
	}
	index.Files[fileName] = &FileEntry{
		BlobID:       resp.BlobID,
		Size:         fileSize,
		ModTime:      time.Now(),
		ExpiryEpoch:  expiryEpoch,
		OriginalPath: filePath,
	}

	// Save index
	if err := saveIndex(index); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save index: %v\n", err)
	}

	fmt.Printf("\n%s\n", color.GreenString("🎉 Successfully uploaded to Walrus"))
	fmt.Printf("  %s %s\n", color.CyanString("Blob ID:"), color.BlueString(resp.BlobID))
	fmt.Printf("  %s %s\n", color.YellowString("Expires:"), color.YellowString("Epoch %d", expiryEpoch))
	fmt.Printf("  %s %s\n", color.MagentaString("Walruscan:"), color.BlueString("https://walruscan.com/testnet/blob/%s", resp.BlobID))
}

func handleDownload(client *backend.WalrusClient, index *FileIndex, fileName, outputPath string) {
	// Find file in index
	entry, exists := index.Files[fileName]
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: File '%s' not found in index\n", fileName)
		fmt.Println("Use 'walrus-cli list' to see available files")
		os.Exit(1)
	}

	fmt.Printf("Downloading %s (Blob ID: %s)... ", fileName, entry.BlobID[:12]+"...")

	// Download from Walrus
	data, err := client.RetrieveBlob(entry.BlobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError downloading: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓")

	// Determine output path
	if outputPath == "" {
		outputPath = fileName
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Saved to: %s (%s)\n", outputPath, formatBytes(int64(len(data))))
}

func handleList(index *FileIndex) {
	if len(index.Files) == 0 {
		fmt.Println("No files stored in Walrus")
		return
	}

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
	fmt.Fprintln(w, "NAME\tSIZE\tBLOB ID\tEXPIRY\tUPLOADED\tWALRUSCAN")

	for _, file := range sortedFiles {
		name := file.name
		entry := file.entry
		blobIDDisplay := entry.BlobID
		walruscanLink := "—"
		if len(entry.BlobID) > 12 {
			blobIDDisplay = entry.BlobID[:12] + "..."
			walruscanLink = "✓ View"
		} else if entry.BlobID != "" {
			walruscanLink = "✓ View"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\tEpoch %d\t%s\t%s\n",
			name,
			formatBytes(entry.Size),
			blobIDDisplay,
			entry.ExpiryEpoch,
			entry.ModTime.Format("2006-01-02 15:04"),
			walruscanLink,
		)
	}

	w.Flush()
	fmt.Println("\nTip: Use 'walrus-cli info <filename>' to get the Walruscan URL")
}

func handleInfo(index *FileIndex, nameOrID string) {
	// Check if it's a filename in our index
	if entry, exists := index.Files[nameOrID]; exists {
		fmt.Printf("File Information\n")
		fmt.Printf("================\n")
		fmt.Printf("Name: %s\n", nameOrID)
		fmt.Printf("Size: %s\n", formatBytes(entry.Size))
		fmt.Printf("Blob ID: %s\n", entry.BlobID)
		fmt.Printf("Uploaded: %s\n", entry.ModTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("Expires: Epoch %d\n", entry.ExpiryEpoch)
		if entry.BlobID != "" {
			fmt.Printf("\nWalruscan URL:\n")
			fmt.Printf("https://walruscan.com/testnet/blob/%s\n", entry.BlobID)
		}
		return
	}

	// Check if it might be a blob ID
	for name, entry := range index.Files {
		if entry.BlobID == nameOrID {
			fmt.Printf("Blob Information\n")
			fmt.Printf("================\n")
			fmt.Printf("Blob ID: %s\n", entry.BlobID)
			fmt.Printf("File Name: %s\n", name)
			fmt.Printf("Size: %s\n", formatBytes(entry.Size))
			fmt.Printf("Uploaded: %s\n", entry.ModTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("Expires: Epoch %d\n", entry.ExpiryEpoch)
			fmt.Printf("\nWalruscan URL:\n")
			fmt.Printf("https://walruscan.com/testnet/blob/%s\n", entry.BlobID)
			return
		}
	}

	fmt.Printf("Error: File or blob ID '%s' not found in index\n", nameOrID)
	fmt.Println("Use 'walrus-cli list' to see available files")
}

func handleStatus(config *backend.Config) {
	fmt.Println("Walrus CLI Configuration Status")
	fmt.Println("===============================")
	fmt.Println()

	// Detect network based on URLs
	var network string
	if strings.Contains(config.Walrus.AggregatorURL, "testnet") {
		network = "Testnet"
	} else if strings.Contains(config.Walrus.AggregatorURL, "walrus.space") {
		network = "Mainnet"
	} else {
		network = "Custom"
	}

	fmt.Printf("Network:       %s\n", network)
	fmt.Printf("Aggregator:    %s\n", config.Walrus.AggregatorURL)
	fmt.Printf("Publisher:     %s\n", config.Walrus.PublisherURL)
	fmt.Printf("Default Epochs: %d\n", config.Walrus.Epochs)

	if config.Walrus.Wallet.PrivateKey != "" {
		fmt.Printf("Wallet:        Configured (%s...)\n", config.Walrus.Wallet.PrivateKey[:15])
		fmt.Printf("Status:        ✅ Ready for uploads\n")
	} else {
		fmt.Printf("Wallet:        Not configured\n")
		if network == "Testnet" {
			fmt.Printf("Status:        ⚠️  Can view/estimate, uploads may fail\n")
		} else {
			fmt.Printf("Status:        ❌ Wallet required for mainnet\n")
		}
	}

	fmt.Println()

	// Show index location and stats
	index := loadIndex()
	fmt.Printf("Local Index:   %d files tracked\n", len(index.Files))

	var totalSize int64
	var validBlobs int
	for _, entry := range index.Files {
		totalSize += entry.Size
		if entry.BlobID != "" {
			validBlobs++
		}
	}

	fmt.Printf("Total Size:    %s\n", formatBytes(totalSize))
	fmt.Printf("Valid Blobs:   %d/%d\n", validBlobs, len(index.Files))

	if len(index.Files) > 0 {
		fmt.Println()
		fmt.Println("Recent uploads:")
		count := 0
		for name, entry := range index.Files {
			if entry.BlobID != "" && count < 3 {
				fmt.Printf("  • %s (%s)\n", name, formatBytes(entry.Size))
				count++
			}
		}
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  walrus-cli setup    # Reconfigure settings")
	fmt.Println("  walrus-cli list     # View all files")
	fmt.Println("  walrus-cli upload   # Upload a file")
}

func handleCost(client *backend.WalrusClient, size int64, epochs int) {
	if size == 0 {
		fmt.Println("Please provide file size with --size flag")
		os.Exit(1)
	}

	cost, err := client.EstimateStorageCost(size, epochs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error estimating cost: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Storage Cost Estimation\n")
	fmt.Printf("=======================\n")
	fmt.Printf("File Size: %s\n", formatBytes(size))
	fmt.Printf("Duration: %d epochs\n", epochs)
	fmt.Printf("Estimated Cost: %s\n", formatWALWithUSD(cost))
}

func handleInit() {
	// Create default configuration
	config := backend.DefaultConfig()

	// Get config path
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(home, ".walrus-rclone", "config.yaml")

	// Save config
	if err := backend.SaveConfig(config, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Configuration initialized at: %s\n", configPath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Edit the config file to add your Sui wallet private key (optional)")
	fmt.Println("2. Start uploading files with: walrus-cli upload <file>")
}

// Helper functions

func getIndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".walrus-rclone-index.json")
}

func loadIndex() *FileIndex {
	data, err := os.ReadFile(getIndexPath())
	if err != nil {
		return &FileIndex{Files: make(map[string]*FileEntry)}
	}

	var index FileIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return &FileIndex{Files: make(map[string]*FileEntry)}
	}

	return &index
}

func saveIndex(index *FileIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getIndexPath(), data, 0644)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatWAL(frost int64) string {
	wal := float64(frost) / 1_000_000_000

	// Format WAL with appropriate precision
	if wal >= 1 {
		return fmt.Sprintf("%.6f", wal) // Standard precision for larger amounts
	} else if wal >= 0.001 {
		return fmt.Sprintf("%.9f", wal) // Higher precision for smaller amounts
	} else {
		return fmt.Sprintf("%.3e", wal) // Scientific notation for very small amounts
	}
}

func formatWALWithUSD(frost int64) string {
	wal := float64(frost) / 1_000_000_000
	// Current WAL price in USD (approximate, should be updated from API)
	walPriceUSD := 0.425 // $0.425 per WAL as of September 2025
	usdValue := wal * walPriceUSD

	walFormatted := formatWAL(frost)

	if usdValue >= 0.01 {
		return fmt.Sprintf("%s WAL (~$%.2f)", walFormatted, usdValue)
	} else {
		return fmt.Sprintf("%s WAL (~$%.4f)", walFormatted, usdValue)
	}
}

func printUsage() {
	fmt.Println("Walrus Storage CLI - Decentralized file storage")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  walrus-cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  setup                    Interactive setup wizard")
	fmt.Println("  init                     Initialize default configuration")
	fmt.Println("  upload <file> [flags]    Upload a file to Walrus")
	fmt.Println("    --epochs <n>           Number of epochs to store (default: 5)")
	fmt.Println("    --dry-run              Estimate cost without uploading")
	fmt.Println()
	fmt.Println("  download <name> [flags]  Download a file from Walrus")
	fmt.Println("    --output <path>        Output file path")
	fmt.Println()
	fmt.Println("  list                     List stored files")
	fmt.Println()
	fmt.Println("  info <name/blob-id>      Show detailed blob information")
	fmt.Println()
	fmt.Println("  status                   Show configuration status")
	fmt.Println()
	fmt.Println("  cost [flags]             Estimate storage cost")
	fmt.Println("    --size <bytes>         File size in bytes")
	fmt.Println("    --epochs <n>           Number of epochs (default: 5)")
	fmt.Println()
	fmt.Println("  web [--background]       Launch the Walrus web UI")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  walrus-cli setup         # Interactive setup wizard")
	fmt.Println("  walrus-cli init          # Quick default setup")
	fmt.Println("  walrus-cli upload document.pdf --epochs 10")
	fmt.Println("  walrus-cli list")
	fmt.Println("  walrus-cli download document.pdf")
	fmt.Println("  walrus-cli cost --size 1048576 --epochs 5")
}
