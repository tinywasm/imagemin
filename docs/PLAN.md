# PLAN: imagemin — WebP Image Processing para Módulos SSR

## Objetivo

`imagemin` descubre imágenes declaradas en archivos `ssr.go` de módulos Go, las convierte
a WebP con variantes responsivas (L, M, S) y las escribe en `web/public/img/`. Procesa cada
imagen **una sola vez** usando hash del archivo fuente para evitar reprocesamiento innecesario.
Opera como **handler independiente** de `assetmin` — `tinywasm/app` los orquesta al mismo nivel.

## Dónde vive `Asset`

`Asset` y el tipo `Variant` se definen en **`tinywasm/imagemin`** en un archivo
**sin build tag** (ej: `types.go`). Son el API pública del paquete — deben compilar en
cualquier contexto. Los módulos los importan en `ssr.go` (build tag `!wasm`), lo que
evita que entren al WASM binary sin necesidad de restringir el tipo en sí.

```go
// types.go — SIN build tag, compila en todos los contextos

type Variant uint8

const (
    VariantS Variant = 1 << iota // 1 — 640px  mobile
    VariantM                      // 2 — 1024px tablet
    VariantL                      // 4 — 1920px desktop
)

// AllVariants en bloque separado para no interferir con iota.
// Si se añade una variante nueva, AllVariants debe actualizarse explícitamente.
const AllVariants = VariantS | VariantM | VariantL

type Asset struct {
    Path     string  // relativo al directorio del módulo: "img/logo.png"
    Variants Variant // ej: AllVariants, VariantS|VariantM, VariantL
    Alt      string  // texto alternativo SEO; si vacío se deriva del nombre de archivo
}
```

## Contrato en `ssr.go` de módulos externos

```go
//go:build !wasm

package clinical_encounter

import "github.com/tinywasm/imagemin"

func RenderImages() []imagemin.Asset {
    return []imagemin.Asset{
        {Path: "img/logo.png", Variants: imagemin.VariantS | imagemin.VariantM, Alt: "Logo clínica"},
        {Path: "img/hero.jpg", Variants: imagemin.AllVariants,                  Alt: "Portada encuentro clínico"},
    }
}
```

**Ventajas del tipo `Variant` sobre `[]string`:**
- Typesafe — el compilador rechaza valores inválidos
- Sin typos (`"l"` vs `"L"`)
- Combinable con `|`: `VariantS | VariantM`
- Predefinido `AllVariants` para el caso más común
- Compacto — un solo `uint8`

## Variantes responsivas

| Variante | MaxWidth | Sufijo de salida | Uso típico |
|---|---|---|---|
| `L` | 1920px | `name.L.webp` | Desktop full HD |
| `M` | 1024px | `name.M.webp` | Tablet landscape |
| `S` | 640px  | `name.S.webp` | Mobile (2x density en 320px físicos) |

Regla: si la imagen fuente es más pequeña que el MaxWidth de una variante, se copia sin
agrandar (evita pérdida de calidad). Solo se procesan las variantes declaradas en `Variants`.

## Salida

```
web/public/img/
  ├── logo.S.webp          (de clinical_encounter/img/logo.png, variante S)
  ├── logo.M.webp          (variante M)
  ├── hero.L.webp          (de clinical_encounter/img/hero.jpg, variante L)
  ├── hero.M.webp
  └── hero.S.webp
```

Estructura plana — sin subdirectorios por módulo. Si dos módulos tienen una imagen con el
mismo nombre base, el último en procesarse gana (comportamiento determinista: orden alfabético
por module path). Considerar prefijo de módulo como mitigación futura.

## Caching — "procesar una sola vez"

Para no reprocesar imágenes que no cambiaron, imagemin mantiene un archivo de caché
`web/public/img/.cache.json` con el hash SHA256 de cada archivo fuente:

```json
{
  "github.com/cdvelop/clinical_encounter/img/logo.png": {
    "srcHash": "abc123...",
    "variants": 3,
    "outputFiles": ["logo.S.webp", "logo.M.webp"]
  }
}
```

