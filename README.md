# imagemin — WebP Image Processing for SSR Modules
<img src="docs/img/badges.svg">

`imagemin` discovers images declared in `ssr.go` files of Go modules, converts them to WebP with responsive variants (L, M, S), and writes them to `web/public/img/`.

## Features

- **Responsive Variants**: Automatically generates S (640px), M (1024px), and L (1920px) variants.
- **WebP Optimized**: Uses WebP for superior compression and quality.
- **Stat-based Caching**: Uses file modification times (`mtime`) to avoid redundant processing without extra state files.
- **No Upscaling**: Never enlarges images smaller than the target variant size.
- **AST-based Extraction**: Discovers images by parsing source code, no runtime execution required.

## Documentation

For a detailed look at the internal workings and design decisions, see the [Architecture Documentation](docs/ARCHITECTURE.md).

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

## Architecture Overview

- `types.go`: Core types (`Asset`, `Variant`) shared across all contexts.
- `extract.go`: AST parser that finds `RenderImages` declarations in modules.
- `convert.go`: Image processing logic using `imaging` and `nativewebp`.
- `loader.go`: Module discovery and orchestration using `go list`.
- `imagemin.go`: Main `Handler` that implements the `devwatch` interfaces.

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
