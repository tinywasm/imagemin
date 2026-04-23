package imagemin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/imagemin"
)

func TestConvertJPGToWebP(t *testing.T) {
	env := newTestEnv(t)
	imgPath := filepath.Join(env.ModuleDir, "img/test.jpg")
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	createTestImage(imgPath, 2000, 1000)

	asset := imagemin.ParsedAsset{
		AbsPath:  imgPath,
		Variants: imagemin.VariantL,
		Alt:      "Test",
		BaseName: "test",
	}

	_, err := imagemin.ProcessImage(asset, env.OutputDir, 82, func(m ...any) {})
	if err != nil {
		t.Fatalf("ProcessImage failed: %v", err)
	}

	env.assertWebPExists("test", imagemin.VariantL)
}

func TestConvertNoUpscale(t *testing.T) {
	env := newTestEnv(t)
	imgPath := filepath.Join(env.ModuleDir, "img/small.jpg")
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	createTestImage(imgPath, 100, 100)

	asset := imagemin.ParsedAsset{
		AbsPath:  imgPath,
		Variants: imagemin.VariantL,
		Alt:      "Small",
		BaseName: "small",
	}

	skipped := false
	_, err := imagemin.ProcessImage(asset, env.OutputDir, 82, func(m ...any) {
		for _, msg := range m {
			if msg == "image smaller than target, skipping resize" {
				skipped = true
			}
		}
	})
	if err != nil {
		t.Fatalf("ProcessImage failed: %v", err)
	}

	if !skipped {
		t.Error("expected resize to be skipped for small image")
	}
	env.assertWebPExists("small", imagemin.VariantL)
}

func TestConvertVariantSubset(t *testing.T) {
	env := newTestEnv(t)
	imgPath := filepath.Join(env.ModuleDir, "img/multi.jpg")
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	createTestImage(imgPath, 2000, 1000)

	asset := imagemin.ParsedAsset{
		AbsPath:  imgPath,
		Variants: imagemin.VariantS | imagemin.VariantM,
		Alt:      "Multi",
		BaseName: "multi",
	}

	_, err := imagemin.ProcessImage(asset, env.OutputDir, 82, func(m ...any) {})
	if err != nil {
		t.Fatalf("ProcessImage failed: %v", err)
	}

	env.assertWebPExists("multi", imagemin.VariantS)
	env.assertWebPExists("multi", imagemin.VariantM)
	env.assertWebPNotExists("multi", imagemin.VariantL)
}