`variants` se almacena como el valor numérico del bitmask `Variant` (`VariantS|VariantM = 3`),
no como array de strings — consistente con el tipo Go y sin conversión en lectura/escritura.

```json
```

**Lógica al procesar una imagen:**
1. Calcular SHA256 del archivo fuente
2. Si hash coincide con caché Y los archivos de salida existen → saltar
3. Si difiere o faltan archivos de salida → procesar + actualizar caché

## Arquitectura del paquete

```
tinywasm/imagemin/
├── types.go         — Variant, Asset, constantes — SIN build tag
├── imagemin.go      — Handler struct, Config, New(), interfaces devwatch
├── extract.go       — AST parser de RenderImages() en ssr.go  (!wasm)
├── loader.go        — descubrimiento via go list + orquestación (!wasm)
├── convert.go       — resize (imaging) + WebP encode (nativewebp)   (!wasm)
├── cache.go         — SHA256 hash cache en .cache.json               (!wasm)
└── docs/
    └── PLAN.md
```

### `imagemin.go` — Handler e interfaces

```go
package imagemin

type Handler struct {
    config        *Config
    log           func(messages ...any)
    listModulesFn func(rootDir string) ([]string, error) // inyectable en tests
}

type Config struct {
    RootDir   string // directorio raíz del proyecto (donde está go.mod)
    OutputDir string // ej: "web/public/img" — siempre en disco
    Quality   int    // WebP quality 0-100, default: 82 (buen balance SEO/tamaño)
}

func New(c *Config) *Handler

// Implementa devwatch.FilesEventHandlers — registrado para logging y exclusión de archivos.
// No escucha extensiones directamente: el hot reload llega via GoModHandler.OnSSRFileChange.
func (h *Handler) Name() string
func (h *Handler) SupportedExtensions() []string        // retorna [] — sin escucha directa
func (h *Handler) NewFileEvent(fileName, extension, filePath, event string) error // no-op, requerido por interfaz
func (h *Handler) UnobservedFiles() []string             // excluye OutputDir + .cache.json
func (h *Handler) MainInputFileRelativePath() string     // retorna ""

// API pública
func (h *Handler) LoadImages() error               // descubrir + procesar al arrancar
func (h *Handler) ReloadModule(moduleDir string) error // hot reload de un módulo
func (h *Handler) SetLog(fn func(messages ...any))
func (h *Handler) SetListModulesFn(fn func(rootDir string) ([]string, error))
func (h *Handler) WaitForLoad(timeout time.Duration) // solo para tests
```

### `extract.go` — AST parser

```go
//go:build !wasm

// ExtractImages parsea ssr.go en moduleDir y retorna las Asset declaradas.
// Busca func RenderImages() []imagemin.Asset y extrae el slice literal.
// Resuelve las constantes Variant (VariantS, VariantM, VariantL, AllVariants) a su valor uint8.
// Soporta: slice literals inline y variables locales referenciadas en el return.
func ExtractImages(moduleDir string) ([]ParsedAsset, error)

type ParsedAsset struct {
    AbsPath  string  // path absoluto: moduleDir + "/" + asset.Path
    Variants Variant // valor resuelto del bitmask
    Alt      string
    BaseName string  // nombre base sin extensión: "logo", "hero"
}
```

### `convert.go` — procesamiento WebP

```go
//go:build !wasm

// ProcessImage convierte una imagen fuente a WebP en las variantes solicitadas.
// Usa disintegration/imaging para resize (Lanczos) + nativewebp para encode.
// No agranda imágenes más pequeñas que MaxWidth de la variante.
// Escribe directamente en outputDir — siempre en disco.
func ProcessImage(src ParsedAsset, outputDir string, quality int) error

// writeManifest escribe img-manifest.json en outputDir con name→alt para uso en HTML.
// nativewebp no soporta XMP — el alt se propaga via manifest, no embebido en el WebP.
func writeManifest(outputDir string, entries []manifestEntry) error
```

### `cache.go` — hash cache

