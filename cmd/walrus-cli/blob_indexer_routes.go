package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/justmert/walrus-cli/backend"
)

func setupBlobIndexerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/blobs/list", corsHandler(handleListUserBlobs))
	mux.HandleFunc("/api/blobs/search", corsHandler(handleSearchUserBlobs))
	mux.HandleFunc("/api/blobs/details", corsHandler(handleGetBlobDetails))
}

func corsHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

type ListBlobsRequest struct {
	UserAddress string `json:"userAddress"`
}

type ListBlobsResponse struct {
	Success bool                    `json:"success"`
	Data    []backend.IndexedBlob   `json:"data,omitempty"`
	Error   string                  `json:"error,omitempty"`
}

type SearchBlobsRequest struct {
	UserAddress string `json:"userAddress"`
	Query       string `json:"query"`
}

type BlobDetailsRequest struct {
	BlobID string `json:"blobId"`
}

func handleListUserBlobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ListBlobsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Invalid request body: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.UserAddress == "" {
		response := ListBlobsResponse{
			Success: false,
			Error:   "User address is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Load config for network settings
	config, err := backend.LoadConfig("")
	if err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Failed to load config: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Determine Sui RPC URL based on aggregator URL (heuristic)
	suiRPCURL := "https://fullnode.testnet.sui.io:443"
	if strings.Contains(config.Walrus.AggregatorURL, "mainnet") {
		suiRPCURL = "https://fullnode.mainnet.sui.io:443"
	}

	// Create blob indexer service
	indexer := backend.NewBlobIndexerService(
		suiRPCURL,
		config.Walrus.AggregatorURL,
		config.Walrus.PublisherURL,
	)

	// Fetch user blobs
	blobs, err := indexer.GetUserBlobs(req.UserAddress)
	if err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to fetch blobs: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := ListBlobsResponse{
		Success: true,
		Data:    blobs,
	}
	json.NewEncoder(w).Encode(response)
}

func handleSearchUserBlobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchBlobsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Invalid request body: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.UserAddress == "" {
		response := ListBlobsResponse{
			Success: false,
			Error:   "User address is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Load config for network settings
	config, err := backend.LoadConfig("")
	if err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Failed to load config: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Determine Sui RPC URL based on aggregator URL (heuristic)
	suiRPCURL := "https://fullnode.testnet.sui.io:443"
	if strings.Contains(config.Walrus.AggregatorURL, "mainnet") {
		suiRPCURL = "https://fullnode.mainnet.sui.io:443"
	}

	// Create blob indexer service
	indexer := backend.NewBlobIndexerService(
		suiRPCURL,
		config.Walrus.AggregatorURL,
		config.Walrus.PublisherURL,
	)

	// Search blobs
	blobs, err := indexer.SearchBlobs(req.UserAddress, req.Query)
	if err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to search blobs: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := ListBlobsResponse{
		Success: true,
		Data:    blobs,
	}
	json.NewEncoder(w).Encode(response)
}

func handleGetBlobDetails(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BlobDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Invalid request body: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.BlobID == "" {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Blob ID is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Load config for network settings
	config, err := backend.LoadConfig("")
	if err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   "Failed to load config: " + err.Error(),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Determine Sui RPC URL based on aggregator URL (heuristic)
	suiRPCURL := "https://fullnode.testnet.sui.io:443"
	if strings.Contains(config.Walrus.AggregatorURL, "mainnet") {
		suiRPCURL = "https://fullnode.mainnet.sui.io:443"
	}

	// Create blob indexer service
	indexer := backend.NewBlobIndexerService(
		suiRPCURL,
		config.Walrus.AggregatorURL,
		config.Walrus.PublisherURL,
	)

	// Get blob details
	blob, err := indexer.GetBlobDetails(req.BlobID)
	if err != nil {
		response := ListBlobsResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get blob details: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	response := ListBlobsResponse{
		Success: true,
		Data:    []backend.IndexedBlob{*blob},
	}
	json.NewEncoder(w).Encode(response)
}