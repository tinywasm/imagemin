# PLAN: imagemin — WebP Image Processing (mtime-based)

## Objetivo
`imagemin` descubre imágenes declaradas en archivos `ssr.go` de módulos Go, las convierte a WebP con variantes responsivas (L, M, S) y las escribe en `web/public/img/`. 

**Principios de esta refactorización:**
1. **Cero archivos JSON**: Se elimina la generación de `img-manifest.json` y `.cache.json`.
2. **Caché basado en mtime**: El sistema decidirá si procesar una imagen comparando la fecha de modificación (`mtime`) del archivo fuente con la del WebP de salida.
3. **Limpieza Global**: Los archivos huérfanos se limpian de forma global durante el arranque.

## 1. Arquitectura y Tipos
El paquete se divide en archivos especializados con build tags `!wasm` (excepto `types.go`):

- **`types.go`** (Sin build tag): Contiene el API público.
  - `type Variant uint8` (S=640px, M=1024px, L=1920px).
  - `type Asset struct { Path, Variants, Alt }`.
- **`imagemin.go`**: Estructura `Handler`, `Config` e interfaces para `devwatch`.
- **`extract.go`**: Parser AST para extraer `RenderImages()` de los archivos `ssr.go`. Soporta literales y variables locales.
- **`convert.go`**: Procesamiento de imagen usando `imaging` (Lanczos) y `nativewebp`.
- **`loader.go`**: Orquestación, descubrimiento vía `go list` y lógica de limpieza.

## 2. Lógica de Procesamiento (mtime)
Para evitar reprocesamientos innecesarios sin usar un archivo de caché persistente:

1. Al procesar un `Asset`, se verifica `isUpToDate(srcPath, variants, outputDir)`:
   - Si para cada variante activa existe un archivo `.webp` en el `outputDir`.
   - Y si el `mtime` del archivo fuente es **menor o igual** al `mtime` de **todos** los archivos de salida correspondientes.
2. Si se cumplen ambas condiciones, se salta el procesamiento. De lo contrario, se genera/sobrescribe el WebP.

## 3. Limpieza de Huérfanos (Orphan Cleanup)
Sin un registro histórico (`.cache.json`), la limpieza se vuelve global para evitar borrar archivos de otros módulos accidentalmente durante un hot-reload parcial.

- **`LoadImages()`**: Al arrancar, recopila todos los nombres base (`BaseName`) de todos los módulos del proyecto. Luego, escanea `web/public/img/` y borra cualquier `.webp` cuyo nombre base no esté en la lista global.
- **`ReloadModule()`**: Solo procesa/actualiza las imágenes del módulo cambiado. **No realiza limpieza de huérfanos** para evitar estados inconsistentes si otros módulos aún no han sido cargados.

## 4. Pruebas Requeridas (`tests/`)

La suite de pruebas debe garantizar la integridad del sistema tras remover los JSON:

### `tests/mtime_test.go` (Reemplaza cache_test.go)
- `TestMtimeSkipsUnchanged`: Procesar -> Volver a procesar -> Verificar que no hubo cambios (mtime idéntico).
- `TestMtimeReprocessesOnChange`: Procesar -> Tocar archivo fuente (`os.Chtimes` al futuro) -> Verificar que se generó un nuevo WebP.
- `TestMtimeMissingVariant`: Borrar una de las variantes WebP manualmente -> Verificar que el sistema la regenera.

### `tests/loader_test.go`
- `TestGlobalOrphanCleanup`: Crear archivos basura en `OutputDir` -> Ejecutar `LoadImages()` -> Verificar que desaparecen.
- `TestLoadImagesFromModule`: Integración completa desde `ssr.go` hasta `.webp` en disco.
- `TestReloadModuleIdempotencia`: Llamar múltiples veces a `ReloadModule` no debe regenerar archivos si no hay cambios.

### `tests/convert_test.go`
- `TestConvertPNGTransparency`: Verificar que el canal alpha se preserva.
- `TestConvertNoUpscale`: Si la fuente es 100x100 y se pide variante L (1920px), el WebP resultante debe medir 100x100.
- `TestConvertCorruptImage`: Manejo grácil de archivos que no son imágenes.

### `tests/extract_test.go`
- `TestExtractImagesLiteral`: Extracción de slice literal en el `return`.
- `TestExtractImagesLocalVar`: Extracción cuando se retorna una variable local definida previamente.

## 5. Actualización de Documentación
- **`README.md`**: Eliminar sección `## Manifest`. Actualizar instrucciones eliminando menciones a archivos `.json` generados. Explicar que el `Alt` text es responsabilidad del componente de UI que consuma las imágenes.

## 6. Dependencias
- `github.com/HugoSmits86/nativewebp` (WebP encoding)
- `github.com/disintegration/imaging` (Resizing con Lanczos)