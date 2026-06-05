# ARCHIVADO: imagemin fusionado en tinywasm/image

`tinywasm/imagemin` se fusionó dentro de `tinywasm/image`. No agregar nuevo código aquí.

## Arquitectura final

El código de imagemin vive en dos destinos dentro de `github.com/tinywasm/image`:

- `github.com/tinywasm/image` — builders HTML: `Img()`, `Picture()`, `Source()`, tipos `Asset`, `Variant`
- `github.com/tinywasm/image/min` — pipeline de optimización: `Handler`, `Config`, `New()`, `ExtractImages()`, `LoadImages()`, etc.

El subpaquete `min` no necesita build tags — nunca se importa desde código wasm.

## Qué se movió y adónde

| Archivo imagemin | Destino | Cambio de paquete |
|------------------|---------|-------------------|
| `types.go` | `image/types.go` | `imagemin` → `image` |
| `convert.go` | `image/min/convert.go` | `imagemin` → `min` |
| `extract.go` | `image/min/extract.go` | `imagemin` → `min` |
| `cache.go` | `image/min/cache.go` | `imagemin` → `min` |
| `loader.go` | `image/min/loader.go` | `imagemin` → `min` |
| `imagemin.go` | `image/min/handler.go` | `imagemin` → `min` |
| `tests/*` | `image/tests/*` | `imagemin_test` → `image_test` |

## Migración de importadores

```go
// ANTES (imagemin):
import "github.com/tinywasm/imagemin"
handler := imagemin.New(&imagemin.Config{...})

// DESPUÉS (image/min):
import "github.com/tinywasm/image/min"
handler := min.New(&min.Config{...})
```

Contrato SSR en componentes — solo cambia el import:
```go
// ANTES:
import "github.com/tinywasm/imagemin"
func RenderImages() []imagemin.Asset { ... }

// DESPUÉS:
import "github.com/tinywasm/image"
func RenderImages() []image.Asset { ... }
```

Ver `tinywasm/image/docs/PLAN.md` para los detalles de ejecución.
