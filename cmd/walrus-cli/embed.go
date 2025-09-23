//go:build embed
// +build embed

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

// Embed the entire web UI dist directory
//
//go:embed web_dist/*
var webUIAssets embed.FS

// GetWebUIFS returns the embedded web UI filesystem
func GetWebUIFS() (http.FileSystem, error) {
	fsys, err := fs.Sub(webUIAssets, "web_dist")
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}

// IsEmbedded returns true when the embed build tag is used
func IsEmbedded() bool {
	return true
}