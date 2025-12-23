# Image Processing - Questions & Answers

## General Questions

### Q1: ¿Por qué usar WebP en lugar de mantener JPG/PNG?
**A:** WebP ofrece mejor compresión que JPG/PNG (generalmente 25-35% menor tamaño) sin pérdida visible de calidad. Es soportado por todos los navegadores modernos (Chrome, Firefox, Safari, Edge). Los usuarios se benefician de tiempos de carga más rápidos y menor consumo de datos.

### Q2: ¿Qué pasa si un navegador no soporta WebP?
**A:** Los navegadores modernos (>95% del mercado) soportan WebP desde hace años:
- Chrome: desde 2010
- Firefox: desde 2019
- Safari: desde 2020
- Edge: desde 2018

Para navegadores muy antiguos, el elemento `<picture>` permite añadir fallbacks (no incluido en implementación inicial por simplicidad).

### Q3: ¿Por qué tres variantes (lg/md/sm) en lugar de más?
**A:** Tres variantes cubren los casos más comunes (móvil, tablet, desktop) con un buen balance entre optimización y complejidad. Más variantes = más archivos, más tiempo de build, más almacenamiento, pero beneficio marginal. Si necesitas más control, puedes personalizar los `Variants` en la configuración.

### Q4: ¿Puedo desactivar el procesamiento de imágenes?
**A:** Sí, establece `ImageConfig.Enabled = false` en tu configuración. El resto de AssetMin funcionará normalmente.

---

## Configuration Questions

### Q5: ¿Cómo personalizo los tamaños de las variantes?
**A:** Personaliza `ImageConfig.Variants`:

```go
config := &assetmin.AssetConfig{
    ImageConfig: &assetmin.ImageConfig{
        Variants: []assetmin.ImageVariant{
            {Name: "xlarge", MaxWidth: 2560, Suffix: "-xl"},
            {Name: "large", MaxWidth: 1920, Suffix: "-lg"},
            {Name: "medium", MaxWidth: 1024, Suffix: "-md"},
            {Name: "small", MaxWidth: 640, Suffix: "-sm"},
            {Name: "tiny", MaxWidth: 320, Suffix: "-xs"},
        },
    },
}
```

### Q6: ¿Puedo cambiar la calidad de WebP?
**A:** Sí, ajusta `ImageConfig.Quality` (0-100):

```go
config := &assetmin.AssetConfig{
    ImageConfig: &assetmin.ImageConfig{
        Quality: 85,  // Mayor calidad, archivos más grandes
    },
}
```

**Recomendación:** 80 es un buen balance. 85-90 para fotografía profesional, 70-75 para uso general.

### Q7: ¿Puedo cambiar las carpetas de entrada/salida?
**A:** Sí:

```go
config := &assetmin.AssetConfig{
    ImageConfig: &assetmin.ImageConfig{
        InputFolder:  "assets/photos",    // Relativo a ThemeFolder
        OutputFolder: "img",              // Relativo a WebFilesFolder
    },
}
```

### Q8: ¿Puedo usar diferentes carpetas para diferentes tipos de imágenes?
**A:** En la implementación inicial, no. Todas las imágenes van a la misma carpeta. Esto se podría añadir en el futuro con una configuración más avanzada:

```go
// Posible mejora futura (no implementado)
ImageConfig: &assetmin.ImageConfig{
    Folders: []ImageFolder{
        {Input: "photos", Output: "gallery"},
        {Input: "logos", Output: "brand"},
    },
}
```

---

## Technical Questions

### Q9: ¿Qué algoritmo de redimensionamiento se usa?
**A:** Lanczos resampling (de la librería `disintegration/imaging`). Es uno de los mejores para calidad, especialmente al reducir tamaños. Es más lento que métodos simples pero produce resultados superiores.

### Q10: ¿Se mantiene el ratio de aspecto?
**A:** Sí, siempre. `MaxHeight: 0` indica "mantener aspect ratio basado en MaxWidth". Si necesitas crop específico, eso sería una mejora futura.

### Q11: ¿Qué pasa con imágenes más pequeñas que MaxWidth?
**A:** No se agrandan. El código verifica el tamaño antes de redimensionar:

```go
if width <= variant.MaxWidth {
    return img // No resize needed
}
```

Esto evita pérdida de calidad por agrandamiento artificial.

