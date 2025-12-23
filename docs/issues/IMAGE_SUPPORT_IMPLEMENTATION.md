# Image Processing - Technical Implementation Guide

## Phase 1: Core Infrastructure Setup

### Step 1.1: Update AssetConfig
**File:** `assetmin.go`

```go
type AssetConfig struct {
    ThemeFolder             func() string
    WebFilesFolder          func() string
    Logger                  func(message ...any)
    GetRuntimeInitializerJS func() (string, error)
    AppName                 string
    
    // New: Image processing configuration
    ImageConfig *ImageConfig
}

type ImageConfig struct {
    // Input/Output folders (relative paths)
    InputFolder  string // default: "images"
    OutputFolder string // default: "images"
    
    // Responsive size variants
    Variants []ImageVariant
    
    // WebP quality (0-100)
    Quality int // default: 80
    
    // Enable/disable processing
    Enabled bool // default: true
    
    // Process existing images at startup
    ProcessExistingOnStartup bool // default: true
}

type ImageVariant struct {
    Name      string // e.g., "desktop", "tablet", "mobile"
    MaxWidth  int    // maximum width in pixels
    MaxHeight int    // maximum height in pixels (0 = maintain aspect ratio)
    Suffix    string // e.g., "-lg", "-md", "-sm"
}

// getDefaultImageConfig returns sensible defaults
func getDefaultImageConfig() *ImageConfig {
    return &ImageConfig{
        InputFolder:              "images",
        OutputFolder:             "images",
        Quality:                  80,
        Enabled:                  true,
        ProcessExistingOnStartup: true,
        Variants: []ImageVariant{
            {Name: "desktop", MaxWidth: 1920, MaxHeight: 0, Suffix: "-lg"},
            {Name: "tablet", MaxWidth: 1024, MaxHeight: 0, Suffix: "-md"},
            {Name: "mobile", MaxWidth: 640, MaxHeight: 0, Suffix: "-sm"},
        },
    }
}
```

### Step 1.2: Update SupportedExtensions
**File:** `assetmin.go`

```go
func (c *AssetMin) SupportedExtensions() []string {
    return []string{".js", ".css", ".svg", ".html", ".jpg", ".jpeg", ".png", ".webp"}
}
```

### Step 1.3: Add Image Handler to AssetMin
**File:** `assetmin.go`

```go
type AssetMin struct {
    mu sync.Mutex
    *AssetConfig
    mainStyleCssHandler *asset
    mainJsHandler       *asset
    spriteSvgHandler    *asset
    faviconSvgHandler   *asset
    indexHtmlHandler    *asset
    imageHandler        *imageHandler  // NEW
    min                 *minify.M

    WriteOnDisk bool
    
    jsMainFileName     string
    cssMainFileName    string
    svgMainFileName    string
    svgFaviconFileName string
    htmlMainFileName   string
}
```

---

## Phase 2: Image Handler Implementation

### Step 2.1: Create image.go
**File:** `image.go` (new file)

