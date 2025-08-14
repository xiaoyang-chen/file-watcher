//go:build !windows
// +build !windows

package watcher

import (
	"path/filepath"
	"strings"
)

func isHiddenFile(path string) (bool, error)   { return strings.HasPrefix(filepath.Base(path), "."), nil }
func isHiddenFileEx(path string) (bool, error) { return isHiddenFile(path) }