### Q12: ¿Se procesan imágenes en paralelo?
**A:** En la implementación inicial, no. Se procesan secuencialmente. Si tienes muchas imágenes y el build es lento, se podría añadir procesamiento paralelo:

```go
// Posible mejora futura
ImageConfig: &assetmin.ImageConfig{
    MaxConcurrency: 4,  // Procesar 4 imágenes simultáneamente
}
```

### Q13: ¿Cómo afecta esto al tiempo de build?
**A:** Depende del número y tamaño de las imágenes:
- 5 imágenes: +200-300ms
- 10 imágenes: +400-600ms
- 50 imágenes: +2-3 segundos

El procesamiento es lineal (~40ms por imagen). Para optimizar, considera procesamiento paralelo o caching basado en checksums.

---

## Workflow Questions

### Q14: ¿Cuándo se procesan las imágenes?
**A:** En dos momentos:
1. **Al iniciar AssetMin:** Procesa todas las imágenes existentes en `ThemeFolder/images/`
2. **En cada file event:** Cuando el file watcher detecta cambios en archivos de imagen

### Q15: ¿Qué pasa si borro una imagen fuente?
**A:** Las variantes procesadas NO se borran automáticamente. Esto es intencional para evitar eliminar assets en producción accidentalmente. Si quieres limpiar variantes huérfanas, puedes:

```bash
# Manual cleanup
rm -rf web/public/images/*.webp
# Restart app para regenerar solo las imágenes actuales
```

Una mejora futura podría añadir limpieza automática con un flag de configuración.

### Q16: ¿Puedo forzar reprocesamiento de todas las imágenes?
**A:** Sí, borra las imágenes de salida y reinicia la aplicación:

```bash
rm -rf web/public/images/*
go run main.go  # Reprocesará todas las imágenes al inicio
```

### Q17: ¿Funciona con hot reload / file watchers?
**A:** Sí, completamente. Cuando guardas/modificas una imagen en `ThemeFolder/images/`, el file watcher envía un evento que dispara el reprocesamiento automático. Las tres variantes se actualizan en el output folder.

---

## Template & HTML Questions

### Q18: ¿Las imágenes se incluyen automáticamente en el HTML?
**A:** Solo en el template por defecto (`index_basic.html`). Si personalizas tu `theme/index.html`, debes incluir las imágenes manualmente con el elemento `<picture>`:

```html
<picture>
    <source media="(min-width: 1024px)" srcset="images/photo-lg.webp">
    <source media="(min-width: 640px)" srcset="images/photo-md.webp">
    <img src="images/photo-sm.webp" alt="Photo" loading="lazy">
</picture>
```

### Q19: ¿Puedo usar solo una variante en mi HTML?
**A:** Sí, puedes usar cualquier variante directamente:

```html
<!-- Solo desktop -->
<img src="images/photo-lg.webp" alt="Photo">

<!-- Solo mobile -->
<img src="images/photo-sm.webp" alt="Photo">
```

Pero pierdes el beneficio de responsive images. Se recomienda usar el elemento `<picture>` completo.

### Q20: ¿Cómo personalizo el alt text?
**A:** Por defecto, se genera del nombre del archivo (`photo-sunset.jpg` → `alt="Photo sunset"`). Para personalizar:

1. **Opción 1:** Nombra tus archivos descriptivamente
2. **Opción 2:** Edita manualmente el HTML generado en `theme/index.html`
3. **Opción 3 (mejora futura):** Configuración de alt text en `ImageConfig`

---

## Error Handling Questions

### Q21: ¿Qué pasa si una imagen está corrupta?
**A:** El procesamiento falla con un error descriptivo y detiene el build (fail fast). Esto te obliga a corregir el problema antes de continuar. Ejemplo:

```
Error: failed to load image: image: unknown format
```

### Q22: ¿Puedo hacer que continúe aunque falle una imagen?
**A:** En la implementación inicial, no (fail fast es más seguro). Una mejora futura podría añadir:

```go
ImageConfig: &assetmin.ImageConfig{
    ContinueOnError: true,  // Log warning pero continúa
}
```

### Q23: ¿Hay límites de tamaño de archivo?
**A:** No hay límites explícitos en el código inicial. Las librerías de imagen pueden manejar archivos grandes, pero considera:
- **Práctico:** Imágenes >10MB son innecesarias para web
- **Recomendado:** Mantén fuentes <5MB
- **Mejora futura:** Añadir `MaxInputSize` en configuración

---

## Integration Questions