```go
package assetmin

import (
    "bytes"
    "errors"
    "fmt"
    "image"
    _ "image/jpeg" // Register JPEG decoder
    _ "image/png"  // Register PNG decoder
    "os"
    "path/filepath"
    "strings"

    "github.com/HugoSmits86/nativewebp"
    "github.com/disintegration/imaging"
)

type imageHandler struct {
    config       *ImageConfig
    themeFolder  string
    outputFolder string
    logger       func(message ...any)
}

// NewImageHandler creates a new image processor
func NewImageHandler(ac *AssetConfig) *imageHandler {
    imgConfig := ac.ImageConfig
    if imgConfig == nil {
        imgConfig = getDefaultImageConfig()
    }

    return &imageHandler{
        config:       imgConfig,
        themeFolder:  filepath.Join(ac.ThemeFolder(), imgConfig.InputFolder),
        outputFolder: filepath.Join(ac.WebFilesFolder(), imgConfig.OutputFolder),
        logger:       ac.Logger,
    }
}

// ProcessImage converts and generates responsive variants for an image
func (h *imageHandler) ProcessImage(inputPath string) error {
    if !h.config.Enabled {
        return nil
    }

    // Validate input file exists
    if _, err := os.Stat(inputPath); err != nil {
        return fmt.Errorf("image file not found: %s", inputPath)
    }

    // Read and decode source image
    srcImage, format, err := h.loadImage(inputPath)
    if err != nil {
        return fmt.Errorf("failed to load image: %w", err)
    }

    if h.logger != nil {
        h.logger("Processing image:", filepath.Base(inputPath), "format:", format)
    }

    // Extract base name (without extension)
    baseName := h.getBaseName(inputPath)

    // Ensure output directory exists
    if err := os.MkdirAll(h.outputFolder, 0755); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }

    // Generate each variant
    for _, variant := range h.config.Variants {
        if err := h.generateVariant(srcImage, baseName, variant); err != nil {
            return fmt.Errorf("failed to generate %s variant: %w", variant.Name, err)
        }
    }

    return nil
}

// loadImage reads and decodes an image file
func (h *imageHandler) loadImage(path string) (image.Image, string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, "", err
    }
    defer file.Close()

    img, format, err := image.Decode(file)
    if err != nil {
        return nil, "", err
    }

    return img, format, nil
}

// getBaseName extracts the base filename without extension
func (h *imageHandler) getBaseName(path string) string {
    base := filepath.Base(path)
    ext := filepath.Ext(base)
    return strings.TrimSuffix(base, ext)
}

// generateVariant creates a single responsive variant
func (h *imageHandler) generateVariant(srcImage image.Image, baseName string, variant ImageVariant) error {
    // Resize image if needed
    resized := h.resizeImage(srcImage, variant)

    // Convert to WebP
    outputPath := filepath.Join(h.outputFolder, baseName+variant.Suffix+".webp")
    if err := h.saveAsWebP(resized, outputPath); err != nil {
        return err
    }

    if h.logger != nil {
        bounds := resized.Bounds()
        h.logger(fmt.Sprintf("Generated %s variant: %dx%d -> %s",
            variant.Name, bounds.Dx(), bounds.Dy(), filepath.Base(outputPath)))
    }

    return nil
}

// resizeImage resizes an image according to variant specifications
func (h *imageHandler) resizeImage(img image.Image, variant ImageVariant) image.Image {
    bounds := img.Bounds()
    width := bounds.Dx()
    height := bounds.Dy()

    // Check if resizing is needed
    if width <= variant.MaxWidth && (variant.MaxHeight == 0 || height <= variant.MaxHeight) {
        return img // No resize needed
    }

    // Calculate new dimensions maintaining aspect ratio
    newWidth := variant.MaxWidth
    newHeight := variant.MaxHeight

    if variant.MaxHeight == 0 {
        // Maintain aspect ratio based on width
        aspectRatio := float64(height) / float64(width)
        newHeight = int(float64(newWidth) * aspectRatio)
    }

    // Use Lanczos resampling for high quality
    return imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
}

// saveAsWebP encodes and saves an image as WebP format
func (h *imageHandler) saveAsWebP(img image.Image, outputPath string) error {
    // Create output file
    outFile, err := os.Create(outputPath)
    if err != nil {
        return fmt.Errorf("failed to create output file: %w", err)
    }
    defer outFile.Close()

    // Encode as WebP
    options := &nativewebp.EncodeOptions{
        Quality: float32(h.config.Quality),
        Lossless: false,
    }

    if err := nativewebp.Encode(outFile, img, options); err != nil {
        return fmt.Errorf("failed to encode WebP: %w", err)
    }

    return nil
}

// DiscoverProcessedImages scans the output directory for processed images
// Returns a list of base names (without suffix and extension)
func (h *imageHandler) DiscoverProcessedImages() []string {
    imageSet := make(map[string]bool)

    entries, err := os.ReadDir(h.outputFolder)
    if err != nil {
        return nil
    }

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }

        name := entry.Name()
        if !strings.HasSuffix(name, ".webp") {
            continue
        }

        // Extract base name by removing suffix and extension
        baseName := h.extractBaseName(name)
        if baseName != "" {
            imageSet[baseName] = true
        }
    }

    // Convert map to slice
    result := make([]string, 0, len(imageSet))
    for name := range imageSet {
        result = append(result, name)
    }

    return result
}

// extractBaseName removes variant suffix and extension
func (h *imageHandler) extractBaseName(filename string) string {
    // Remove .webp extension
    name := strings.TrimSuffix(filename, ".webp")

    // Remove variant suffix
    for _, variant := range h.config.Variants {
        if strings.HasSuffix(name, variant.Suffix) {
            return strings.TrimSuffix(name, variant.Suffix)
        }
    }

    return name
}

// ProcessExistingImages scans input folder and processes all images
func (h *imageHandler) ProcessExistingImages() error {
    if !h.config.Enabled || !h.config.ProcessExistingOnStartup {
        return nil
    }

    // Check if input folder exists
    if _, err := os.Stat(h.themeFolder); os.IsNotExist(err) {
        return nil // Input folder doesn't exist, nothing to process
    }

    entries, err := os.ReadDir(h.themeFolder)
    if err != nil {
        return fmt.Errorf("failed to read input directory: %w", err)
    }

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }

        ext := strings.ToLower(filepath.Ext(entry.Name()))
        if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
            inputPath := filepath.Join(h.themeFolder, entry.Name())
            if err := h.ProcessImage(inputPath); err != nil {
                if h.logger != nil {
                    h.logger("Warning: Failed to process", entry.Name(), ":", err)
                }
                // Continue with other images
            }
        }
    }

    return nil
}
```

