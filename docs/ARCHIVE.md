# ARCHIVADO: imagemin fusionado en webtyp/image

`webtyp/imagemin` se fusionó dentro de `webtyp/image`. No agregar nuevo código aquí.

## Arquitectura final

El código de imagemin vive en dos destinos dentro de `webtyp.com/image`:

- `webtyp.com/image` — builders HTML: `Img()`, `Picture()`, `Source()`, tipos `Asset`, `Variant`
- `webtyp.com/image/min` — pipeline de optimización: `Handler`, `Config`, `New()`, `ExtractImages()`, `LoadImages()`, etc.

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
import "webtyp.com/imagemin"
handler := imagemin.New(&imagemin.Config{...})

// DESPUÉS (image/min):
import "webtyp.com/image/min"
handler := min.New(&min.Config{...})
```

Contrato SSR en componentes — solo cambia el import:
```go
// ANTES:
import "webtyp.com/imagemin"
func RenderImages() []imagemin.Asset { ... }

// DESPUÉS:
import "webtyp.com/image"
func RenderImages() []image.Asset { ... }
```

Ver `webtyp/image/docs/PLAN.md` para los detalles de ejecución.