```go
//go:build !wasm

func LoadCache(outputDir string) (*Cache, error)
func (c *Cache) IsUpToDate(absPath string, variants Variant) bool   // usa Variant, no []string
func (c *Cache) Update(modulePath, absPath, hash string, variants Variant, outputs []string)
func (c *Cache) Save(outputDir string) error
```

### `loader.go` — descubrimiento

```go
//go:build !wasm

// LoadImages descubre módulos via go list -m -json all en Config.RootDir,
// extrae RenderImages() de cada ssr.go, procesa solo las que no están en caché.
// Degrada silenciosamente si go list falla: warning + continúa sin procesar.
func (h *Handler) LoadImages() error

// ReloadModule re-extrae y re-procesa las imágenes de un único moduleDir.
// Llamado desde GoModHandler.OnSSRFileChange cuando ssr.go cambia.
func (h *Handler) ReloadModule(moduleDir string) error
```

## Dependencias

```
github.com/HugoSmits86/nativewebp  — WebP encoding
github.com/disintegration/imaging   — resize con Lanczos (mejor calidad para SEO)
```

**Calidad WebP default: 82** (no 80) — valor recomendado por Google para imágenes SEO.
Con `imaging.Lanczos` el resize preserva bordes nítidos, crítico para texto en imágenes
e indexación por Google Vision.

## SEO — consideraciones incorporadas

- **Alt text** en nombre de archivo descriptivo y en un `img-manifest.json` generado junto a los WebP — `nativewebp` no soporta XMP nativo; el alt debe proveerse al HTML desde el manifest
- **Naming descriptivo:** `hero.L.webp` es más legible para crawlers que `img001-lg.webp`
- **Lanczos resampling:** imágenes más nítidas → mejor puntuación en Core Web Vitals
- **No agrandar:** nunca generar variante de peor calidad que el original
- **Tamaño de archivo:** quality 82 + WebP garantiza < 200KB para M, < 80KB para S
  en fotografías típicas — cumple recomendaciones de PageSpeed

## Caching de imágenes borradas

Si un módulo elimina una imagen de `RenderImages()`, las variantes WebP anteriores quedan
huérfanas en `web/public/img/`. Al ejecutar `ReloadModule`, imagemin compara la lista
nueva con la caché y **elimina los archivos de salida huérfanos**.

## Tests requeridos en `tests/`

Todos en `package imagemin_test` (paquete externo).

### `tests/setup_test.go` — infraestructura compartida

Centraliza toda la lógica repetida: creación de handler, directorios temporales,
fixtures de `ssr.go` y helpers de verificación. Ningún test crea su propio handler
o directorio directamente — todo pasa por este setup.

```go
package imagemin_test

// TestEnv agrupa el estado compartido de un test.
type TestEnv struct {
    t         *testing.T
    ModuleDir string // t.TempDir() — directorio de módulo ficticio
    OutputDir string // t.TempDir() — web/public/img ficticio
    Handler   *imagemin.Handler
}

// newTestEnv crea un Handler con listModulesFn inyectada apuntando a ModuleDir.
// Registra t.Cleanup para limpiar OutputDir automáticamente.
func newTestEnv(t *testing.T) *TestEnv

// writeSSRGo escribe un ssr.go ficticio en env.ModuleDir con el contenido dado.
func (e *TestEnv) writeSSRGo(content string)

// writeSSRGoWithImages escribe un ssr.go que declara las imágenes dadas.
func (e *TestEnv) writeSSRGoWithImages(assets []imagemin.Asset)

// copyTestImage copia una imagen de testdata/ al ModuleDir en el path relativo dado.
// Ej: e.copyTestImage("img/logo.png", "small.jpg")
func (e *TestEnv) copyTestImage(destRelPath, testdataFile string)

// assertWebPExists verifica que el archivo WebP existe en OutputDir.
func (e *TestEnv) assertWebPExists(name string, v imagemin.Variant)

// assertWebPNotExists verifica que el archivo WebP NO existe en OutputDir.
func (e *TestEnv) assertWebPNotExists(name string, v imagemin.Variant)

// assertWebPDecodable verifica que el WebP es un archivo válido y decodificable.
func (e *TestEnv) assertWebPDecodable(name string, v imagemin.Variant)

// assertCacheHas verifica que el cache contiene la entrada para el absPath dado.
func (e *TestEnv) assertCacheHas(absPath string)

// assertCacheNotHas verifica que el cache NO contiene la entrada.
func (e *TestEnv) assertCacheNotHas(absPath string)

// variantName convierte Variant a sufijo de archivo: VariantS → "S", etc.
func variantName(v imagemin.Variant) string
```

