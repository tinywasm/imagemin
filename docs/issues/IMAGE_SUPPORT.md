# Image Processing Support for AssetMin

## Overview

Add image processing capabilities to AssetMin library to automatically convert and optimize images (JPG, JPEG, PNG) to WebP format with responsive sizing for different devices (desktop, tablet, mobile).

## Current State

AssetMin currently processes:
- JavaScript files (`.js`) → `script.js`
- CSS files (`.css`) → `style.css`  
- SVG files (`.svg`) → `icons.svg` (sprite) and `favicon.svg`
- HTML files (`.html`) → `index.html`

Images are not processed and must be manually optimized.

## Objectives

1. **Auto-convert images to WebP format** using `github.com/HugoSmits86/nativewebp`
2. **Generate responsive variants** for different screen sizes (Large, Medium, Small)
3. **Process images from ThemeFolder** and output to WebFilesFolder
4. **Event-driven processing only** - no scanning of existing files on startup
5. **Strict Suffix Requirement** - Images must be named with suffixes (e.g., `image.L.M.jpg`) to be processed
6. **Maintain consistency** with existing AssetMin architecture

## Proposed Architecture

### 1. Image Extensions Support

Add image extensions to `SupportedExtensions()`:

```go
func (c *AssetMin) SupportedExtensions() []string {
    return []string{".js", ".css", ".svg", ".html", ".jpg", ".jpeg", ".png", ".webp"}
}
```

### 2. Image Processing Flow

```
Input: ThemeFolder/images/*.{L,M,S}.{jpg,jpeg,png}
   ↓
Process: Parse suffixes -> Generate ONLY requested variants (L, M, or S) -> Convert to WebP
   ↓
Output: WebFilesFolder/images/*.{L,M,S}.webp
```

**Important:** Images without L, M, or S suffixes are IGNORED.

### 3. Configuration Structure

Add to `AssetConfig`:

```go
type AssetConfig struct {
    ThemeFolder             func() string
    WebFilesFolder          func() string
    Logger                  func(message ...any)
    GetRuntimeInitializerJS func() (string, error)
    AppName                 string
    
    // New: Image processing configuration
    ImageConfig *ImageConfig `optional`
}

type ImageConfig struct {
    // Input/Output folders (relative to ThemeFolder/WebFilesFolder)
    InputFolder  string // default: "images"
    OutputFolder string // default: "images"
    
    // Responsive sizes configuration
    Variants []ImageVariant
    
    // WebP quality settings
    Quality int // default: 80 (0-100)
    
    // Enable/disable image processing
    Enabled bool // default: true
}

type ImageVariant struct {
    Name      string // "L", "M", "S"
    MaxWidth  int    // maximum width in pixels
    MaxHeight int    // maximum height in pixels (0 = maintain aspect ratio)
    Suffix    string // "L", "M", "S"
}
```

### 4. Default Configuration

If `ImageConfig` is nil, use these defaults:

```go
defaultImageConfig := &ImageConfig{
    InputFolder:  "images",
    OutputFolder: "images",
    Quality:      80,
    Enabled:      true,
    Variants: []ImageVariant{
        {Name: "Large",  MaxWidth: 1920, MaxHeight: 0, Suffix: "L"},
        {Name: "Medium", MaxWidth: 1024, MaxHeight: 0, Suffix: "M"},
        {Name: "Small",  MaxWidth: 640,  MaxHeight: 0, Suffix: "S"},
    },
}
```

### 5. File Naming Convention

**Input:**
```
ThemeFolder/images/
  ├── photo.L.M.jpg    (Processes L and M variants)
  ├── logo.S.png       (Processes S variant only)
  ├── banner.L.M.S.jpg (Processes all 3 variants)
  └── ignored.jpg      (IGNORED - no suffix)
```

**Output:**
```
WebFilesFolder/images/
  ├── photo.L.webp    (Large: max 1920px)
  ├── photo.M.webp    (Medium: max 1024px)
  ├── logo.S.webp     (Small: max 640px)
  ├── banner.L.webp
  ├── banner.M.webp
  ├── banner.S.webp
```

### 6. Handler Structure

Create new `image.go` file:

```go
type imageHandler struct {
    *asset
    config *ImageConfig
}

func NewImageHandler(ac *AssetConfig) *imageHandler {
    imgConfig := ac.ImageConfig
    if imgConfig == nil {
        imgConfig = getDefaultImageConfig()
    }
    
    return &imageHandler{
        asset:  newAssetFile("", "image/webp", ac, nil),
        config: imgConfig,
    }
}

func (h *imageHandler) ProcessImage(inputPath string) error {
    // 1. Check filename for suffixes (L, M, S)
    // 2. If no valid suffixes, return nil (ignore)
    // 3. Read source image
    // 4. For each PRESENT suffix in filename:
    //    - Resize to corresponding variant size
    //    - Convert to WebP
    //    - Save to output folder with suffix (e.g. name.L.webp)
    // 5. Return error if any step fails
}
```

### 7. Integration with NewFileEvent

Modify `UpdateFileContentInMemory()` in `events.go`:

```go
func (c *AssetMin) UpdateFileContentInMemory(filePath, extension, event string, content []byte) (*asset, error) {
    file := &contentFile{
        path:    filePath,
        content: content,
    }

    switch extension {
    case ".css":
        // existing code...
    case ".js":
        // existing code...
    case ".svg":
        // existing code...
    case ".html":
        // existing code...
    
    // New: Image processing
    case ".jpg", ".jpeg", ".png", ".webp":
        err := c.imageHandler.ProcessImage(filePath)
        return nil, err // Images don't use asset buffering
    }

    return nil, errors.New("UpdateFileContentInMemory extension: " + extension + " not found " + filePath)
}
```

### 8. Updated index_basic.html Template

Add responsive images to the default template:

```html
<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.AppName}}</title>
    <link rel="icon" type="image/svg+xml" href="favicon.svg" />
    <link rel="stylesheet" href="style.css" type="text/css" />
</head>
<body>
    <h1>Welcome to {{.AppName}}</h1>
    <p>This is a basic HTML template generated by AssetMin.</p>
    
    {{if .HasImages}}
    <section class="gallery">
        <h2>Image Gallery</h2>
        {{range .Images}}
        <picture>
            <source media="(min-width: 1024px)" srcset="images/{{.Name}}-lg.webp">
            <source media="(min-width: 640px)" srcset="images/{{.Name}}-md.webp">
            <img src="images/{{.Name}}-sm.webp" alt="{{.Alt}}" loading="lazy">
        </picture>
        {{end}}
    </section>
    {{end}}
    
    <script src="script.js" type="text/javascript"></script>
</body>
</html>
```

### 9. Template Data Enhancement

Update `templateData` in `htmlGenerator.go`:

```go
type templateData struct {
    AppName   string
    HasImages bool
    Images    []ImageData
}

type ImageData struct {
    Name string // base name without extension and suffixes
    Alt  string // alt text (derived from filename)
    // Availability flags
    HasL bool
    HasM bool
    HasS bool
}
```

## Recommended Image Sizes

Based on modern web standards and common device viewports:

### Large (L)
- **Max Width:** 1920px
- **Reasoning:** Covers full HD displays (1920x1080)
- **Suffix:** `L`

### Medium (M)
- **Max Width:** 1024px
- **Reasoning:** Standard tablet landscape (iPad: 1024x768)
- **Suffix:** `M`

### Small (S)
- **Max Width:** 640px
- **Reasoning:** Covers most mobile devices (2x density on 320px physical width)
- **Suffix:** `S`

### Additional Considerations
- **Event-Driven Only:** Images are only processed when a file event occurs. Existing files are not scanned on startup.
- Always maintain aspect ratio (MaxHeight: 0)
- Use WebP quality: 80 (good balance between quality and file size)
- Support lazy loading with `loading="lazy"` attribute
- Use `<picture>` element for art direction and better browser support

## Implementation Steps

### Phase 1: Core Infrastructure
1. ✅ Create `docs/issues/IMAGE_SUPPORT.md` (this document)
2. Add `ImageConfig` to `AssetConfig` struct
3. Implement default image configuration
4. Add image extensions to `SupportedExtensions()`

### Phase 2: Image Processing
5. Create `image.go` with `imageHandler` struct
6. Implement WebP conversion using `github.com/HugoSmits86/nativewebp`
7. Implement responsive variant generation
8. Add proper error handling and logging

### Phase 3: Integration
9. Update `NewAssetMin()` to initialize image handler
10. Modify `UpdateFileContentInMemory()` to handle image extensions
11. Update `NewFileEvent()` to process images (if needed)

