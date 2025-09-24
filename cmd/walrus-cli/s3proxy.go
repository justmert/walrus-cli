package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/justmert/walrus-cli/backend"
)

type S3ProxyRequest struct {
	Action      string                   `json:"action"`
	Credentials backend.S3Credentials    `json:"credentials"`
	Bucket      string                   `json:"bucket,omitempty"`
	Prefix      string                   `json:"prefix,omitempty"`
	Key         string                   `json:"key,omitempty"`
	Filter      *backend.S3TransferFilter `json:"filter,omitempty"`
}

type S3ProxyResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type S3BucketInfo struct {
	Name string `json:"name"`
}

type S3ObjectInfo struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"etag,omitempty"`
}

func handleS3Proxy(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for the web UI
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req S3ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendS3ProxyError(w, "Invalid request body: "+err.Error())
		return
	}

	// Validate credentials
	if req.Credentials.AccessKeyID == "" || req.Credentials.SecretAccessKey == "" {
		sendS3ProxyError(w, "AWS credentials are required")
		return
	}

	// Create S3 client
	s3Client, err := backend.NewS3Client(req.Credentials)
	if err != nil {
		sendS3ProxyError(w, "Failed to create S3 client: "+err.Error())
		return
	}

	ctx := context.Background()

	switch req.Action {
	case "listBuckets":
		handleListBuckets(ctx, w, s3Client)
	case "listObjects":
		handleListObjects(ctx, w, s3Client, req.Bucket, req.Filter)
	case "downloadObject":
		handleDownloadObject(ctx, w, s3Client, req.Bucket, req.Key)
	case "estimateTransfer":
		handleEstimateTransfer(ctx, w, s3Client, req.Bucket, req.Filter)
	default:
		sendS3ProxyError(w, "Unknown action: "+req.Action)
	}
}

func handleListBuckets(ctx context.Context, w http.ResponseWriter, client *backend.S3Client) {
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		sendS3ProxyError(w, err.Error())
		return
	}

	bucketInfos := make([]S3BucketInfo, len(buckets))
	for i, name := range buckets {
		bucketInfos[i] = S3BucketInfo{Name: name}
	}

	sendS3ProxySuccess(w, bucketInfos)
}

func handleListObjects(ctx context.Context, w http.ResponseWriter, client *backend.S3Client, bucket string, filter *backend.S3TransferFilter) {
	if bucket == "" {
		sendS3ProxyError(w, "Bucket name is required")
		return
	}

	if filter == nil {
		filter = &backend.S3TransferFilter{}
	}

	objects, err := client.ListObjects(ctx, bucket, filter)
	if err != nil {
		sendS3ProxyError(w, err.Error())
		return
	}

	objectInfos := make([]S3ObjectInfo, len(objects))
	for i, obj := range objects {
		objectInfos[i] = S3ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format("2006-01-02T15:04:05Z"),
			ETag:         obj.ETag,
		}
	}

	sendS3ProxySuccess(w, objectInfos)
}

func handleDownloadObject(ctx context.Context, w http.ResponseWriter, client *backend.S3Client, bucket, key string) {
	if bucket == "" || key == "" {
		sendS3ProxyError(w, "Bucket and key are required")
		return
	}

	reader, size, err := client.DownloadObject(ctx, bucket, key)
	if err != nil {
		sendS3ProxyError(w, err.Error())
		return
	}
	defer reader.Close()

	// For large files, stream the response
	if size > 10*1024*1024 { // 10MB
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", key))

		_, err = io.Copy(w, reader)
		if err != nil {
			// Can't send error after starting to write body
			fmt.Printf("Error streaming object: %v\n", err)
		}
		return
	}

	// For smaller files, read into memory and return as base64
	data, err := io.ReadAll(reader)
	if err != nil {
		sendS3ProxyError(w, "Failed to read object: "+err.Error())
		return
	}

	sendS3ProxySuccess(w, map[string]interface{}{
		"key":  key,
		"size": size,
		"data": data, // Will be base64 encoded in JSON
	})
}

func handleEstimateTransfer(ctx context.Context, w http.ResponseWriter, client *backend.S3Client, bucket string, filter *backend.S3TransferFilter) {
	if bucket == "" {
		sendS3ProxyError(w, "Bucket name is required")
		return
	}

	if filter == nil {
		filter = &backend.S3TransferFilter{}
	}

	totalSize, fileCount, err := client.EstimateTransferSize(ctx, bucket, filter)
	if err != nil {
		sendS3ProxyError(w, err.Error())
		return
	}

	sendS3ProxySuccess(w, map[string]interface{}{
		"totalSize": totalSize,
		"fileCount": fileCount,
	})
}

func sendS3ProxySuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(S3ProxyResponse{
		Success: true,
		Data:    data,
	})
}

func sendS3ProxyError(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(S3ProxyResponse{
		Success: false,
		Error:   errMsg,
	})
}

// S3 to Walrus transfer endpoint
func handleS3Transfer(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type TransferRequest struct {
		Credentials backend.S3Credentials `json:"credentials"`
		Bucket      string                `json:"bucket"`
		Keys        []string              `json:"keys"`
		Epochs      int                   `json:"epochs"`
		Encrypt     bool                  `json:"encrypt"`
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendS3ProxyError(w, "Invalid request: "+err.Error())
		return
	}

	// Create S3 client
	s3Client, err := backend.NewS3Client(req.Credentials)
	if err != nil {
		sendS3ProxyError(w, "Failed to create S3 client: "+err.Error())
		return
	}

	// Load Walrus config
	config, err := backend.LoadConfig("")
	if err != nil {
		sendS3ProxyError(w, "Failed to load Walrus config: "+err.Error())
		return
	}

	walrusClient := backend.NewWalrusClient(config.Walrus.AggregatorURL, config.Walrus.PublisherURL)
	simpleFS := backend.NewSimpleFs(config.Walrus.AggregatorURL, config.Walrus.PublisherURL)

	// Create transfer manager
	transferManager := backend.NewTransferManager(s3Client, walrusClient, simpleFS, 1)

	// Transfer each file
	results := []map[string]interface{}{}
	for _, key := range req.Keys {
		result, err := transferManager.TransferSingle(context.Background(), req.Bucket, key, req.Epochs)
		if err != nil {
			results = append(results, map[string]interface{}{
				"key":     key,
				"success": false,
				"error":   err.Error(),
			})
		} else {
			results = append(results, map[string]interface{}{
				"key":           key,
				"success":       result.Success,
				"blobId":        result.BlobID,
				"size":          result.Size,
				"expiryEpoch":   result.ExpiryEpoch,
				"registeredEpoch": result.RegisteredEpoch,
				"suiObjectId":   result.SuiObjectID,
			})
		}
	}

	sendS3ProxySuccess(w, results)
}

// handleUpdateIndex updates the CLI index when files are uploaded from web
func handleUpdateIndex(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileName    string `json:"fileName"`
		BlobID      string `json:"blobId"`
		Size        int64  `json:"size"`
		ExpiryEpoch int    `json:"expiryEpoch"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Load existing index
	index := loadIndex()

	// Add new entry
	index.Files[req.FileName] = &FileEntry{
		BlobID:      req.BlobID,
		Size:        req.Size,
		ModTime:     time.Now(),
		ExpiryEpoch: req.ExpiryEpoch,
	}

	// Save index
	if err := saveIndex(index); err != nil {
		http.Error(w, "Failed to update index", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Add these routes to your web server
func setupS3ProxyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/s3/proxy", handleS3Proxy)
	mux.HandleFunc("/api/s3/transfer", handleS3Transfer)
	mux.HandleFunc("/api/index/update", handleUpdateIndex)
}