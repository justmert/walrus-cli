package backend

// This file provides a simple interface for the CLI without Rclone dependencies

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SimpleFs provides a simple file system interface for Walrus
type SimpleFs struct {
	client  *WalrusClient
	index   *SimpleFileIndex
	indexMu sync.RWMutex
}

// SimpleFileIndex manages file mappings
type SimpleFileIndex struct {
	Files map[string]*SimpleFileEntry `json:"files"`
}

// SimpleFileEntry represents a file in the index
type SimpleFileEntry struct {
	BlobID      string    `json:"blob_id"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	ExpiryEpoch int       `json:"expiry_epoch"`
}

// NewSimpleFs creates a new simple filesystem
func NewSimpleFs(aggregatorURL, publisherURL string) *SimpleFs {
	return &SimpleFs{
		client: NewWalrusClient(aggregatorURL, publisherURL),
		index:  &SimpleFileIndex{Files: make(map[string]*SimpleFileEntry)},
	}
}

// Upload stores a file in Walrus
func (fs *SimpleFs) Upload(name string, data []byte, epochs int) (*StoreResponse, error) {
	resp, err := fs.client.StoreBlob(data, epochs)
	if err != nil {
		return nil, err
	}

	// Update index
	expiryEpoch := 0
	if resp.EndEpoch != nil {
		expiryEpoch = int(*resp.EndEpoch)
	}
	fs.indexMu.Lock()
	fs.index.Files[name] = &SimpleFileEntry{
		BlobID:      resp.BlobID,
		Size:        int64(len(data)),
		ModTime:     time.Now(),
		ExpiryEpoch: expiryEpoch,
	}
	fs.indexMu.Unlock()

	// Save index
	if err := fs.SaveIndex(); err != nil {
		// Log but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to save index: %v\n", err)
	}

	return resp, nil
}

// Download retrieves a file from Walrus
func (fs *SimpleFs) Download(name string) ([]byte, error) {
	fs.indexMu.RLock()
	entry, exists := fs.index.Files[name]
	fs.indexMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("file not found in index")
	}

	return fs.client.RetrieveBlob(entry.BlobID)
}

// List returns all files in the index
func (fs *SimpleFs) List() map[string]*SimpleFileEntry {
	fs.indexMu.RLock()
	defer fs.indexMu.RUnlock()

	// Create a copy to avoid race conditions
	files := make(map[string]*SimpleFileEntry)
	for k, v := range fs.index.Files {
		files[k] = v
	}
	return files
}

// GetIndexPath returns the path to the index file
func (fs *SimpleFs) GetIndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".walrus-simple-index.json")
}

// LoadIndex loads the index from disk
func (fs *SimpleFs) LoadIndex() error {
	fs.indexMu.Lock()
	defer fs.indexMu.Unlock()

	data, err := os.ReadFile(fs.GetIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			// Index doesn't exist yet, that's ok
			fs.index = &SimpleFileIndex{Files: make(map[string]*SimpleFileEntry)}
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &fs.index)
}

// SaveIndex saves the index to disk
func (fs *SimpleFs) SaveIndex() error {
	fs.indexMu.RLock()
	data, err := json.MarshalIndent(fs.index, "", "  ")
	fs.indexMu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(fs.GetIndexPath(), data, 0644)
}