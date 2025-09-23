package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/walrus-rclone/mvp/backend"
)

var indexerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "Index and query user's Walrus blobs",
	Long:  "Commands for indexing and querying blobs stored on Walrus by a user account",
}

var listBlobsCmd = &cobra.Command{
	Use:   "list [user-address]",
	Short: "List all blobs owned by a user address",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		userAddress := args[0]

		config, err := backend.LoadConfig("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Default Sui RPC URLs
		suiRPCURL := "https://fullnode.testnet.sui.io:443"
		if strings.Contains(config.Walrus.AggregatorURL, "mainnet") {
			suiRPCURL = "https://fullnode.mainnet.sui.io:443"
		}

		indexer := backend.NewBlobIndexerService(
			suiRPCURL,
			config.Walrus.AggregatorURL,
			config.Walrus.PublisherURL,
		)

		query, _ := cmd.Flags().GetString("query")

		var blobs []backend.IndexedBlob
		var err2 error

		if query != "" {
			blobs, err2 = indexer.SearchBlobs(userAddress, query)
		} else {
			blobs, err2 = indexer.GetUserBlobs(userAddress)
		}

		if err2 != nil {
			fmt.Fprintf(os.Stderr, "Error fetching blobs: %v\n", err2)
			os.Exit(1)
		}

		outputFormat, _ := cmd.Flags().GetString("output")

		switch outputFormat {
		case "json":
			jsonData, err := json.MarshalIndent(blobs, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(jsonData))
		default:
			printBlobsTable(blobs)
		}
	},
}

var getBlobCmd = &cobra.Command{
	Use:   "get [blob-id]",
	Short: "Get detailed information about a specific blob",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		blobID := args[0]

		config, err := backend.LoadConfig("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		suiRPCURL := "https://fullnode.testnet.sui.io:443"
		if strings.Contains(config.Walrus.AggregatorURL, "mainnet") {
			suiRPCURL = "https://fullnode.mainnet.sui.io:443"
		}

		indexer := backend.NewBlobIndexerService(
			suiRPCURL,
			config.Walrus.AggregatorURL,
			config.Walrus.PublisherURL,
		)

		blob, err := indexer.GetBlobDetails(blobID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting blob details: %v\n", err)
			os.Exit(1)
		}

		outputFormat, _ := cmd.Flags().GetString("output")

		switch outputFormat {
		case "json":
			jsonData, err := json.MarshalIndent(blob, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(jsonData))
		default:
			printBlobDetails(*blob)
		}
	},
}

func printBlobsTable(blobs []backend.IndexedBlob) {
	if len(blobs) == 0 {
		fmt.Println("No blobs found.")
		return
	}

	fmt.Printf("Found %d blob(s):\n\n", len(blobs))

	// Print header
	fmt.Printf("%-12s %-42s %-10s %-12s %-10s %s\n",
		"STATUS", "BLOB ID", "SIZE", "END EPOCH", "SOURCE", "IDENTIFIER")
	fmt.Println(strings.Repeat("-", 100))

	for _, blob := range blobs {
		status := "❌ UNAVAILABLE"
		if blob.Available {
			status = "✅ AVAILABLE"
		}

		size := formatBytes(blob.Size)
		endEpoch := "N/A"
		if blob.EndEpoch != nil {
			endEpoch = fmt.Sprintf("%d", *blob.EndEpoch)
		}

		identifier := blob.Identifier
		if len(identifier) > 20 {
			identifier = identifier[:17] + "..."
		}
		if identifier == "" {
			identifier = "-"
		}

		blobIDDisplay := blob.BlobID
		if len(blobIDDisplay) > 40 {
			blobIDDisplay = blobIDDisplay[:37] + "..."
		}

		fmt.Printf("%-12s %-42s %-10s %-12s %-10s %s\n",
			status, blobIDDisplay, size, endEpoch, blob.Source, identifier)
	}
}

func printBlobDetails(blob backend.IndexedBlob) {
	fmt.Printf("Blob Details:\n")
	fmt.Printf("  Blob ID:      %s\n", blob.BlobID)
	fmt.Printf("  Sui Object:   %s\n", blob.SuiObjectID)
	fmt.Printf("  Size:         %s\n", formatBytes(blob.Size))
	fmt.Printf("  Available:    %t\n", blob.Available)
	fmt.Printf("  Source:       %s\n", blob.Source)
	fmt.Printf("  Owner:        %s\n", blob.Owner)

	if blob.EndEpoch != nil {
		fmt.Printf("  End Epoch:    %d\n", *blob.EndEpoch)
	}

	if blob.ContentType != "" {
		fmt.Printf("  Content Type: %s\n", blob.ContentType)
	}

	if blob.Identifier != "" {
		fmt.Printf("  Identifier:   %s\n", blob.Identifier)
	}

	if blob.StorageRebate > 0 {
		fmt.Printf("  Storage Rebate: %d\n", blob.StorageRebate)
	}

	if !blob.CreatedAt.IsZero() {
		fmt.Printf("  Created At:   %s\n", blob.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}


func init() {
	indexerCmd.AddCommand(listBlobsCmd)
	indexerCmd.AddCommand(getBlobCmd)

	// Add flags
	listBlobsCmd.Flags().StringP("query", "q", "", "Search query to filter blobs")
	listBlobsCmd.Flags().StringP("output", "o", "table", "Output format (table, json)")

	getBlobCmd.Flags().StringP("output", "o", "table", "Output format (table, json)")
}