---

## Phase 3: Integration with NewFileEvent

### Step 3.1: Update events.go

```go
func (c *AssetMin) UpdateFileContentInMemory(filePath, extension, event string, content []byte) (*asset, error) {
    file := &contentFile{
        path:    filePath,
        content: content,
    }

    switch extension {
    case ".css":
        err := c.mainStyleCssHandler.UpdateContent(filePath, event, file)
        return c.mainStyleCssHandler, err

    case ".js":
        file.content = stripLeadingUseStrict(file.content)
        err := c.mainJsHandler.UpdateContent(filePath, event, file)
        return c.mainJsHandler, err

    case ".svg":
        if filepath.Base(filePath) == c.svgFaviconFileName {
            err := c.faviconSvgHandler.UpdateContent(filePath, event, file)
            return c.faviconSvgHandler, err
        }
        err := c.spriteSvgHandler.UpdateContent(filePath, event, file)
        return c.spriteSvgHandler, err

    case ".html":
        err := c.indexHtmlHandler.UpdateContent(filePath, event, file)
        return c.indexHtmlHandler, err

    // NEW: Image processing
    case ".jpg", ".jpeg", ".png":
        // Images don't use the asset buffer system
        // Process directly and write to disk
        if event == "remove" || event == "delete" {
            return nil, c.imageHandler.RemoveImage(filePath)
        }
        return nil, c.imageHandler.ProcessImage(filePath)
    }

    return nil, errors.New("UpdateFileContentInMemory extension: " + extension + " not found " + filePath)
}
```

### Step 3.2: Initialize image handler in NewAssetMin
**File:** `assetmin.go`

```go
func NewAssetMin(ac *AssetConfig) *AssetMin {
    c := &AssetMin{
        AssetConfig: ac,
        min:         minify.New(),
        WriteOnDisk: true,
        jsMainFileName:     "script.js",
        cssMainFileName:    "style.css",
        svgMainFileName:    "icons.svg",
        svgFaviconFileName: "favicon.svg",
        htmlMainFileName:   "index.html",
    }

    if c.AppName == "" {
        c.AppName = "MyApp"
    }

    // Initialize handlers
    c.mainStyleCssHandler = newAssetFile(c.cssMainFileName, "text/css", ac, nil)
    c.mainJsHandler = newAssetFile(c.jsMainFileName, "text/javascript", ac, ac.GetRuntimeInitializerJS)
    c.spriteSvgHandler = NewSvgHandler(ac, c.svgMainFileName)
    c.faviconSvgHandler = NewFaviconSvgHandler(ac, c.svgFaviconFileName)
    c.indexHtmlHandler = NewHtmlHandler(ac, c.htmlMainFileName, c.cssMainFileName, c.jsMainFileName)
    c.imageHandler = NewImageHandler(ac)  // NEW
    
    // ... rest of initialization ...

    // Process existing images at startup
    if err := c.imageHandler.ProcessExistingImages(); err != nil {
        c.writeMessage("Warning: Error processing existing images:", err)
    }

    return c
}
```

