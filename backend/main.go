package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	modelName          = "gemini-2.5-flash"
	maxUploadBytes     = 25 << 20 // 25 MB total request size
	maxFileBytes       = 10 << 20 // 10 MB per file for inline Gemini API requests
	defaultHTTPTimeout = 90 * time.Second
)

type AnalysisResult struct {
	FileName       string `json:"fileName"`
	Edad           string `json:"edad"`
	AreaPrincipal  string `json:"area_principal"`
	Justificacion  string `json:"justificacion"`
	Status         string `json:"status"`
	Source         string `json:"source,omitempty"`
	FromCache      bool   `json:"fromCache"`
	Error          string `json:"error,omitempty"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func main() {
	var closeDB func()
	cacheDB, closeDB = initCacheDB()
	defer closeDB()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/analizar", analizarHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Servidor escuchando en puerto %s", port)
	log.Fatal(server.ListenAndServe())
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cacheStatus := "disabled"
	if cacheDB != nil {
		cacheStatus = "enabled"
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "cache": cacheStatus})
}

func analizarHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Método no permitido"})
		return
	}

	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Falta configurar GEMINI_API_KEY en Railway"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "La subida supera el límite permitido o el formulario no es válido"})
		return
	}

	files := r.MultipartForm.File["recursos"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se recibieron archivos en el campo 'recursos'"})
		return
	}

	client := &http.Client{Timeout: defaultHTTPTimeout}
	results := make([]AnalysisResult, 0, len(files))

	for _, fileHeader := range files {
		result := processOneFile(r.Context(), client, apiKey, fileHeader)
		results = append(results, result)
		if !result.FromCache {
			time.Sleep(800 * time.Millisecond)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func processOneFile(ctx context.Context, client *http.Client, apiKey string, fileHeader *multipart.FileHeader) AnalysisResult {
	result := AnalysisResult{FileName: sanitizeFileName(fileHeader.Filename), Status: "error"}

	if fileHeader.Size <= 0 {
		result.Error = "Archivo vacío"
		return result
	}
	if fileHeader.Size > maxFileBytes {
		result.Error = fmt.Sprintf("Archivo demasiado grande: máximo %d MB por archivo", maxFileBytes>>20)
		return result
	}

	file, err := fileHeader.Open()
	if err != nil {
		result.Error = "No se pudo abrir el archivo"
		return result
	}
	defer file.Close()

	mimeType, content, err := readAndValidateFile(file, fileHeader)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	fileHash := sha256Hex(content)
	if cached, ok := getCachedAnalysis(ctx, fileHash); ok {
		cached.FileName = result.FileName
		cached.Status = "ok"
		cached.Source = "cache"
		cached.FromCache = true
		return cached
	}

	analysis, err := callGemini(ctx, client, apiKey, mimeType, content)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	analysis.FileName = result.FileName
	analysis.Status = "ok"
	analysis.Source = "ia"
	analysis.FromCache = false

	if err := saveCachedAnalysis(ctx, fileHash, fileHeader.Size, mimeType, analysis); err != nil {
		log.Printf("No se pudo guardar la clasificación en caché: %v", err)
	}

	return analysis
}

func readAndValidateFile(file multipart.File, fileHeader *multipart.FileHeader) (string, []byte, error) {
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		return "", nil, errors.New("No se pudo leer el archivo")
	}

	content := buf.Bytes()
	mimeType := http.DetectContentType(content)
	if headerType := fileHeader.Header.Get("Content-Type"); headerType != "" {
		if isAllowedMime(headerType) {
			mimeType = headerType
		}
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == ".pdf" {
		mimeType = "application/pdf"
	}

	if !isAllowedMime(mimeType) {
		return "", nil, fmt.Errorf("Tipo de archivo no permitido: %s", mimeType)
	}

	return mimeType, content, nil
}

func isAllowedMime(mimeType string) bool {
	switch strings.ToLower(strings.Split(mimeType, ";")[0]) {
	case "application/pdf", "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func callGemini(ctx context.Context, client *http.Client, apiKey, mimeType string, content []byte) (AnalysisResult, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", modelName)

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": pedagogicalPrompt()},
					{
						"inline_data": map[string]string{
							"mime_type": mimeType,
							"data":      base64.StdEncoding.EncodeToString(content),
						},
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":      0.1,
			"responseMimeType": "application/json",
			"responseSchema": map[string]any{
				"type":     "OBJECT",
				"required": []string{"edad", "area_principal", "justificacion"},
				"properties": map[string]any{
					"edad": map[string]any{
						"type": "STRING",
						"enum": []string{"0-2 años", "3-5 años", "6-8 años", "9-12 años", "13+ años"},
					},
					"area_principal": map[string]any{
						"type": "STRING",
						"enum": []string{
							"Grafomotricidad y Preescritura",
							"Conciencia Fonológica",
							"Léxico y Vocabulario",
							"Comprensión Lectora",
							"Expresión Escrita y Gramática",
							"Pragmática y Uso del Lenguaje",
						},
					},
					"justificacion": map[string]any{"type": "STRING"},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return AnalysisResult{}, errors.New("No se pudo preparar la petición a Gemini")
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return AnalysisResult{}, errors.New("No se pudo crear la petición a Gemini")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-goog-api-key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return parseGeminiResponse(respBody)
			}

			lastErr = fmt.Errorf("Gemini respondió %d: %s", resp.StatusCode, trimForUser(string(respBody), 240))
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				break
			}
		}

		select {
		case <-ctx.Done():
			return AnalysisResult{}, errors.New("La petición fue cancelada")
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}

	if lastErr == nil {
		lastErr = errors.New("Error desconocido al llamar a Gemini")
	}
	return AnalysisResult{}, lastErr
}

func parseGeminiResponse(respBody []byte) (AnalysisResult, error) {
	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return AnalysisResult{}, errors.New("Gemini devolvió una respuesta no interpretable")
	}
	if geminiResp.Error != nil {
		return AnalysisResult{}, errors.New(geminiResp.Error.Message)
	}
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return AnalysisResult{}, errors.New("Gemini no devolvió contenido")
	}

	text := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	var analysis AnalysisResult
	if err := json.Unmarshal([]byte(text), &analysis); err != nil {
		return AnalysisResult{}, fmt.Errorf("No se pudo parsear el JSON de Gemini: %s", trimForUser(text, 160))
	}
	return analysis, nil
}

func pedagogicalPrompt() string {
	return `Eres un experto en Psicopedagogía, Neuroeducación y Didáctica del Lenguaje con más de 20 años de experiencia en la clasificación de materiales curriculares.

Tu tarea es analizar exhaustivamente el documento adjunto, que puede contener textos narrativos, ejercicios prácticos, pictogramas, ilustraciones visuales o fichas de trazo. Debes examinar tanto el nivel de complejidad textual como el tipo de diseño visual.

Debes extraer y determinar tres puntos específicos bajo criterios estrictamente profesionales:

1. "edad": Clasifica el recurso en uno de los siguientes rangos de edad estándar de desarrollo evolutivo: "0-2 años", "3-5 años", "6-8 años", "9-12 años", "13+ años". Elige el rango basándote en la densidad del texto, el tamaño de la tipografía y la complejidad cognitiva de las instrucciones visuales.
2. "area_principal": Identifica la dimensión lingüística principal que aborda el recurso. Debes elegir obligatoriamente una de estas opciones: "Grafomotricidad y Preescritura", "Conciencia Fonológica", "Léxico y Vocabulario", "Comprensión Lectora", "Expresión Escrita y Gramática", "Pragmática y Uso del Lenguaje".
3. "justificacion": Escribe una sola frase, máximo 25 palabras, explicando el indicador clave del documento que determinó tu decisión.

RESTRICCIÓN ABSOLUTA DE SALIDA: Responde únicamente con un objeto JSON válido con las claves exactas edad, area_principal y justificacion.`
}

func setCORSHeaders(w http.ResponseWriter) {
	origin := strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		return "archivo"
	}
	return name
}

func trimForUser(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