### `tests/testdata/`

Imágenes de prueba reales — usadas via `copyTestImage`, nunca referenciadas directamente
en los tests individuales:

```
tests/testdata/
├── gopher.S.png  — PNG existente (~640px), para verificar PNG→WebP y no-upscale con VariantM/L
├── gatos.M.jpeg  — JPEG existente (~1024px), para resize a S y no-upscale con L
├── perros.L.jpg  — JPG existente (~1920px), para verificar resize a todas las variantes
└── corrupt.jpg   — archivo inválido (texto renombrado a .jpg) — generado en TestMain
```

Las imágenes de `testdata/` son las reales del proyecto (antes en `templates/images/`).
`corrupt.jpg` se genera programáticamente en `TestMain` con `imaging` — no se commitea.

### `tests/types_test.go`

Verifica el API público de `types.go` — sin build tag, debe compilar siempre.

| Test | Por qué |
|---|---|
| `TestVariantBitmask` | `AllVariants == VariantS\|VariantM\|VariantL` — contrato del bitmask correcto. Si alguien añade una constante nueva, este test fuerza actualizar `AllVariants`. |
| `TestVariantHasS` | `AllVariants & VariantS != 0` — verificar que los bits no se solapan entre variantes. |
| `TestVariantZeroValue` | `Variant(0)` no coincide con ninguna variante declarada — `0 & VariantS == 0`. Verifica que el zero value no genera variantes fantasma. |

### `tests/extract_test.go`

| Test | Por qué |
|---|---|
| `TestExtractImagesLiteral` | Caso base — slice literal con `imagemin.Asset{}`. Sin este test no hay garantía de que el AST parser funciona en absoluto. |
| `TestExtractImagesAllVariants` | `AllVariants` es una constante compuesta — el AST parser debe resolver `VariantS\|VariantM\|VariantL` a su valor uint8. Sin este test un bug en la resolución de constantes compuestas pasaría desapercibido. |
| `TestExtractImagesPartialVariants` | `VariantS \| VariantM` — verifica que el parser resuelve OR de constantes parciales correctamente, no solo `AllVariants`. |
| `TestExtractImagesAltEmpty` | `Alt: ""` — el parser no debe eliminar entradas con Alt vacío; imagemin debe derivar el alt del filename. |
| `TestExtractImagesNoRenderImages` | `ssr.go` sin `RenderImages()` → slice vacío, no error. Módulos sin imágenes son válidos. |
| `TestExtractImagesNoSSRFile` | Directorio sin `ssr.go` → slice vacío, no error. La mayoría de módulos proxy no tendrán imágenes. |
| `TestExtractImagesInvalidPath` | `Path: ""` en un Asset → error descriptivo, no procesar. Evita crear WebP con nombre vacío en disco. |
| `TestExtractAbsPathResolution` | `Path: "img/logo.png"` → `AbsPath` es `moduleDir + "/img/logo.png"`. Crítico — un error aquí hace que imagemin lea archivos del lugar equivocado. |

### `tests/convert_test.go`

