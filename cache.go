//go:build !wasm

package imagemin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type Cache struct {
	mu      sync.RWMutex
	Entries map[string]CacheEntry `json:"entries"`
}

type CacheEntry struct {
	SrcHash     string   `json:"srcHash"`
	Variants    Variant  `json:"variants"`
	OutputFiles []string `json:"outputFiles"`
	Alt         string   `json:"alt"`
	BaseName    string   `json:"baseName"`
}

func LoadCache(outputDir string) (*Cache, error) {
	cachePath := filepath.Join(outputDir, ".cache.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{Entries: make(map[string]CacheEntry)}, nil
		}
		return nil, err
	}

	var cache Cache
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return &Cache{Entries: make(map[string]CacheEntry)}, nil
	}

	return &cache, nil
}

func (c *Cache) IsUpToDate(absPath string, variants Variant, outputDir string) bool {
	c.mu.RLock()
	entry, ok := c.Entries[absPath]
	c.mu.RUnlock()
	if !ok {
		return false
	}

	if entry.Variants != variants {
		return false
	}

	currentHash, err := computeHash(absPath)
	if err != nil {
		return false
	}

	if currentHash != entry.SrcHash {
		return false
	}

	for _, f := range entry.OutputFiles {
		if _, err := os.Stat(filepath.Join(outputDir, f)); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func (c *Cache) Update(absPath, hash string, variants Variant, outputs []string, alt, baseName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Entries[absPath] = CacheEntry{
		SrcHash:     hash,
		Variants:    variants,
		OutputFiles: outputs,
		Alt:         alt,
		BaseName:    baseName,
	}
}

func (c *Cache) Save(outputDir string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cachePath := filepath.Join(outputDir, ".cache.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}

func computeHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
