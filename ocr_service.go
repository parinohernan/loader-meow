package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const ocrExtractionPrompt = "Extrae todo el texto legible de esta imagen. Responde únicamente con el texto extraído, sin explicaciones ni formato adicional. Si no hay texto visible, responde con una cadena vacía."

// createMinimalTestPNG crea una imagen PNG mínima para pruebas de OCR
func createMinimalTestPNG(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	if err != nil {
		return err
	}
	return os.WriteFile(path, pngData, 0644)
}

// OCRService maneja la extracción de texto desde imágenes
type OCRService struct {
	client        *http.Client
	configManager *OCRConfigManager
}

// NewOCRService crea una nueva instancia del servicio OCR
func NewOCRService(configManager *OCRConfigManager) *OCRService {
	return &OCRService{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		configManager: configManager,
	}
}

// ExtractTextFromImage extrae texto de una imagen usando la configuración OCR activa
func (s *OCRService) ExtractTextFromImage(config *OCRConfigDB, imagePath string) (string, error) {
	if config == nil {
		return "", fmt.Errorf("no OCR configuration provided")
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %v", err)
	}

	mimeType := detectImageMIME(imagePath)

	var text string
	switch config.ProviderName {
	case "gemini":
		text, err = s.callGeminiVision(config, imageData, mimeType)
	case "ocrspace":
		text, err = s.callOCRSpace(config, imageData, mimeType)
	default:
		return "", fmt.Errorf("unsupported OCR provider: %s", config.ProviderName)
	}

	if err != nil {
		s.configManager.ReportError(config.ID, err.Error())
		return "", err
	}

	s.configManager.ReportSuccess(config.ID)
	return strings.TrimSpace(text), nil
}

// ExtractTextFromImageActive usa la configuración OCR activa
func (s *OCRService) ExtractTextFromImageActive(imagePath string) (string, error) {
	config, err := s.configManager.GetActiveConfig()
	if err != nil {
		return "", fmt.Errorf("no hay configuración OCR activa. Ve a 'Configuración OCR / Visión' y activa una")
	}

	return s.ExtractTextFromImage(config, imagePath)
}

func detectImageMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func (s *OCRService) callGeminiVision(config *OCRConfigDB, imageData []byte, mimeType string) (string, error) {
	request := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": ocrExtractionPrompt},
					{
						"inline_data": map[string]interface{}{
							"mime_type": mimeType,
							"data":      base64.StdEncoding.EncodeToString(imageData),
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"maxOutputTokens": config.MaxTokens,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OCR request: %v", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		normalizeGeminiModelName(config.ModelName), config.APIKey)

	const maxAttempts = 2
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create OCR request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("OCR request failed: %v", err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("failed to read OCR response: %v", readErr)
		}

		if resp.StatusCode == http.StatusOK {
			var geminiResp map[string]interface{}
			if err := json.Unmarshal(body, &geminiResp); err != nil {
				return "", fmt.Errorf("failed to parse OCR response: %v", err)
			}
			return extractGeminiText(geminiResp), nil
		}

		lastErr = fmt.Errorf("OCR API error (status %d): %s", resp.StatusCode, string(body))

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts-1 {
			delay := parseGeminiRetryDelay(body)
			fmt.Printf("⏳ OCR rate limit (429) - reintentando en %v...\n", delay)
			time.Sleep(delay)
			continue
		}

		return "", lastErr
	}

	return "", lastErr
}

func parseGeminiRetryDelay(body []byte) time.Duration {
	const defaultDelay = 45 * time.Second

	var payload struct {
		Error struct {
			Message string `json:"message"`
			Details []struct {
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, detail := range payload.Error.Details {
			if detail.RetryDelay != "" {
				if d, err := time.ParseDuration(detail.RetryDelay); err == nil && d > 0 {
					return d + 2*time.Second
				}
			}
		}
		if payload.Error.Message != "" {
			re := regexp.MustCompile(`retry in ([0-9.]+)s`)
			if match := re.FindStringSubmatch(payload.Error.Message); len(match) == 2 {
				if seconds, err := time.ParseDuration(match[1] + "s"); err == nil && seconds > 0 {
					return seconds + 2*time.Second
				}
			}
		}
	}

	return defaultDelay
}

func extractGeminiText(response map[string]interface{}) string {
	candidates, ok := response["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return ""
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return ""
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return ""
	}

	parts, ok := content["parts"].([]interface{})
	if !ok {
		return ""
	}

	var textParts []string
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := partMap["text"].(string); ok {
			textParts = append(textParts, text)
		}
	}

	return strings.Join(textParts, "\n")
}

func (s *OCRService) callOCRSpace(config *OCRConfigDB, imageData []byte, mimeType string) (string, error) {
	filetype := ocrSpaceFileType(mimeType)
	engine := ocrSpaceEngine(config.ModelName)
	base64Image := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imageData))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"base64Image":       base64Image,
		"language":          "spa",
		"isOverlayRequired": "false",
		"detectOrientation": "true",
		"scale":             "true",
		"OCREngine":         strconv.Itoa(engine),
		"filetype":          filetype,
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return "", fmt.Errorf("failed to build OCR.space request: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize OCR.space request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.ocr.space/parse/image", &body)
	if err != nil {
		return "", fmt.Errorf("failed to create OCR.space request: %v", err)
	}
	req.Header.Set("apikey", config.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR.space request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OCR.space response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OCR.space API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ocrResp struct {
		ParsedResults []struct {
			ParsedText string `json:"ParsedText"`
		} `json:"ParsedResults"`
		OCRExitCode           int         `json:"OCRExitCode"`
		IsErroredOnProcessing bool        `json:"IsErroredOnProcessing"`
		ErrorMessage          interface{} `json:"ErrorMessage"`
	}
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return "", fmt.Errorf("failed to parse OCR.space response: %v", err)
	}

	if ocrResp.IsErroredOnProcessing || ocrResp.OCRExitCode != 1 {
		return "", fmt.Errorf("OCR.space processing error: %s", formatOCRSpaceError(ocrResp.ErrorMessage))
	}

	var parts []string
	for _, result := range ocrResp.ParsedResults {
		if text := strings.TrimSpace(result.ParsedText); text != "" {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "\n"), nil
}

func ocrSpaceFileType(mimeType string) string {
	switch mimeType {
	case "image/png":
		return "PNG"
	case "image/gif":
		return "GIF"
	case "image/webp":
		return "WEBP"
	case "image/tiff":
		return "TIFF"
	default:
		return "JPG"
	}
}

func ocrSpaceEngine(modelName string) int {
	switch strings.ToLower(modelName) {
	case "engine-1", "1":
		return 1
	case "engine-3", "3":
		return 3
	default:
		return 2
	}
}

func formatOCRSpaceError(errMsg interface{}) string {
	switch v := errMsg.(type) {
	case string:
		if v != "" {
			return v
		}
	case []interface{}:
		var parts []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	return "unknown OCR.space error"
}
