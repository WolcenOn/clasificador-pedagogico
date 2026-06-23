# Backend Go para Railway

Servidor HTTP en Go que recibe recursos pedagógicos mediante `multipart/form-data`, consulta una caché anónima opcional en PostgreSQL, llama a Gemini 2.5 Flash cuando no existe clasificación previa y devuelve una clasificación JSON por archivo.

## Endpoints

### `GET /health`

Devuelve:

```json
{"status":"ok","cache":"enabled"}
```

Si `DATABASE_URL` no está configurada, devuelve:

```json
{"status":"ok","cache":"disabled"}
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
      "status": "ok",
      "source": "cache",
      "fromCache": true
    }
  ]
}
```

## Caché anónima

El backend calcula SHA-256 sobre el contenido del archivo. Si ya existe una clasificación para ese hash, la devuelve sin llamar a Gemini.

No se guardan:

- Archivos originales.
- Imágenes.
- Texto extraído.
- Nombre original del archivo.
- Datos de usuario.

Sí se guardan:

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

La tabla `resource_classifications` se crea automáticamente al iniciar el servidor.

## Variables de entorno

```text
GEMINI_API_KEY=...
FRONTEND_ORIGIN=https://wolcenon.github.io
DATABASE_URL=postgresql://...
PORT=8080
```

`DATABASE_URL` es opcional. Si no existe, el backend sigue funcionando sin caché.

`PORT` lo establece Railway automáticamente.

## Ejecución local

```bash
cd backend
export GEMINI_API_KEY="tu_api_key"
export FRONTEND_ORIGIN="*"
# Opcional:
# export DATABASE_URL="postgresql://usuario:password@localhost:5432/db?sslmode=disable"
go run .
```

Luego prueba:

```bash
curl http://localhost:8080/health
```
