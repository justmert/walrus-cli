package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SuiIndexerClient handles communication with Sui RPC for Walrus blob indexing
type SuiIndexerClient struct {
	RPCURL     string
	HTTPClient *http.Client
}

// SuiObject represents a Sui blockchain object
type SuiObject struct {
	ObjectID string `json:"objectId"`
	Version  string `json:"version"`
	Digest   string `json:"digest"`
	Type     string `json:"type"`
	Owner    interface{} `json:"owner"`
	Content  map[string]interface{} `json:"content"`
}

// SuiRPCRequest represents a JSON-RPC request to Sui
type SuiRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// SuiRPCResponse represents a JSON-RPC response from Sui
type SuiRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *SuiRPCError    `json:"error"`
}

// SuiRPCError represents an error in Sui RPC response
type SuiRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WalrusBlobObject represents a Walrus blob object on Sui
type WalrusBlobObject struct {
	ObjectID     string    `json:"objectId"`
	BlobID       string    `json:"blobId"`
	Size         int64     `json:"size"`
	EndEpoch     *int64    `json:"endEpoch"`
	StorageRebate int64    `json:"storageRebate"`
	CreatedAt    time.Time `json:"createdAt"`
	Owner        string    `json:"owner"`
}

// NewSuiIndexerClient creates a new Sui indexer client
func NewSuiIndexerClient(rpcURL string) *SuiIndexerClient {
	return &SuiIndexerClient{
		RPCURL: rpcURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetOwnedObjects fetches objects owned by a specific address
func (c *SuiIndexerClient) GetOwnedObjects(address string, objectType string) ([]SuiObject, error) {
	filter := map[string]interface{}{
		"MatchAll": []map[string]interface{}{
			{
				"StructType": objectType,
			},
		},
	}

	options := map[string]interface{}{
		"showType":    true,
		"showContent": true,
		"showOwner":   true,
	}

	request := SuiRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "suix_getOwnedObjects",
		Params:  []interface{}{address, filter, nil, nil, options},
	}

	return c.executeRPCRequest(request)
}

// GetWalrusBlobsForAddress fetches Walrus blob objects for a specific address
func (c *SuiIndexerClient) GetWalrusBlobsForAddress(address string) ([]WalrusBlobObject, error) {
	// Query for Walrus blob objects
	// The exact type may vary, but typically something like "0x...::blob::Blob" or similar
	walrusBlobType := "0x*::walrus::Blob" // This is a placeholder - we'll need the actual type

	objects, err := c.GetOwnedObjects(address, walrusBlobType)
	if err != nil {
		return nil, fmt.Errorf("failed to get owned objects: %w", err)
	}

	var blobs []WalrusBlobObject
	for _, obj := range objects {
		if blobObj, err := c.parseWalrusBlobObject(obj); err == nil {
			blobs = append(blobs, blobObj)
		}
	}

	return blobs, nil
}

// parseWalrusBlobObject extracts Walrus blob information from a Sui object
func (c *SuiIndexerClient) parseWalrusBlobObject(obj SuiObject) (WalrusBlobObject, error) {
	blob := WalrusBlobObject{
		ObjectID: obj.ObjectID,
	}

	// Extract blob information from content
	if obj.Content != nil {
		if fields, ok := obj.Content["fields"].(map[string]interface{}); ok {
			if blobID, ok := fields["blob_id"].(string); ok {
				blob.BlobID = blobID
			}
			if size, ok := fields["size"].(float64); ok {
				blob.Size = int64(size)
			}
			if endEpoch, ok := fields["end_epoch"].(float64); ok {
				epoch := int64(endEpoch)
				blob.EndEpoch = &epoch
			}
			if rebate, ok := fields["storage_rebate"].(float64); ok {
				blob.StorageRebate = int64(rebate)
			}
		}
	}

	// Extract owner information
	if owner, ok := obj.Owner.(map[string]interface{}); ok {
		if addressOwner, ok := owner["AddressOwner"].(string); ok {
			blob.Owner = addressOwner
		}
	}

	if blob.BlobID == "" {
		return blob, fmt.Errorf("blob ID not found in object")
	}

	return blob, nil
}

// executeRPCRequest executes a JSON-RPC request to Sui
func (c *SuiIndexerClient) executeRPCRequest(request SuiRPCRequest) ([]SuiObject, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.RPCURL, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp SuiRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var result struct {
		Data    []map[string]interface{} `json:"data"`
		HasNextPage bool                `json:"hasNextPage"`
	}

	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	var objects []SuiObject
	for _, item := range result.Data {
		if data, ok := item["data"].(map[string]interface{}); ok {
			obj := SuiObject{
				ObjectID: getString(data, "objectId"),
				Version:  getString(data, "version"),
				Digest:   getString(data, "digest"),
				Type:     getString(data, "type"),
				Owner:    data["owner"],
				Content:  getMap(data, "content"),
			}
			objects = append(objects, obj)
		}
	}

	return objects, nil
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}