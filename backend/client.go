package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WalrusClient handles communication with Walrus storage network
type WalrusClient struct {
	AggregatorURL  string
	PublisherURL   string
	UploadRelayURL string // Optional upload relay to reduce client requests
	HTTPClient     *http.Client
	UseUploadRelay bool
}

// BlobInfo represents information about a stored blob
type BlobInfo struct {
	BlobID      string            `json:"blobId"`
	Size        int64             `json:"size"`
	EndEpoch    int               `json:"endEpoch"`
	CreatedAt   time.Time         `json:"createdAt"`
	ContentType string            `json:"contentType"`
	Identifier  string            `json:"identifier,omitempty"` // File name/identifier if available
	Tags        map[string]string `json:"tags,omitempty"`       // Metadata tags
	IsQuilt     bool              `json:"isQuilt,omitempty"`    // Whether this blob contains multiple files
	FileCount   int               `json:"fileCount,omitempty"`  // Number of files if it's a quilt
}

// StoreResponse represents the response from storing a blob
type StoreResponse struct {
	BlobID           string `json:"blobId"`
	EndEpoch         *int64 `json:"endEpoch"`
	RegisteredEpoch  *int64 `json:"registeredEpoch,omitempty"`
	Cost             int64  `json:"cost"`
	Size             int64  `json:"size"`
	AlreadyCertified bool   `json:"alreadyCertified"`
	SuiObjectID      string `json:"suiObjectId,omitempty"`
}

type storeResponseEnvelope struct {
	NewlyCreated     json.RawMessage `json:"newlyCreated"`
	AlreadyCertified json.RawMessage `json:"alreadyCertified"`
	Cost             *int64          `json:"cost"`
}

type walrusStoragePayload struct {
	EndEpoch       int   `json:"endEpoch"`
	EndEpochAlt    int   `json:"storage_end_epoch"`
	StorageSize    int64 `json:"storage_size"`
	StorageSizeAlt int64 `json:"storageSize"`
}

func (p walrusStoragePayload) endEpoch() int {
	if p.EndEpoch != 0 {
		return p.EndEpoch
	}
	if p.EndEpochAlt != 0 {
		return p.EndEpochAlt
	}
	return 0
}

func (p walrusStoragePayload) size() int64 {
	if p.StorageSize != 0 {
		return p.StorageSize
	}
	if p.StorageSizeAlt != 0 {
		return p.StorageSizeAlt
	}
	return 0
}

type walrusNewlyCreatedLegacy struct {
	BlobObject struct {
		BlobID          string               `json:"blobId"`
		RegisteredEpoch int                  `json:"registeredEpoch"`
		Storage         walrusStoragePayload `json:"storage"`
		Size            int64                `json:"size"`
	} `json:"blobObject"`
	Cost int64 `json:"cost"`
}

type walrusNewlyCreatedModern struct {
	BlobID          string               `json:"blobId"`
	RegisteredEpoch int                  `json:"registeredEpoch"`
	Storage         walrusStoragePayload `json:"storage"`
	Size            int64                `json:"size"`
	Cost            int64                `json:"cost"`
}

type walrusAlreadyCertified struct {
	BlobID          string               `json:"blobId"`
	RegisteredEpoch int                  `json:"registeredEpoch"`
	EndEpoch        int                  `json:"endEpoch"`
	Storage         walrusStoragePayload `json:"storage"`
	Size            int64                `json:"size"`
}

// NewWalrusClient creates a new Walrus client
func NewWalrusClient(aggregatorURL, publisherURL string) *WalrusClient {
	return &WalrusClient{
		AggregatorURL:  aggregatorURL,
		PublisherURL:   publisherURL,
		UploadRelayURL: "https://upload-relay.testnet.walrus.space", // Default upload relay
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second, // Increased timeout to match TS SDK
		},
		UseUploadRelay: false, // Disabled until the relay flow is fully implemented
	}
}

