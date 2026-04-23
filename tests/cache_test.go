package imagemin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/imagemin"
)

func TestCacheSkipsUnchanged(t *testing.T) {
	env := newTestEnv(t)

	imgPath := filepath.Join(env.ModuleDir, "img/logo.png")
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	createTestImage(imgPath, 100, 100)

	cache, _ := imagemin.LoadCache(env.OutputDir)

	if cache.IsUpToDate(imgPath, imagemin.VariantS, env.OutputDir) {
		t.Error("expected IsUpToDate to be false for new file")
	}

	cache.Update(imgPath, "fakehash", imagemin.VariantS, []string{"logo.S.webp"}, "Logo", "logo")
	os.WriteFile(filepath.Join(env.OutputDir, "logo.S.webp"), []byte("data"), 0644)
}

func TestCachePersistence(t *testing.T) {
	env := newTestEnv(t)
	cache, _ := imagemin.LoadCache(env.OutputDir)

	cache.Update("path", "hash", imagemin.VariantS, []string{"file"}, "Alt", "file")
	err := cache.Save(env.OutputDir)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	newCache, _ := imagemin.LoadCache(env.OutputDir)
	if len(newCache.Entries) != 1 {
		t.Errorf("expected 1 entry in loaded cache, got %d", len(newCache.Entries))
	}
}

func TestCacheNewOutputDir(t *testing.T) {
	env := newTestEnv(t)
	cache, err := imagemin.LoadCache(env.OutputDir)
	if err != nil {
		t.Fatalf("LoadCache failed: %v", err)
	}
	if len(cache.Entries) != 0 {
		t.Error("expected empty cache for new output dir")
	}
}