---

## Phase 4: Template Enhancement

### Step 4.1: Update index_basic.html
**File:** `templates/index_basic.html`

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
    <section class="image-gallery">
        <h2>Images</h2>
        {{range .Images}}
        <figure>
            <picture>
                <source media="(min-width: 1024px)" srcset="images/{{.Name}}-lg.webp">
                <source media="(min-width: 640px)" srcset="images/{{.Name}}-md.webp">
                <img src="images/{{.Name}}-sm.webp" alt="{{.Alt}}" loading="lazy">
            </picture>
            <figcaption>{{.Alt}}</figcaption>
        </figure>
        {{end}}
    </section>
    {{end}}
    
    <script src="script.js" type="text/javascript"></script>
</body>
</html>
```

### Step 4.2: Update style_basic.css
**File:** `templates/style_basic.css`

```css
/* AssetMin CSS Template */

body {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    line-height: 1.6;
}

h1 {
    color: #333;
}

p {
    color: #666;
}

/* Image Gallery Styles */
.image-gallery {
    max-width: 1200px;
    margin: 2rem auto;
    padding: 0 1rem;
}

.image-gallery figure {
    margin: 2rem 0;
}

.image-gallery picture {
    display: block;
}

.image-gallery img {
    max-width: 100%;
    height: auto;
    display: block;
    border-radius: 8px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.image-gallery figcaption {
    margin-top: 0.5rem;
    font-size: 0.9rem;
    color: #666;
    text-align: center;
}
```

### Step 4.3: Update htmlGenerator.go

```go
type templateData struct {
    AppName   string
    HasImages bool
    Images    []ImageData
}

type ImageData struct {
    Name string // base name without extension/suffix
    Alt  string // alt text derived from filename
}

func (a *AssetMin) CreateDefaultIndexHtmlIfNotExist() *AssetMin {
    targetPath := filepath.Join(a.ThemeFolder(), a.htmlMainFileName)

    if _, err := os.Stat(targetPath); err == nil {
        if a.Logger != nil {
            a.Logger("HTML file already exists at", targetPath, ", skipping generation")
        }
        return a
    }

    raw, errRead := embeddedFS.ReadFile("templates/index_basic.html")
    if errRead != nil {
        if a.Logger != nil {
            a.Logger("Error reading embedded template:", errRead)
        }
        return a
    }

    // Discover processed images
    images := a.discoverImages()

    data := templateData{
        AppName:   a.AppName,
        HasImages: len(images) > 0,
        Images:    images,
    }

    // Parse template
    content, err := a.parseTemplate(string(raw), data)
    if err != nil {
        if a.Logger != nil {
            a.Logger("Error parsing template:", err)
        }
        return a
    }

    if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
        if a.Logger != nil {
            a.Logger("Error creating directory:", err)
        }
        return a
    }

    if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
        if a.Logger != nil {
            a.Logger("Error writing HTML file:", err)
        }
        return a
    }

    if a.Logger != nil {
        a.Logger("Generated HTML file at", targetPath)
    }

    return a
}

// discoverImages finds all processed images
func (a *AssetMin) discoverImages() []ImageData {
    if a.imageHandler == nil {
        return nil
    }

    baseNames := a.imageHandler.DiscoverProcessedImages()
    images := make([]ImageData, len(baseNames))

    for i, name := range baseNames {
        images[i] = ImageData{
            Name: name,
            Alt:  generateAltText(name),
        }
    }

    return images
}

// generateAltText creates alt text from filename
func generateAltText(filename string) string {
    // Replace hyphens/underscores with spaces
    alt := strings.ReplaceAll(filename, "-", " ")
    alt = strings.ReplaceAll(alt, "_", " ")
    
    // Capitalize first letter
    if len(alt) > 0 {
        alt = strings.ToUpper(alt[:1]) + alt[1:]
    }
    
    return alt
}