| Test | Por qué |
|---|---|
| `TestConvertJPGToWebP` | Caso base — JPG → WebP existe en disco y es decodificable. Sin este test no sabemos si nativewebp funciona en el entorno. |
| `TestConvertPNGTransparency` | PNG con canal alpha → WebP debe preservar transparencia. WebP soporta alpha; si el encode la pierde, las imágenes con fondo transparente quedan con fondo negro — bug visual silencioso. |
| `TestConvertNoUpscale` | `gopher.S.png` (~640px) + `VariantL` (1920px target) → output WebP con ancho ≤ 640px (sin upscale). Sin este test imagemin podría agrandar imágenes degradando calidad y aumentando peso. Verifica también que el log emite un warning `"image smaller than target, skipping resize"`. |
| `TestConvertVariantSubset` | `VariantS\|VariantM` → solo `name.S.webp` y `name.M.webp` en disco, no existe `name.L.webp`. Verifica que el bitmask se interpreta correctamente en el procesamiento real. |
| `TestConvertOutputNaming` | `large.jpg` con `VariantL` → archivo de salida llamado `large.L.webp`. Contrato de naming — si cambia, rompe las referencias HTML del dev. |
| `TestConvertAltDerivedFromFilename` | `Alt: ""` + `Path: "img/my-hero.jpg"` → alt derivado es `"my hero"` (guiones a espacios). Importante para SEO automático cuando el dev no especifica alt. |
| `TestConvertQualityRange` | quality 82 → tamaño de archivo razonable (< original). Verifica que el quality setting tiene efecto real. |
| `TestConvertCorruptImage` | `corrupt.jpg` → error descriptivo, no panic, no archivo parcial en disco. Un archivo corrupto a medio guardar durante un save no debe romper el proceso completo. |
| `TestConvertOutputDirCreated` | OutputDir no existe → se crea automáticamente. En un proyecto nuevo `web/public/img/` no existe hasta que imagemin lo crea. |
| `TestVariantSkipsUpscale` | Imagen 640x320 + `VariantS` (target 720px) → WebP generado a 640x320 (dimensiones originales), no 720px. Verifica que el ancho del WebP resultante sea ≤ ancho original. Verifica que el log contiene `"skipping resize"`. El archivo de salida DEBE existir (se convierte a WebP igualmente) — solo se omite el resize, no la conversión. |

### `tests/cache_test.go`

| Test | Por qué |
|---|---|
| `TestCacheSkipsUnchanged` | Mismo hash → `ProcessImage` no se ejecuta. **Test más crítico del sistema de caché** — si falla, cada arranque reprocesa todas las imágenes aunque no hayan cambiado, bloqueando el dev. |
| `TestCacheReprocessesOnChange` | Hash diferente → reprocesa y actualiza `.cache.json`. Sin este test el caché podría quedar desactualizado sirviendo WebP viejos. |
| `TestCacheCleansOrphans` | Imagen removida de `RenderImages()` → `name.L.webp` borrado + entrada eliminada de caché. Sin este test las imágenes eliminadas quedan huérfanas en disco indefinidamente. |
| `TestCachePersistence` | Crear handler, procesar, destruir handler, crear nuevo handler → caché cargado del `.cache.json`, no reprocesa. Verifica que el caché sobrevive reinicios del proceso. |
| `TestCacheNewOutputDir` | `OutputDir` no tiene `.cache.json` → `LoadCache` retorna caché vacío sin error. Primer arranque en proyecto nuevo. |
| `TestCacheCorruptJSON` | `.cache.json` con JSON inválido → `LoadCache` retorna caché vacío + warning, no error fatal. Un archivo corrupto no debe bloquear el arranque. |
| `TestCacheCorruptJSON` | `.cache.json` con JSON inválido → `LoadCache` retorna caché vacío + warning, no error fatal. Un archivo corrupto no debe bloquear el arranque. |

### `tests/loader_test.go`

