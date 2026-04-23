package imagemin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Handler struct {
	mu            sync.Mutex
	config        *Config
	log           func(messages ...any)
	listModulesFn func(rootDir string) ([]string, error)
}

type Config struct {
	RootDir   string
	OutputDir string
	Quality   int
}

func New(c *Config) *Handler {
	return &Handler{
		config: c,
		log:    func(messages ...any) {},
	}
}

func (h *Handler) SetLog(fn func(messages ...any)) {
	h.log = fn
}

func (h *Handler) SetListModulesFn(fn func(rootDir string) ([]string, error)) {
	h.listModulesFn = fn
}

func (h *Handler) Name() string { return "imagemin" }

func (h *Handler) SupportedExtensions() []string { return []string{} }

func (h *Handler) NewFileEvent(fileName, extension, filePath, event string) error { return nil }

func (h *Handler) UnobservedFiles() []string {
	return []string{h.config.OutputDir, filepath.Join(h.config.OutputDir, ".cache.json")}
}

func (h *Handler) MainInputFileRelativePath() string { return "" }

func (h *Handler) LoadImages() error {
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

func (h *Handler) ReloadModule(moduleDir string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	assets, err := ExtractImages(moduleDir)
	if err != nil {
		return err
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

func (h *Handler) WaitForLoad(timeout time.Duration) {}