// parseTemplate processes the template with data
func (a *AssetMin) parseTemplate(templateStr string, data templateData) (string, error) {
    // Simple template parsing (replace placeholders)
    result := templateStr
    
    // Replace {{.AppName}}
    result = strings.ReplaceAll(result, "{{.AppName}}", data.AppName)
    
    // Handle {{if .HasImages}}...{{end}}
    if data.HasImages {
        result = strings.ReplaceAll(result, "{{if .HasImages}}", "")
        result = strings.ReplaceAll(result, "{{end}}", "")
        
        // Build image gallery HTML
        galleryHTML := ""
        for _, img := range data.Images {
            galleryHTML += fmt.Sprintf(`
        <figure>
            <picture>
                <source media="(min-width: 1024px)" srcset="images/%s-lg.webp">
                <source media="(min-width: 640px)" srcset="images/%s-md.webp">
                <img src="images/%s-sm.webp" alt="%s" loading="lazy">
            </picture>
            <figcaption>%s</figcaption>
        </figure>
`, img.Name, img.Name, img.Name, img.Alt, img.Alt)
        }
        result = strings.ReplaceAll(result, "{{range .Images}}", galleryHTML)
    } else {
        // Remove the entire if block
        ifStart := strings.Index(result, "{{if .HasImages}}")
        if ifStart >= 0 {
            ifEnd := strings.Index(result, "{{end}}")
            if ifEnd >= 0 {
                result = result[:ifStart] + result[ifEnd+7:]
            }
        }
    }
    
    return result, nil
}
```

---

## Phase 5: Testing

### Step 5.1: Create image_test.go

```go
package assetmin

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestImageProcessing(t *testing.T) {
    env := setupTestEnv("image_processing", t)
    defer env.CleanDirectory()

    // Copy test images to theme folder
    imagesDir := filepath.Join(env.ThemeDir, "images")
    require.NoError(t, os.MkdirAll(imagesDir, 0755))

    // Test with PNG image
    testImage := filepath.Join(imagesDir, "test.png")
    copyTestImage(t, "templates/images/gopher.png", testImage)

    // Trigger processing
    err := env.AssetsHandler.NewFileEvent("test.png", ".png", testImage, "write")
    require.NoError(t, err)

    // Verify variants were created
    outputDir := filepath.Join(env.PublicDir, "images")
    
    lgPath := filepath.Join(outputDir, "test-lg.webp")
    mdPath := filepath.Join(outputDir, "test-md.webp")
    smPath := filepath.Join(outputDir, "test-sm.webp")

    assert.FileExists(t, lgPath, "Large variant should exist")
    assert.FileExists(t, mdPath, "Medium variant should exist")
    assert.FileExists(t, smPath, "Small variant should exist")

    // Verify file sizes are reasonable
    lgInfo, _ := os.Stat(lgPath)
    mdInfo, _ := os.Stat(mdPath)
    smInfo, _ := os.Stat(smPath)

    assert.True(t, smInfo.Size() < mdInfo.Size(), "Small should be smaller than medium")
    assert.True(t, mdInfo.Size() < lgInfo.Size(), "Medium should be smaller than large")
}

func copyTestImage(t *testing.T, src, dst string) {
    data, err := os.ReadFile(src)
    require.NoError(t, err)
    require.NoError(t, os.WriteFile(dst, data, 0644))
}
```

---

## Dependencies to Add

### go.mod updates needed:

```bash
go get github.com/HugoSmits86/nativewebp@latest
go get github.com/disintegration/imaging@latest
```

---

## Success Checklist

- [ ] ImageConfig added to AssetConfig
- [ ] Default configuration implemented
- [ ] Image extensions added to SupportedExtensions
- [ ] image.go created with full handler
- [ ] Integration with NewFileEvent working
- [ ] Image handler initialized in NewAssetMin
- [ ] Existing images processed at startup
- [ ] index_basic.html updated with responsive images
- [ ] style_basic.css updated with gallery styles
- [ ] Template parsing enhanced with image discovery
- [ ] Unit tests passing
- [ ] Integration tests passing
- [ ] Documentation updated

## Next Steps After Implementation

1. Test with real golite integration
2. Measure performance impact
3. Add configuration examples to README
4. Consider additional optimizations (caching, parallel processing)
5. Gather user feedback
6. Plan Phase 2 features (AVIF, animated images, etc.)