### Q24: ¿Funciona con golite?
**A:** Sí, ese es el caso de uso principal. Golite llama a `AssetMin.NewFileEvent()` cuando detecta cambios en archivos. Con las extensiones de imagen añadidas a `SupportedExtensions()`, los eventos de imagen se procesarán automáticamente.

### Q25: ¿Necesito cambiar mi código de golite?
**A:** No. Solo necesitas tener imágenes en `ThemeFolder/images/`. AssetMin las detectará y procesará automáticamente. Si quieres personalizar la configuración de imágenes, pasa `ImageConfig` al crear AssetMin:

```go
// En golite o tu aplicación
assetsHandler := assetmin.NewAssetMin(&assetmin.AssetConfig{
    ThemeFolder: func() string { return "web/theme" },
    WebFilesFolder: func() string { return "web/public" },
    ImageConfig: &assetmin.ImageConfig{
        Quality: 85,
        // ... personalización
    },
})
```

### Q26: ¿Puedo usar AssetMin solo para imágenes (sin JS/CSS)?
**A:** Técnicamente sí, pero AssetMin está diseñado para procesar todos los assets juntos. Si solo necesitas procesamiento de imágenes, considera usar las librerías directamente (`nativewebp`, `imaging`) o crear una herramienta CLI dedicada.

---

## Performance Questions

### Q27: ¿Cómo optimizo el tiempo de build con muchas imágenes?
**A:** Opciones:

1. **Procesamiento paralelo** (mejora futura):
   ```go
   ImageConfig: &assetmin.ImageConfig{
       MaxConcurrency: 4,
   }
   ```

2. **Caching basado en checksums** (mejora futura):
   - Solo reprocesar si el contenido cambió
   - Comparar hash del archivo fuente

3. **Procesamiento selectivo**:
   - Mantén imágenes grandes fuera de ThemeFolder
   - Procesa manualmente imágenes que rara vez cambian

4. **Desactivar en desarrollo**:
   ```go
   ImageConfig: &assetmin.ImageConfig{
       ProcessExistingOnStartup: false,  // Solo procesar nuevas
   }
   ```

### Q28: ¿Las variantes se cachean?
**A:** No en la implementación inicial. Cada build regenera todas las variantes. Para optimizar:

1. **Mejora futura:** Content-based hashing
2. **Workaround actual:** No borres el output folder entre builds

### Q29: ¿Cuánto espacio en disco ocupan las variantes?
**A:** Aproximadamente:
- 1 imagen JPG 2MB → 3 variantes WebP (~250KB + 150KB + 80KB) = 480KB total
- **Ahorro:** 76% vs original
- **Costo:** 3 archivos vs 1

Para 50 imágenes: ~24MB total vs ~100MB originales.

---

## Advanced Questions

### Q30: ¿Puedo añadir soporte para AVIF?
**A:** Sí, como extensión futura. Requiere:
1. Añadir librería AVIF encoder
2. Añadir opción de formato en configuración
3. Generar variantes adicionales en formato AVIF

Estructura propuesta:
```
photo-lg.webp
photo-lg.avif
photo-md.webp
photo-md.avif
...
```

### Q31: ¿Puedo añadir marcas de agua?
**A:** No está implementado, pero es posible con `imaging` library:

```go
// Mejora futura
ImageConfig: &assetmin.ImageConfig{
    Watermark: &WatermarkConfig{
        Image:    "watermark.png",
        Position: "bottom-right",
        Opacity:  0.5,
    },
}
```

### Q32: ¿Puedo generar imágenes con diferentes crops (aspect ratios)?
**A:** No en implementación inicial (siempre mantiene aspect ratio). Mejora futura podría añadir:

```go
// Posible mejora futura
Variants: []ImageVariant{
    {Name: "hero", Width: 1920, Height: 600, Crop: true},  // 16:5 crop
    {Name: "thumb", Width: 300, Height: 300, Crop: true},  // Square crop
}
```

### Q33: ¿Puedo integrar con un CDN?
**A:** AssetMin genera archivos estáticos que puedes subir a cualquier CDN. El upload al CDN sería responsabilidad de tu pipeline de deploy, no de AssetMin.

Ejemplo workflow:
```bash
# 1. AssetMin procesa
go run main.go

# 2. Deploy a CDN
aws s3 sync web/public s3://my-cdn-bucket
# o
netlify deploy --prod --dir=web/public
```

---

## Troubleshooting