// StoreBlob uploads data to Walrus storage, optionally using upload relay
func (c *WalrusClient) StoreBlob(data []byte, epochs int) (*StoreResponse, error) {
	// Use upload relay if configured and available
	baseURL := c.PublisherURL
	if c.UseUploadRelay && c.UploadRelayURL != "" {
		baseURL = c.UploadRelayURL
	}

	url := fmt.Sprintf("%s/v1/blobs?epochs=%d", baseURL, epochs)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("uploading blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	storeResp, err := decodeStoreResponse(body, int64(len(data)))
	if err != nil {
		return nil, err
	}

	return storeResp, nil
}

func decodeStoreResponse(payload []byte, fallbackSize int64) (*StoreResponse, error) {
	var envelope storeResponseEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("decoding response: %w (body: %s)", err, string(payload))
	}

	resp := &StoreResponse{}
	if envelope.Cost != nil {
		resp.Cost = *envelope.Cost
	}

	if len(envelope.NewlyCreated) > 0 {
		if parsed, err := parseNewlyCreated(envelope.NewlyCreated, fallbackSize); err == nil {
			if resp.Cost == 0 {
				resp.Cost = parsed.Cost
			}
			parsed.Cost = resp.Cost
			return parsed, nil
		} else {
			return nil, fmt.Errorf("unexpected newlyCreated payload: %s", string(envelope.NewlyCreated))
		}
	}

	if len(envelope.AlreadyCertified) > 0 {
		parsed, err := parseAlreadyCertified(envelope.AlreadyCertified, fallbackSize)
		if err != nil {
			return nil, fmt.Errorf("unexpected alreadyCertified payload: %s", string(envelope.AlreadyCertified))
		}
		parsed.Cost = resp.Cost
		return parsed, nil
	}

	return nil, fmt.Errorf("unexpected response format: %s", string(payload))
}

func parseNewlyCreated(raw json.RawMessage, fallbackSize int64) (*StoreResponse, error) {
	var modern walrusNewlyCreatedModern
	if err := json.Unmarshal(raw, &modern); err == nil && modern.BlobID != "" {
		endEpoch := int64(modern.Storage.endEpoch())
		resp := &StoreResponse{
			BlobID:           modern.BlobID,
			EndEpoch:         &endEpoch,
			Size:             resolveSize(fallbackSize, modern.Storage.size(), modern.Size),
			AlreadyCertified: false,
			Cost:             modern.Cost,
		}
		return resp, nil
	}

	var legacy walrusNewlyCreatedLegacy
	if err := json.Unmarshal(raw, &legacy); err == nil && legacy.BlobObject.BlobID != "" {
		endEpoch := int64(legacy.BlobObject.Storage.endEpoch())
		resp := &StoreResponse{
			BlobID:           legacy.BlobObject.BlobID,
			EndEpoch:         &endEpoch,
			Size:             resolveSize(fallbackSize, legacy.BlobObject.Storage.size(), legacy.BlobObject.Size),
			AlreadyCertified: false,
			Cost:             legacy.Cost,
		}
		return resp, nil
	}

	return nil, fmt.Errorf("unable to decode newlyCreated payload")
}

func parseAlreadyCertified(raw json.RawMessage, fallbackSize int64) (*StoreResponse, error) {
	var modern walrusAlreadyCertified
	if err := json.Unmarshal(raw, &modern); err == nil && modern.BlobID != "" {
		endEpoch := modern.EndEpoch
		if endEpoch == 0 {
			endEpoch = modern.Storage.endEpoch()
		}
		endEpoch64 := int64(endEpoch)
		resp := &StoreResponse{
			BlobID:           modern.BlobID,
			EndEpoch:         &endEpoch64,
			Size:             resolveSize(fallbackSize, modern.Storage.size(), modern.Size),
			AlreadyCertified: true,
		}
		return resp, nil
	}

	var legacy walrusNewlyCreatedLegacy
	if err := json.Unmarshal(raw, &legacy); err == nil && legacy.BlobObject.BlobID != "" {
		endEpoch := int64(legacy.BlobObject.Storage.endEpoch())
		resp := &StoreResponse{
			BlobID:           legacy.BlobObject.BlobID,
			EndEpoch:         &endEpoch,
			Size:             resolveSize(fallbackSize, legacy.BlobObject.Storage.size(), legacy.BlobObject.Size),
			AlreadyCertified: true,
			Cost:             legacy.Cost,
		}
		return resp, nil
	}

	return nil, fmt.Errorf("unable to decode alreadyCertified payload")
}

