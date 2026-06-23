# Backend Go para Railway

Servidor HTTP en Go que recibe recursos pedagógicos mediante `multipart/form-data`, llama a Gemini 2.5 Flash y devuelve una clasificación JSON por archivo.

## Endpoints

### `GET /health`

Devuelve:

```json
{"status":"ok"}
```

### `POST /analizar`

Recibe archivos en el campo multipart:

```text
recursos
```

Devuelve:

```json
{
  "results": [
    {
      "fileName": "ficha.pdf",
      "edad": "3-5 años",
      "area_principal": "Grafomotricidad y Preescritura",
      "justificacion": "La presencia de trazos guiados indica trabajo de control motor fino.",
      "status": "ok"
    }
  ]
}
```

## Variables de entorno

```text
GEMINI_API_KEY=...
FRONTEND_ORIGIN=https://wolcenon.github.io
PORT=8080
```

`PORT` lo establece Railway automáticamente.

## Ejecución local

```bash
cd backend
export GEMINI_API_KEY="tu_api_key"
export FRONTEND_ORIGIN="*"
go run .
```

Luego prueba:

```bash
curl http://localhost:8080/health
```