### Phase 4: Template Enhancement
12. Update `index_basic.html` template with responsive images
13. Update `templateData` structure
14. Implement image discovery in template generation
15. Add default CSS for responsive images in `style_basic.css`

### Phase 5: Testing
16. Create test images in `templates/images/`
17. Write unit tests for image processing
18. Write integration tests with file events
19. Test with golite integration

### Phase 6: Documentation
20. Update README.md with image processing features
21. Add examples and best practices
22. Document configuration options

## Dependencies

### Primary Library
- `github.com/HugoSmits86/nativewebp` - WebP encoding/decoding

### Image Manipulation (if needed for resizing)
Consider adding one of these if `nativewebp` doesn't handle resizing:
- `github.com/disintegration/imaging` - Simple and powerful
- `github.com/nfnt/resize` - Lightweight
- `golang.org/x/image/draw` - Standard library (limited features)

**Recommendation:** Start with `disintegration/imaging` for resizing, then use `nativewebp` for final WebP conversion.

## Design Decisions

### ✅ Pros
1. **Automatic optimization** - Developers don't need to manually convert images
2. **Responsive by default** - Generated variants work out of the box
3. **Modern format** - WebP provides better compression than JPG/PNG
4. **Consistent architecture** - Follows existing AssetMin patterns
5. **Configurable** - Users can customize sizes and quality
6. **Backward compatible** - Optional feature, doesn't break existing code

### ⚠️ Considerations
1. **Build time increase** - Image processing adds overhead
2. **Disk space** - 3 variants per image increases storage
3. **Complexity** - More moving parts to maintain
4. **Browser support** - WebP is well-supported but fallbacks may be needed

### 🔄 Alternative Approaches
See separate documents:
- `IMAGE_SUPPORT_ALTERNATIVES.md` - Alternative architectures
- `IMAGE_SUPPORT_TRADEOFFS.md` - Detailed pros/cons analysis

## Open Questions

1. **Should we keep original format?**
   - Option A: Only generate WebP variants (recommended)
   - Option B: Also keep one original format as fallback
   - **Decision:** Option A (modern browsers support WebP)

2. **How to handle already-converted WebP images?**
   - Option A: Skip processing if input is already WebP
   - Option B: Still generate variants from WebP source
   - **Decision:** Option B (maintain consistency)

3. **Should we process SVG images differently?**
   - Current: SVG goes to sprite handler
   - Proposal: Keep SVG separate from raster images
   - **Decision:** Keep existing SVG behavior, don't mix with WebP

4. **Caching strategy?**
   - Option A: Always regenerate on file event
   - Option B: Check output timestamps, skip if unchanged
   - Option C: Content-based hashing to detect changes
   - **Recommendation:** Start with Option A, add Option C later for optimization

5. **Should we support animated images (GIF, animated WebP)?**
   - **Decision:** Phase 2 feature, not in initial implementation

## Testing Strategy

### Unit Tests
- Test WebP conversion for JPG, JPEG, PNG
- Test variant generation with different sizes
- Test naming conventions
- Test error handling (corrupt images, unsupported formats)

### Integration Tests
- Test with file watcher events (create, write, delete)
- Test with golite integration
- Test template generation with images
- Test default configuration vs custom configuration

### Test Images
Use existing test images in `templates/images/`:
- `gopher.png` - PNG format
- `gatos.jpeg` - JPEG format
- `perros.jpg` - JPG format

## Success Criteria

1. ✅ Images are automatically detected and processed
2. ✅ Three responsive variants generated per image
3. ✅ Output WebP files are smaller than originals
4. ✅ Template includes images with responsive srcset
5. ✅ Integration with golite works seamlessly
6. ✅ Configuration is intuitive and well-documented
7. ✅ Error messages are clear and actionable

## Future Enhancements

- Add AVIF format support (next-gen after WebP)
- Support for animated images (GIF → WebP/AVIF)
- Smart cropping for different aspect ratios
- Image optimization hints (lazy loading, blur placeholder)
- CDN integration for image delivery
- Automatic alt text generation using AI/ML

## References

- WebP documentation: https://developers.google.com/speed/webp
- Responsive images: https://developer.mozilla.org/en-US/docs/Learn/HTML/Multimedia_and_embedding/Responsive_images
- Picture element: https://developer.mozilla.org/en-US/docs/Web/HTML/Element/picture
- nativewebp library: https://github.com/HugoSmits86/nativewebp