### Q34: Error: "image: unknown format"
**A:** El archivo no es una imagen válida o el formato no es soportado. Verifica:
- ¿Es realmente JPG/PNG?
- ¿Está corrupto el archivo?
- ¿La extensión es correcta?

### Q35: Las variantes no aparecen en el output
**A:** Checklist:
1. ¿`ImageConfig.Enabled = true`?
2. ¿Las imágenes están en `ThemeFolder/images/`?
3. ¿Tiene permisos de escritura en output folder?
4. ¿Hay errores en los logs?
5. ¿El file watcher está funcionando?

### Q36: Las imágenes se ven de mala calidad
**A:** Ajusta la calidad WebP:

```go
ImageConfig: &assetmin.ImageConfig{
    Quality: 90,  // Mayor calidad (default: 80)
}
```

Ten en cuenta que mayor calidad = archivos más grandes.

### Q37: El build es muy lento
**A:** Causas comunes:
1. **Muchas imágenes grandes:** Reduce tamaño de fuentes antes de procesar
2. **Discos lentos:** Usa SSD en lugar de HDD
3. **Procesamiento secuencial:** Espera implementación de paralelización
4. **Reprocesamiento innecesario:** Espera implementación de caching

Workaround: Desactiva procesamiento en desarrollo y activa solo para producción.

---

## Future Features

### Q38: ¿Qué mejoras están planeadas?
Posibles mejoras futuras (no implementadas):

1. **Procesamiento paralelo** - Múltiples imágenes simultáneamente
2. **Caching inteligente** - Solo reprocesar si cambió el contenido
3. **Soporte AVIF** - Formato de próxima generación
4. **Animated WebP** - Convertir GIF a WebP animado
5. **Smart cropping** - Diferentes aspect ratios por variante
6. **Marcas de agua** - Watermarking automático
7. **Metadata preservation** - Mantener EXIF data
8. **Lazy loading placeholder** - Generar blur placeholders
9. **Art direction** - Diferentes crops para diferentes dispositivos
10. **Alt text inteligente** - Generación con AI/ML

### Q39: ¿Cómo puedo contribuir o sugerir mejoras?
Abre un issue en el repositorio de GitHub con:
- Descripción del caso de uso
- Problema que resuelve
- Propuesta de API/configuración
- Ejemplos de uso

### Q40: ¿Dónde reporto bugs?
GitHub Issues del proyecto assetmin. Incluye:
- Versión de Go y librerías
- Configuración usada
- Archivo de imagen problemático (si es pequeño)
- Logs de error completos
- Pasos para reproducir

---

## Quick Reference

### Configuración Mínima (Usar Defaults)
```go
// Solo con defaults
config := &assetmin.AssetConfig{
    ThemeFolder:    func() string { return "web/theme" },
    WebFilesFolder: func() string { return "web/public" },
}
handler := assetmin.NewAssetMin(config)
```

### Configuración Personalizada
```go
config := &assetmin.AssetConfig{
    ThemeFolder:    func() string { return "web/theme" },
    WebFilesFolder: func() string { return "web/public" },
    ImageConfig: &assetmin.ImageConfig{
        InputFolder:  "images",
        OutputFolder: "images",
        Quality:      85,
        Enabled:      true,
        Variants: []assetmin.ImageVariant{
            {Name: "desktop", MaxWidth: 1920, Suffix: "-lg"},
            {Name: "tablet",  MaxWidth: 1024, Suffix: "-md"},
            {Name: "mobile",  MaxWidth: 640,  Suffix: "-sm"},
        },
    },
}
```

### HTML Responsive Pattern
```html
<picture>
    <source media="(min-width: 1024px)" srcset="images/photo-lg.webp">
    <source media="(min-width: 640px)" srcset="images/photo-md.webp">
    <img src="images/photo-sm.webp" alt="Description" loading="lazy">
</picture>
```

### Estructura de Archivos
```
project/
├── web/
│   ├── theme/              (ThemeFolder)
│   │   └── images/
│   │       ├── photo1.jpg
│   │       ├── photo2.png
│   │       └── logo.jpeg
│   └── public/             (WebFilesFolder)
│       └── images/
│           ├── photo1-lg.webp
│           ├── photo1-md.webp
│           ├── photo1-sm.webp
│           ├── photo2-lg.webp
│           ├── photo2-md.webp
│           ├── photo2-sm.webp
│           ├── logo-lg.webp
│           ├── logo-md.webp
│           └── logo-sm.webp
```
