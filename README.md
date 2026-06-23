# Clasificador Pedagógico Multimodal

Aplicación web para clasificar recursos pedagógicos multimodales, como PDFs e imágenes, usando Gemini 2.5 Flash.

## Arquitectura

```text
GitHub Pages /docs/index.html
        ↓ fetch()
Railway backend Go /analizar
        ↓
PostgreSQL cache anónima por SHA-256
        ↓ si no existe
Gemini API
```

## Estructura

```text
.
├── backend/
│   ├── main.go
│   ├── cache.go
│   ├── go.mod
│   ├── railway.json
│   └── .gitignore
└── docs/
    └── index.html
```

## Funcionalidades incluidas

- Frontend estático compatible con GitHub Pages.
- Drag & drop de PDFs, PNG, JPG y WEBP.
- Tabla de resultados.
- Exportación CSV desde el navegador.
- Backend Go preparado para Railway.
- CORS configurable.
- API Key de Gemini protegida como variable de entorno.
- Validación de tipo y tamaño de archivo.
- Reintentos básicos ante errores temporales de Gemini.
- Salida JSON estructurada con categorías cerradas.
- Caché anónima opcional en PostgreSQL usando SHA-256 del contenido.
- Indicador de origen del resultado: `IA` o `Caché`.

## Cómo funciona la caché anónima

El backend no guarda PDFs, imágenes ni contenido textual del archivo. Calcula una huella SHA-256 del contenido y la usa como clave anónima.

```text
Usuario sube archivo
        ↓
Backend calcula SHA-256
        ↓
Busca clasificación existente en PostgreSQL
        ↓
Si existe: devuelve resultado de Caché
        ↓
Si no existe: llama a Gemini, guarda clasificación y devuelve resultado de IA
```

La tabla se crea automáticamente al arrancar el backend si `DATABASE_URL` está configurada.

Campos guardados:

```text
file_hash
file_size
mime_type
edad
area_principal
justificacion
created_at
last_used_at
uses_count
```

No se guarda el nombre original del archivo para mantener la base de datos más anónima.

## Despliegue del backend en Railway

1. Crea un nuevo proyecto en Railway desde este repositorio.
2. Selecciona como directorio raíz del servicio:

```text
backend
```

3. Añade las variables de entorno:

```text
GEMINI_API_KEY=tu_api_key_de_gemini
FRONTEND_ORIGIN=https://wolcenon.github.io
```

4. Para activar la caché, añade PostgreSQL en Railway y conecta su variable al backend:

```text
DATABASE_URL=postgresql://...
```

En Railway, normalmente se puede hacer desde el servicio backend añadiendo una referencia a la variable `DATABASE_URL` del servicio PostgreSQL.

Nota sobre CORS: el navegador envía como origen `https://wolcenon.github.io`, sin la ruta `/clasificador-pedagogico`.

5. Railway usará `backend/railway.json` para compilar y arrancar el servidor.

## Activar GitHub Pages

1. Entra en Settings → Pages.
2. En “Build and deployment”, selecciona:
   - Source: `Deploy from a branch`
   - Branch: `main`
   - Folder: `/docs`
3. Guarda los cambios.

Tu frontend quedará publicado normalmente en:

```text
https://wolcenon.github.io/clasificador-pedagogico/
```

## Uso

1. Abre la web de GitHub Pages.
2. Introduce la URL pública de Railway, por ejemplo:

```text
https://tu-proyecto.up.railway.app
```

3. Sube PDFs o imágenes.
4. Pulsa “Analizar recursos”.
5. Revisa si cada resultado viene de `IA` o `Caché`.
6. Descarga los resultados en CSV.

## Comprobar estado del backend

```text
https://tu-proyecto.up.railway.app/health
```

Respuesta esperada sin PostgreSQL:

```json
{"status":"ok","cache":"disabled"}
```

Respuesta esperada con PostgreSQL:

```json
{"status":"ok","cache":"enabled"}
```

## Límites actuales

Esta versión usa archivos inline en la petición a Gemini para simplificar el despliegue inicial. Por eso limita cada archivo a 10 MB. La siguiente mejora natural es migrar a Gemini Files API para manejar PDFs más grandes y reducir presión de memoria.

## Próximas mejoras recomendadas

- Worker Pool con concurrencia limitada.
- Progreso real con SSE o polling.
- Exportación Excel `.xlsx`.
- Gemini Files API para documentos grandes.
- Panel de estadísticas de caché.
- Historial de análisis sin datos personales.
