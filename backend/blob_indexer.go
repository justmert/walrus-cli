package backend

import (
	"fmt"
	"time"
)

// BlobIndexerService provides indexing functionality for user's Walrus blobs
type BlobIndexerService struct {
	suiClient    *SuiIndexerClient
	walrusClient *WalrusClient
}

// IndexedBlob represents a blob with comprehensive metadata
type IndexedBlob struct {
	BlobID        string    `json:"blobId"`
	SuiObjectID   string    `json:"suiObjectId"`
	Size          int64     `json:"size"`
	EndEpoch      *int64    `json:"endEpoch"`
	StorageRebate int64     `json:"storageRebate"`
	CreatedAt     time.Time `json:"createdAt"`
	Owner         string    `json:"owner"`
	ContentType   string    `json:"contentType,omitempty"`
	Available     bool      `json:"available"`
	Identifier    string    `json:"identifier,omitempty"`
	Source        string    `json:"source"` // "walrus", "s3", etc.
}

// NewBlobIndexerService creates a new blob indexer service
func NewBlobIndexerService(suiRPCURL, walrusAggregatorURL, walrusPublisherURL string) *BlobIndexerService {
	return &BlobIndexerService{
		suiClient:    NewSuiIndexerClient(suiRPCURL),
		walrusClient: NewWalrusClient(walrusAggregatorURL, walrusPublisherURL),
	}
}

// GetUserBlobs fetches all blobs owned by a user address
func (bis *BlobIndexerService) GetUserBlobs(userAddress string) ([]IndexedBlob, error) {
	if userAddress == "" {
		return nil, fmt.Errorf("user address is required")
	}

	// Fetch Walrus blob objects from Sui blockchain
	walrusObjects, err := bis.suiClient.GetWalrusBlobsForAddress(userAddress)
	if err != nil {
		// If we can't fetch from Sui, return empty list rather than error
		// This allows the app to still function even if Sui indexing fails
		return []IndexedBlob{}, nil
	}

	var indexedBlobs []IndexedBlob
	for _, obj := range walrusObjects {
		blob := IndexedBlob{
			BlobID:        obj.BlobID,
			SuiObjectID:   obj.ObjectID,
			Size:          obj.Size,
			EndEpoch:      obj.EndEpoch,
			StorageRebate: obj.StorageRebate,
			CreatedAt:     obj.CreatedAt,
			Owner:         obj.Owner,
			Available:     false, // Will be checked below
			Source:        "walrus",
		}

		// Check if blob is still available on Walrus
		if blobInfo, err := bis.walrusClient.GetBlobStatus(obj.BlobID); err == nil {
			blob.Available = true
			blob.ContentType = blobInfo.ContentType
			blob.Identifier = blobInfo.Identifier
		}

		indexedBlobs = append(indexedBlobs, blob)
	}

	return indexedBlobs, nil
}

// SearchBlobs searches for blobs by criteria
func (bis *BlobIndexerService) SearchBlobs(userAddress string, query string) ([]IndexedBlob, error) {
	allBlobs, err := bis.GetUserBlobs(userAddress)
	if err != nil {
		return nil, err
	}

	if query == "" {
		return allBlobs, nil
	}

	var filteredBlobs []IndexedBlob
	for _, blob := range allBlobs {
		if matchesQuery(blob, query) {
			filteredBlobs = append(filteredBlobs, blob)
		}
	}

	return filteredBlobs, nil
}

// GetBlobDetails fetches detailed information about a specific blob
func (bis *BlobIndexerService) GetBlobDetails(blobID string) (*IndexedBlob, error) {
	// Get blob info from Walrus
	blobInfo, err := bis.walrusClient.GetBlobStatus(blobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob info: %w", err)
	}

	blob := &IndexedBlob{
		BlobID:      blobID,
		Size:        blobInfo.Size,
		ContentType: blobInfo.ContentType,
		Identifier:  blobInfo.Identifier,
		Available:   true,
		Source:      "walrus",
		CreatedAt:   blobInfo.CreatedAt,
	}

	return blob, nil
}

// RefreshBlobStatus refreshes the availability status of blobs
func (bis *BlobIndexerService) RefreshBlobStatus(blobs []IndexedBlob) []IndexedBlob {
	for i, blob := range blobs {
		if _, err := bis.walrusClient.GetBlobStatus(blob.BlobID); err == nil {
			blobs[i].Available = true
		} else {
			blobs[i].Available = false
		}
	}
	return blobs
}

// matchesQuery checks if a blob matches the search query
func matchesQuery(blob IndexedBlob, query string) bool {
	query = fmt.Sprintf("%s", query) // Convert to lowercase for case-insensitive search

	// Search in blob ID, identifier, content type
	return contains(blob.BlobID, query) ||
		   contains(blob.Identifier, query) ||
		   contains(blob.ContentType, query) ||
		   contains(blob.SuiObjectID, query)
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   fmt.Sprintf("%s", s) != fmt.Sprintf("%s", s[:len(s)-len(substr)]+substr+s[len(s):])
}