| Test | Por qué |
|---|---|
| `TestLoadImagesFromModule` | Módulo con `RenderImages()` + imágenes reales → WebP en OutputDir. Test de integración end-to-end del flujo completo. |
| `TestLoadImagesSkipsCached` | Segunda llamada a `LoadImages()` → no reprocesa (verifica integración caché + loader). Cubre idempotencia y skip por hash simultáneamente. |
| `TestLoadImagesGoListFails` | `listModulesFn` retorna error → warning en log, no panic, continúa sin procesar. El dev sin internet no debe ver el servidor bloqueado. |
| `TestLoadImagesRootDirEmpty` | `Config.RootDir=""` → error claro antes de ejecutar `go list`. Evita ejecutar `go list` en directorio incorrecto. |
| `TestReloadModuleNewImage` | `ssr.go` modificado con imagen nueva → nueva variante WebP aparece en OutputDir. Valida el hot reload completo. |
| `TestReloadModuleRemovedImage` | Imagen eliminada de `RenderImages()` → WebP anterior borrado del disco. Sin este test el dev acumula imágenes obsoletas en `web/public/img/`. |
| `TestReloadModuleNoSSRFile` | `moduleDir` sin `ssr.go` → no error, no cambios en disco. Módulos que no declaran imágenes son válidos. |
| `TestWaitForLoadTimeout` | `WaitForLoad(1ms)` con proceso lento → retorna sin bloquear. Solo para tests — evita que otros tests cuelguen esperando. |

### `tests/handler_test.go`

| Test | Por qué |
|---|---|
| `TestHandlerUnobservedFiles` | `Handler.UnobservedFiles()` incluye `OutputDir` y `.cache.json`. Sin este test devwatch vigila el JSON de caché y dispara un loop infinito de eventos al escribirlo. |
| `TestHandlerNewFileEventNoop` | `NewFileEvent(...)` retorna `nil` sin efectos secundarios — es un no-op requerido por la interfaz. Verifica que no procesa archivos accidentalmente. |

### `tests/concurrency_test.go`

| Test | Por qué |
|---|---|
| `TestReloadConcurrency` | 5 goroutines llamando `ReloadModule` simultáneamente → sin data race en caché, archivos WebP consistentes. El watcher puede disparar múltiples eventos de `ssr.go` en ráfaga (save rápido del editor). Sin este test con `-race` podrían aparecer corrupciones silenciosas en `.cache.json`. |
| `TestLoadAndReloadConcurrent` | `LoadImages()` en goroutine + `ReloadModule()` simultáneo → sin deadlock. Escenario real: imagemin carga módulos proxy mientras llega un evento de hot reload de un módulo local. |

## Orden de implementación

1. `types.go` + `tests/types_test.go` — base de todo el sistema de tipos, sin dependencias
2. `cache.go` + `tests/cache_test.go` — depende solo de `types.go`
3. `convert.go` + `tests/convert_test.go` — depende de `types.go`; núcleo del procesamiento WebP
4. `extract.go` + `tests/extract_test.go` — depende de `types.go`; AST parser independiente
5. `imagemin.go` + `tests/handler_test.go` — Handler struct + Config + interfaces devwatch
6. `loader.go` + `tests/loader_test.go` — depende de todos los anteriores + `go list`
7. `tests/concurrency_test.go` — race conditions, requiere loader completo
8. Integración en `tinywasm/app` (ver `tinywasm/app/docs/PLAN.md`)

## Estado de dependencias

| Módulo | Estado |
|---|---|
| `tinywasm/devflow` | ✅ v0.4.16 — `OnSSRFileChange` listo |
| `tinywasm/assetmin` | Pendiente — PLAN.md en revisión |
| `tinywasm/imagemin` | Pendiente — este plan |
| `tinywasm/app` | Pendiente — requiere assetmin + imagemin publicados |

## Preguntas abiertas para revisión

**Q1.** Si dos módulos declaran una imagen con el mismo `BaseName` (ej: ambos tienen `logo`),
el segundo sobreescribe el primero en `web/public/img/`. ¿Se prefiere agregar prefijo
automático del módulo (`ce-logo.L.webp`) o documentar la convención y que el dev evite colisiones?

**Q2.** El `Alt` text en XMP de WebP es leído por Google Images pero no todos los
decodificadores lo respetan. ¿También debe incluirse en el HTML generado, o imagemin
solo produce los archivos y el HTML es responsabilidad del dev/módulo?

**Q3.** ¿`web/public/img/.cache.json` debe estar en `UnobservedFiles()` para que devwatch
no lo vigile y evitar loops de eventos? Confirmar que sí.
