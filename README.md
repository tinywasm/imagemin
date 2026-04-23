# imagemin — WebP Image Processing for SSR Modules

`imagemin` discovers images declared in `ssr.go` files of Go modules, converts them to WebP with responsive variants (L, M, S), and writes them to `web/public/img/`.

## Features

- **Responsive Variants**: Automatically generates S (640px), M (1024px), and L (1920px) variants.
- **WebP Optimized**: Uses WebP for superior compression and quality.
- **Smart Caching**: Processes each image only once using SHA256 hashes.
- **SEO Ready**: Generates `img-manifest.json` with alt text for easy HTML integration.
- **No Upscaling**: Never enlarges images smaller than the target variant size.

## Usage in `ssr.go`

Modules declare their images by implementing a `RenderImages` function:

```go
//go:build !wasm

package mymodule

import "github.com/tinywasm/imagemin"

func RenderImages() []imagemin.Asset {
    return []imagemin.Asset{
        {
            Path: "img/logo.png",
            Variants: imagemin.VariantS | imagemin.VariantM,
            Alt: "Company Logo",
        },
        {
            Path: "img/hero.jpg",
            Variants: imagemin.AllVariants,
        },
    }
}
```

## Architecture

- `types.go`: Core types (`Asset`, `Variant`) shared across all contexts.
- `extract.go`: AST parser that finds `RenderImages` declarations in modules.
- `convert.go`: Image processing logic using `imaging` and `nativewebp`.
- `cache.go`: SHA256 hash-based cache management.
- `loader.go`: Module discovery using `go list`.
- `imagemin.go`: Main `Handler` that orchestrates the entire process.

## API

### Configuration

```go
type Config struct {
    RootDir   string // Project root (where go.mod is)
    OutputDir string // Output directory for WebP images (e.g., "web/public/img")
    Quality   int    // WebP quality (0-100)
}
```

### Handler

```go
handler := imagemin.New(&config)
handler.InitDefaultLoader() // Uses 'go list' for discovery
err := handler.LoadImages() // Initial load
```

## Manifest

An `img-manifest.json` is generated in the output directory to map image base names to their alt text:

```json
[
  {
    "name": "logo",
    "alt": "Company Logo"
  }
]
```
