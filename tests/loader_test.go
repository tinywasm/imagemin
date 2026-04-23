package imagemin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/imagemin"
)

func TestLoadImagesFromModule(t *testing.T) {
	env := newTestEnv(t)

	imgPath := "img/logo.png"
	env.copyTestImage(imgPath, "gopher.S.png")

	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: imgPath, Variants: imagemin.VariantS, Alt: "Gopher"},
	})

	err := env.Handler.LoadImages()
	if err != nil {
		t.Fatalf("LoadImages failed: %v", err)
	}

	env.assertWebPExists("logo", imagemin.VariantS)
}

func TestReloadModuleNewImage(t *testing.T) {
	env := newTestEnv(t)

	env.copyTestImage("img/one.png", "gopher.S.png")
	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: "img/one.png", Variants: imagemin.VariantS},
	})
	env.Handler.ReloadModule(env.ModuleDir)
	env.assertWebPExists("one", imagemin.VariantS)

	env.copyTestImage("img/two.png", "gopher.S.png")
	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: "img/one.png", Variants: imagemin.VariantS},
		{Path: "img/two.png", Variants: imagemin.VariantS},
	})

	err := env.Handler.ReloadModule(env.ModuleDir)
	if err != nil {
		t.Fatalf("ReloadModule failed: %v", err)
	}

	env.assertWebPExists("one", imagemin.VariantS)
	env.assertWebPExists("two", imagemin.VariantS)
}

func TestReloadModuleRemovedImage(t *testing.T) {
	env := newTestEnv(t)

	env.copyTestImage("img/one.png", "gopher.S.png")
	env.copyTestImage("img/two.png", "gopher.S.png")
	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: "img/one.png", Variants: imagemin.VariantS},
		{Path: "img/two.png", Variants: imagemin.VariantS},
	})
	env.Handler.ReloadModule(env.ModuleDir)
	env.assertWebPExists("one", imagemin.VariantS)
	env.assertWebPExists("two", imagemin.VariantS)

	env.writeSSRGoWithImages([]imagemin.Asset{
		{Path: "img/one.png", Variants: imagemin.VariantS},
	})

	err := env.Handler.ReloadModule(env.ModuleDir)
	if err != nil {
		t.Fatalf("ReloadModule failed: %v", err)
	}

	env.assertWebPExists("one", imagemin.VariantS)
	env.assertWebPNotExists("two", imagemin.VariantS)
}

func TestManifestMultiModule(t *testing.T) {
	env := newTestEnv(t)

	module1 := t.TempDir()
	module2 := t.TempDir()

	env.Handler.SetListModulesFn(func(rootDir string) ([]string, error) {
		return []string{module1, module2}, nil
	})

	// Module 1
	os.WriteFile(filepath.Join(module1, "ssr.go"), []byte(`
package m1
import "github.com/tinywasm/imagemin"
func RenderImages() []imagemin.Asset {
	return []imagemin.Asset{{Path: "img/m1.png", Variants: imagemin.VariantS, Alt: "M1"}}
}
`), 0644)
	os.MkdirAll(filepath.Join(module1, "img"), 0755)
	createTestImage(filepath.Join(module1, "img/m1.png"), 100, 100)

	// Module 2
	os.WriteFile(filepath.Join(module2, "ssr.go"), []byte(`
package m2
import "github.com/tinywasm/imagemin"
func RenderImages() []imagemin.Asset {
	return []imagemin.Asset{{Path: "img/m2.png", Variants: imagemin.VariantS, Alt: "M2"}}
}
`), 0644)
	os.MkdirAll(filepath.Join(module2, "img"), 0755)
	createTestImage(filepath.Join(module2, "img/m2.png"), 100, 100)

	err := env.Handler.LoadImages()
	if err != nil {
		t.Fatalf("LoadImages failed: %v", err)
	}

	manifestPath := filepath.Join(env.OutputDir, "img-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	if !strings.Contains(string(data), "m1") || !strings.Contains(string(data), "m2") {
		t.Errorf("manifest should contain both images, got: %s", string(data))
	}
}
