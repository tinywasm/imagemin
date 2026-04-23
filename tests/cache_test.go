package imagemin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
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

func TestCacheReprocessesOnChange(t *testing.T) {
	env := newTestEnv(t)
	imgPath := filepath.Join(env.ModuleDir, "test.jpg")
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	createTestImage(imgPath, 100, 100)

	env.writeSSRGoWithImages([]imagemin.Asset{{Path: "test.jpg", Variants: imagemin.VariantS}})
	env.Handler.ReloadModule(env.ModuleDir)
	env.assertWebPExists("test", imagemin.VariantS)

	// Change image
	createTestImage(imgPath, 200, 200)

	// Reload
	env.Handler.ReloadModule(env.ModuleDir)

	// Verify it updated in cache
	cache, _ := imagemin.LoadCache(env.OutputDir)
	entry := cache.Entries[imgPath]

	// Get new hash manually
	f, _ := os.Open(imgPath)
	h := sha256.New()
	io.Copy(h, f)
	f.Close()
	expectedHash := hex.EncodeToString(h.Sum(nil))

	if entry.SrcHash != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, entry.SrcHash)
	}
}

func TestCacheCleansOrphans(t *testing.T) {
	env := newTestEnv(t)
	img1 := filepath.Join(env.ModuleDir, "img1.jpg")
	img2 := filepath.Join(env.ModuleDir, "img2.jpg")
	os.MkdirAll(filepath.Dir(img1), 0755)
	createTestImage(img1, 100, 100)
	createTestImage(img2, 100, 100)

	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: "img1.jpg", Variants: imagemin.VariantS},
		{Path: "img2.jpg", Variants: imagemin.VariantS},
	})
	env.Handler.ReloadModule(env.ModuleDir)
	env.assertWebPExists("img1", imagemin.VariantS)
	env.assertWebPExists("img2", imagemin.VariantS)

	// Remove img2 from RenderImages
	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: "img1.jpg", Variants: imagemin.VariantS},
	})
	env.Handler.ReloadModule(env.ModuleDir)

	env.assertWebPExists("img1", imagemin.VariantS)
	env.assertWebPNotExists("img2", imagemin.VariantS)
}

func TestCacheCorruptJSON(t *testing.T) {
	env := newTestEnv(t)
	os.MkdirAll(env.OutputDir, 0755)
	os.WriteFile(filepath.Join(env.OutputDir, ".cache.json"), []byte("invalid json"), 0644)

	cache, err := imagemin.LoadCache(env.OutputDir)
	if err != nil {
		t.Fatalf("LoadCache failed: %v", err)
	}
	if len(cache.Entries) != 0 {
		t.Error("expected empty cache on corrupt JSON")
	}
}
