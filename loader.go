//go:build !wasm

package imagemin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (h *Handler) listModulesReal(rootDir string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = rootDir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var dirs []string
	dec := json.NewDecoder(&out)
	for dec.More() {
		var m struct {
			Dir string
		}
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		if m.Dir != "" {
			dirs = append(dirs, m.Dir)
		}
	}

	return dirs, nil
}

func (h *Handler) InitDefaultLoader() {
	h.listModulesFn = h.listModulesReal
}

// LoadImages discovers modules via go list and processes their images.
func (h *Handler) LoadImages() error {
	if h.config.RootDir == "" {
		return fmt.Errorf("config.RootDir is empty")
	}

	if h.listModulesFn == nil {
		return fmt.Errorf("listModulesFn not set")
	}

	moduleDirs, err := h.listModulesFn(h.config.RootDir)
	if err != nil {
		h.log("warning: failed to list modules:", err)
		return nil
	}

	for _, dir := range moduleDirs {
		err := h.ReloadModule(dir)
		if err != nil {
			h.log("warning: failed to process module", dir, ":", err)
		}
	}

	return nil
}

// ReloadModule re-extracts and re-processes images for a single module.
func (h *Handler) ReloadModule(moduleDir string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	assets, err := ExtractImages(moduleDir)
	if err != nil {
		return err
	}

	if len(assets) > 0 {
		if err := os.MkdirAll(h.config.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	cache, err := LoadCache(h.config.OutputDir)
	if err != nil {
		return err
	}

	for _, asset := range assets {
		if cache.IsUpToDate(asset.AbsPath, asset.Variants, h.config.OutputDir) {
			continue
		}

		outputs, err := ProcessImage(asset, h.config.OutputDir, h.config.Quality, h.log)
		if err != nil {
			h.log("error processing image", asset.BaseName, ":", err)
			continue
		}

		hash, _ := computeHash(asset.AbsPath)
		cache.Update(asset.AbsPath, hash, asset.Variants, outputs, asset.Alt, asset.BaseName)
	}

	// Orphan cleaning
	currentAssetPaths := make(map[string]bool)
	for _, asset := range assets {
		currentAssetPaths[asset.AbsPath] = true
	}

	for absPath, entry := range cache.Entries {
		// Only clean orphans that belong to the current moduleDir
		if strings.HasPrefix(absPath, moduleDir) {
			if !currentAssetPaths[absPath] {
				for _, f := range entry.OutputFiles {
					os.Remove(filepath.Join(h.config.OutputDir, f))
				}
				delete(cache.Entries, absPath)
			}
		}
	}

	err = cache.Save(h.config.OutputDir)
	if err != nil {
		return err
	}

	// Build full manifest from cache
	cache.mu.RLock()
	var fullManifest []manifestEntry
	uniqueBaseNames := make(map[string]string)
	for _, entry := range cache.Entries {
		uniqueBaseNames[entry.BaseName] = entry.Alt
	}
	cache.mu.RUnlock()

	for name, alt := range uniqueBaseNames {
		fullManifest = append(fullManifest, manifestEntry{Name: name, Alt: alt})
	}

	return writeManifest(h.config.OutputDir, fullManifest)
}
