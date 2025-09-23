//go:build !embed
// +build !embed

package main

import (
	"net/http"
)

// GetWebUIFS returns nil when not using embed build tag
func GetWebUIFS() (http.FileSystem, error) {
	// In development mode, we'll serve files directly from the filesystem
	// The web.go file will handle running npm dev server
	return nil, nil
}

// IsEmbedded returns false when the embed build tag is not used
func IsEmbedded() bool {
	return false
}