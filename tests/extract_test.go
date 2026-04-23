package imagemin_test

import (
	"testing"

	"github.com/tinywasm/imagemin"
)

func TestExtractImagesLiteral(t *testing.T) {
	env := newTestEnv(t)
	env.writeSSRGo(`
package module
import "github.com/tinywasm/imagemin"
func RenderImages() []imagemin.Asset {
	return []imagemin.Asset{
		{Path: "img/logo.png", Variants: imagemin.VariantS | imagemin.VariantM, Alt: "Logo"},
	}
}
`)

	assets, err := imagemin.ExtractImages(env.ModuleDir)
	if err != nil {
		t.Fatalf("ExtractImages failed: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	if assets[0].BaseName != "logo" {
		t.Errorf("expected BaseName 'logo', got %q", assets[0].BaseName)
	}

	if assets[0].Variants != (imagemin.VariantS | imagemin.VariantM) {
		t.Errorf("expected Variants S|M, got %d", assets[0].Variants)
	}
}

func TestExtractImagesAllVariants(t *testing.T) {
	env := newTestEnv(t)
	env.writeSSRGo(`
package module
import "github.com/tinywasm/imagemin"
func RenderImages() []imagemin.Asset {
	return []imagemin.Asset{
		{Path: "hero.jpg", Variants: imagemin.AllVariants},
	}
}
`)

	assets, _ := imagemin.ExtractImages(env.ModuleDir)
	if len(assets) != 1 || assets[0].Variants != imagemin.AllVariants {
		t.Errorf("failed to resolve AllVariants")
	}
}

func TestExtractImagesAltEmpty(t *testing.T) {
	env := newTestEnv(t)
	env.writeSSRGo(`
package module
import "github.com/tinywasm/imagemin"
func RenderImages() []imagemin.Asset {
	return []imagemin.Asset{
		{Path: "my-hero.jpg", Variants: imagemin.VariantS},
	}
}
`)

	assets, _ := imagemin.ExtractImages(env.ModuleDir)
	if assets[0].Alt != "my hero" {
		t.Errorf("expected alt 'my hero', got %q", assets[0].Alt)
	}
}