func resolveSize(fallback int64, candidates ...int64) int64 {
	for _, val := range candidates {
		if val > 0 {
			return val
		}
	}
	return fallback
}

// RetrieveBlob downloads a blob from Walrus storage with retry logic
func (c *WalrusClient) RetrieveBlob(blobID string) ([]byte, error) {
	url := fmt.Sprintf("%s/v1/blobs/%s", c.AggregatorURL, blobID)

	// Retry logic for transient failures
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		resp, err := c.HTTPClient.Get(url)
		if err != nil {
			// Check if it's a retryable network error
			if isRetryableError(err) {
				lastErr = err
				continue
			}
			return nil, fmt.Errorf("retrieving blob: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			// Retryable status codes
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, body)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("retrieval failed with status %d: %s", resp.StatusCode, body)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading blob data: %w", err)
		}

		return data, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
	}
	return nil, errors.New("failed to retrieve blob")
}

// GetBlobStatus checks if a blob exists and returns its info
func (c *WalrusClient) GetBlobStatus(blobID string) (*BlobInfo, error) {
	// Try to retrieve just the headers to check if blob exists
	url := fmt.Sprintf("%s/v1/blobs/%s", c.AggregatorURL, blobID)

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking blob status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("blob not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status check failed with code %d", resp.StatusCode)
	}

	// Extract metadata from response headers if available
	info := &BlobInfo{
		BlobID: blobID,
	}

	// Try to parse content-type header
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		info.ContentType = contentType
	}

	// Try to parse content-length
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := fmt.Sscanf(contentLength, "%d", &info.Size); err == nil && size == 1 {
			// Size parsed successfully
		}
	}

	return info, nil
}

// EstimateStorageCost estimates the cost for storing data based on actual Walrus pricing
// Returns costs in FROST units (smallest denomination)
func (c *WalrusClient) EstimateStorageCost(sizeBytes int64, epochs int) (int64, error) {
	// Based on Walrus pricing research:
	// - Current price: ~55,000 FROST per MB per epoch with 80% subsidy
	// - Encoded size is ~5x larger than original due to erasure coding
	// - Fixed metadata overhead of ~64MB for small files
	// - Upload relay reduces network overhead

	// Calculate encoded size (5x larger + metadata overhead)
	encodedSizeBytes := sizeBytes * 5
	fixedMetadataBytes := int64(64 * 1024 * 1024) // 64MB metadata overhead

	// For small files, metadata dominates the cost
	if sizeBytes < 10*1024*1024 { // Files < 10MB
		encodedSizeBytes = fixedMetadataBytes
	} else {
		encodedSizeBytes += fixedMetadataBytes
	}

	// Convert to MB for pricing calculation
	encodedSizeMB := (encodedSizeBytes + 1048575) / 1048576 // Round up to nearest MB

	// Current Walrus pricing: 55,000 FROST per MB per epoch
	baseCostPerMBPerEpoch := int64(55_000) // FROST per MB per epoch

	// Apply 80% subsidy (pay only 20% of base cost)
	subsidizedCostPerMBPerEpoch := baseCostPerMBPerEpoch / 5 // 20% of base cost

	totalCostFrost := encodedSizeMB * subsidizedCostPerMBPerEpoch * int64(epochs)

	return totalCostFrost, nil
}

// isRetryableError checks if an error is retryable (network issues)
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"no such host",
		"network is unreachable",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}
