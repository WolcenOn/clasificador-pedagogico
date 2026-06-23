# Clasificador Pedagógico Multimodal

Aplicación web para clasificar recursos pedagógicos multimodales, como PDFs e imágenes, usando Gemini 2.5 Flash.

## Arquitectura

```text
GitHub Pages /docs/index.html
        ↓ fetch()
Railway backend Go /analizar
        ↓
Gemini API
```

## Estructura

```text
.
├── backend/
│   ├── main.go
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

Si ya conoces la URL exacta de GitHub Pages, puedes usarla completa, por ejemplo:

```text
FRONTEND_ORIGIN=https://wolcenon.github.io/clasificador-pedagogico
```

Nota: si CORS falla, usa temporalmente `*` para probar y luego vuelve a restringirlo.

4. Railway usará `backend/railway.json` para compilar y arrancar el servidor.

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
5. Descarga los resultados en CSV.

## Límites actuales de la primera versión

Esta versión usa archivos inline en la petición a Gemini para simplificar el despliegue inicial. Por eso limita cada archivo a 10 MB. La siguiente mejora natural es migrar a Gemini Files API para manejar PDFs más grandes y reducir presión de memoria.

## Próximas mejoras recomendadas

- Worker Pool con concurrencia limitada.
- Progreso real con SSE o polling.
- Exportación Excel `.xlsx`.
- Gemini Files API para documentos grandes.
- Autenticación de usuarios.
- Historial de análisis.
