package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIProviderService maneja las llamadas a diferentes proveedores de IA
type AIProviderService struct {
	client       *http.Client
	configManager *AIConfigManager
}

// NewAIProviderService crea una nueva instancia del servicio
func NewAIProviderService(configManager *AIConfigManager) *AIProviderService {
	return &AIProviderService{
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		configManager: configManager,
	}
}

// ProcessMessage procesa un mensaje usando el proveedor activo
func (s *AIProviderService) ProcessMessage(systemPrompt, userMessage, realPhone string) ([]byte, error) {
	return s.processMessageWithRetry(systemPrompt, userMessage, realPhone, 0, make(map[int]bool))
}

// processMessageWithRetry procesa un mensaje con reintentos automáticos
func (s *AIProviderService) processMessageWithRetry(systemPrompt, userMessage, realPhone string, retryCount int, triedConfigs map[int]bool) ([]byte, error) {
	// Límite de reintentos: 2 rotaciones (3 intentos totales: 1 inicial + 2 rotaciones)
	// Esto evita consumir muchos tokens cuando todas las keys tienen límite diario
	const MAX_RETRIES = 2
	
	if retryCount >= MAX_RETRIES {
		fmt.Printf("⛔ LÍMITE DE REINTENTOS ALCANZADO (%d intentos totales)\n", retryCount+1)
		return nil, fmt.Errorf("alcanzado límite de reintentos (%d rotaciones). Detén el procesamiento y espera a que se restablezcan las quotas", MAX_RETRIES)
	}
	
	// Obtener configuración activa
	config, err := s.configManager.GetActiveConfig()
	if err != nil {
		return nil, fmt.Errorf("no active AI configuration: %v", err)
	}
	
	// Verificar si ya intentamos con esta configuración (evitar bucles)
	if triedConfigs[config.ID] {
		fmt.Printf("⚠️ Configuración %s ya fue intentada, buscando otra...\n", config.Name)
		newConfig, rotateErr := s.configManager.RotateToNextConfig()
		if rotateErr != nil || newConfig.ID == config.ID {
			return nil, fmt.Errorf("no hay más configuraciones disponibles para intentar")
		}
		config = newConfig
	}
	
	// Marcar esta configuración como intentada
	triedConfigs[config.ID] = true
	
	if retryCount > 0 {
		fmt.Printf("🔄 Reintento #%d/%d con: %s - %s (%s)\n", retryCount, MAX_RETRIES, config.ProviderDisplay, config.ModelDisplay, config.Name)
	} else {
		fmt.Printf("🤖 Intento 1/%d - Usando: %s - %s (%s)\n", MAX_RETRIES+1, config.ProviderDisplay, config.ModelDisplay, config.Name)
	}
	
	// Llamar al proveedor correspondiente
	var response []byte
	switch config.ProviderName {
	case "gemini":
		response, err = s.callGemini(config, systemPrompt, userMessage, realPhone)
	case "groq":
		response, err = s.callGroq(config, systemPrompt, userMessage, realPhone)
	case "grok":
		response, err = s.callGrok(config, systemPrompt, userMessage, realPhone)
	case "deepseek":
		response, err = s.callDeepSeek(config, systemPrompt, userMessage, realPhone)
	case "qwen":
		response, err = s.callQwen(config, systemPrompt, userMessage, realPhone)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.ProviderName)
	}
	
	if err != nil {
		// Reportar error
		s.configManager.ReportError(config.ID, err.Error())
		
		// Si es error de rate limit (429, 503), intentar rotar y reintentar
		if isRateLimitError(err) {
			fmt.Printf("⚠️ Rate limit detectado (429/503) en %s - %s (%s)\n", config.ProviderDisplay, config.ModelDisplay, config.Name)
			fmt.Printf("🔄 Intentando rotar a otra configuración disponible...\n")
			
			newConfig, rotateErr := s.configManager.RotateToNextConfig()
			if rotateErr != nil {
				fmt.Printf("❌ No se pudo rotar: %v\n", rotateErr)
				fmt.Printf("❌ FINALIZANDO - No hay más configuraciones para intentar\n")
				return nil, fmt.Errorf("rate limit alcanzado y no hay más configuraciones: %v", err)
			}
			
			if newConfig.ID == config.ID {
				fmt.Printf("❌ No hay otras configuraciones disponibles (solo hay 1 configuración)\n")
				fmt.Printf("❌ FINALIZANDO - Solo hay 1 configuración y tiene rate limit\n")
				return nil, fmt.Errorf("rate limit alcanzado en única configuración disponible: %v", err)
			}
			
			// Verificar si ya intentamos con esta config (evitar bucle)
			if triedConfigs[newConfig.ID] {
				fmt.Printf("❌ Ya se intentó con configuración ID=%d\n", newConfig.ID)
				fmt.Printf("❌ FINALIZANDO - Todas las configuraciones disponibles (%d) ya fueron intentadas\n", len(triedConfigs))
				return nil, fmt.Errorf("todas las configuraciones probadas tienen rate limit de tokens (%d keys intentadas). Espera a que se restablezcan las quotas diarias", len(triedConfigs))
			}
			
			fmt.Printf("✅ Rotado exitosamente a: %s - %s (%s)\n", newConfig.ProviderDisplay, newConfig.ModelDisplay, newConfig.Name)
			fmt.Printf("🔁 Reintentando mensaje con nueva configuración...\n")
			
			// Reintentar con la nueva configuración
			return s.processMessageWithRetry(systemPrompt, userMessage, realPhone, retryCount+1, triedConfigs)
		}
		
		// Si es otro tipo de error, no reintentar
		fmt.Printf("❌ Error no recuperable: %v\n", err)
		return nil, err
	}
	
	// Reportar éxito
	s.configManager.ReportSuccess(config.ID)
	
	if retryCount > 0 {
		fmt.Printf("✅ Reintento exitoso después de %d intentos\n", retryCount)
	} else {
		fmt.Printf("✅ Procesamiento exitoso con %s - %s (%s)\n", config.ProviderDisplay, config.ModelDisplay, config.Name)
	}
	
	return response, nil
}

// callGemini llama a la API de Gemini
func (s *AIProviderService) callGemini(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Construir el prompt completo
	fullPrompt := fmt.Sprintf("%s\n\n## Información del Cliente\n- Teléfono: %s\n\n## Mensaje del Cliente\n%s",
		systemPrompt, realPhone, userMessage)
	
	// Construir request
	request := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": fullPrompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": config.MaxTokens,
			"topP":            0.95,
			"topK":            40,
			"responseMimeType": "application/json",
		},
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// Construir URL
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		config.ModelName, config.APIKey)
	
	fmt.Printf("🤖 [Gemini] Enviando request...\n")
	fmt.Printf("📏 Tamaño del prompt: %d caracteres\n", len(fullPrompt))
	
	startTime := time.Now()
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	elapsed := time.Since(startTime).Seconds()
	fmt.Printf("⏱️ Respuesta recibida en %.2f segundos\n", elapsed)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	fmt.Printf("📥 Recibiendo respuesta de Gemini... (Status: %d)\n", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		// Log detallado del error con el código exacto
		if resp.StatusCode == 429 {
			fmt.Printf("🚨 ERROR 429 DETECTADO - RATE LIMIT EXCEDIDO\n")
			fmt.Printf("📄 Respuesta completa: %s\n", string(body))
		}
		
		return nil, fmt.Errorf("gemini API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Parsear respuesta
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %v", err)
	}
	
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}
	
	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseText))
	
	return []byte(responseText), nil
}

// callGroq llama a la API de Groq
func (s *AIProviderService) callGroq(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Groq usa formato compatible con OpenAI
	request := map[string]interface{}{
		"model": config.ModelName,
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": fmt.Sprintf("%s\n\nIMPORTANTE: Debes responder con un array JSON. Si hay UNA carga, responde [{...carga...}]. Si hay MÚLTIPLES cargas, responde [{...carga1...}, {...carga2...}].\nEl formato debe ser SIEMPRE un array, nunca un objeto suelto.", systemPrompt),
			},
			{
				"role": "user",
				"content": fmt.Sprintf("Teléfono del cliente: %s\n\n%s", realPhone, userMessage),
			},
		},
		"temperature": 0.7,
		"max_tokens": config.MaxTokens,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	url := "https://api.groq.com/openai/v1/chat/completions"
	
	fmt.Printf("🤖 [Groq] Enviando request...\n")
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	
	startTime := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	elapsed := time.Since(startTime).Seconds()
	fmt.Printf("⏱️ Respuesta recibida en %.2f segundos\n", elapsed)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	fmt.Printf("📥 Recibiendo respuesta de Groq... (Status: %d)\n", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			fmt.Printf("🚨 ERROR 429 DETECTADO EN GROQ - RATE LIMIT EXCEDIDO\n")
			fmt.Printf("📄 Respuesta completa: %s\n", string(body))
		}
		
		return nil, fmt.Errorf("groq API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Parsear respuesta (formato OpenAI)
	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.Unmarshal(body, &groqResp); err != nil {
		return nil, fmt.Errorf("failed to parse Groq response: %v", err)
	}
	
	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from Groq")
	}
	
	responseText := groqResp.Choices[0].Message.Content
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseText))
	
	return []byte(responseText), nil
}

// callDeepSeek llama a la API de DeepSeek
func (s *AIProviderService) callDeepSeek(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// DeepSeek usa formato compatible con OpenAI
	request := map[string]interface{}{
		"model": config.ModelName,
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": fmt.Sprintf("%s\n\nIMPORTANTE: Debes responder con un array JSON. Si hay UNA carga, responde [{...carga...}]. Si hay MÚLTIPLES cargas, responde [{...carga1...}, {...carga2...}].\nEl formato debe ser SIEMPRE un array, nunca un objeto suelto.", systemPrompt),
			},
			{
				"role": "user",
				"content": fmt.Sprintf("Teléfono del cliente: %s\n\n%s", realPhone, userMessage),
			},
		},
		"temperature": 0.7,
		"max_tokens": config.MaxTokens,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	url := "https://api.deepseek.com/v1/chat/completions"
	
	fmt.Printf("🤖 [DeepSeek] Enviando request...\n")
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	
	startTime := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	elapsed := time.Since(startTime).Seconds()
	fmt.Printf("⏱️ Respuesta recibida en %.2f segundos\n", elapsed)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	fmt.Printf("📥 Recibiendo respuesta de DeepSeek... (Status: %d)\n", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			fmt.Printf("🚨 ERROR 429 DETECTADO EN DEEPSEEK - RATE LIMIT EXCEDIDO\n")
			fmt.Printf("📄 Respuesta completa: %s\n", string(body))
		} else if resp.StatusCode == 402 {
			fmt.Printf("💳 ERROR 402 DETECTADO EN DEEPSEEK - SALDO INSUFICIENTE\n")
			fmt.Printf("📄 Respuesta: %s\n", string(body))
			fmt.Printf("💡 SOLUCIÓN: Recarga tu cuenta en https://platform.deepseek.com\n")
		}
		
		return nil, fmt.Errorf("deepseek API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Parsear respuesta (formato OpenAI)
	var deepseekResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.Unmarshal(body, &deepseekResp); err != nil {
		return nil, fmt.Errorf("failed to parse DeepSeek response: %v", err)
	}
	
	if len(deepseekResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from DeepSeek")
	}
	
	responseText := deepseekResp.Choices[0].Message.Content
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseText))
	
	return []byte(responseText), nil
}

// callGrok llama a la API de Grok (xAI)
func (s *AIProviderService) callGrok(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Grok usa formato compatible con OpenAI
	request := map[string]interface{}{
		"model": config.ModelName,
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": fmt.Sprintf("%s\n\nIMPORTANTE: Debes responder ÚNICAMENTE con un array JSON válido de cargas.", systemPrompt),
			},
			{
				"role": "user",
				"content": fmt.Sprintf("Teléfono del cliente: %s\n\n%s", realPhone, userMessage),
			},
		},
		"temperature": 0.7,
		"max_tokens": config.MaxTokens,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	url := "https://api.x.ai/v1/chat/completions"
	
	fmt.Printf("🤖 [Grok] Enviando request...\n")
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	
	startTime := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	elapsed := time.Since(startTime).Seconds()
	fmt.Printf("⏱️ Respuesta recibida en %.2f segundos\n", elapsed)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	fmt.Printf("📥 Recibiendo respuesta de Grok... (Status: %d)\n", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			fmt.Printf("🚨 ERROR 429 DETECTADO EN GROK - RATE LIMIT EXCEDIDO\n")
			fmt.Printf("📄 Respuesta completa: %s\n", string(body))
		}
		
		return nil, fmt.Errorf("grok API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Parsear respuesta (formato OpenAI)
	var grokResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	
	if err := json.Unmarshal(body, &grokResp); err != nil {
		return nil, fmt.Errorf("failed to parse Grok response: %v", err)
	}
	
	if len(grokResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from Grok")
	}
	
	responseText := grokResp.Choices[0].Message.Content
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseText))
	
	return []byte(responseText), nil
}

// callQwen llama a la API de Qwen (Alibaba)
func (s *AIProviderService) callQwen(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Qwen usa formato similar a OpenAI
	request := map[string]interface{}{
		"model": config.ModelName,
		"input": map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "system",
					"content": fmt.Sprintf("%s\n\nIMPORTANTE: Responde ÚNICAMENTE con JSON válido.", systemPrompt),
				},
				{
					"role": "user",
					"content": fmt.Sprintf("Teléfono: %s\n\n%s", realPhone, userMessage),
				},
			},
		},
		"parameters": map[string]interface{}{
			"result_format": "message",
			"temperature": 0.7,
			"max_tokens": config.MaxTokens,
		},
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
	
	fmt.Printf("🤖 [Qwen] Enviando request...\n")
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	
	startTime := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	elapsed := time.Since(startTime).Seconds()
	fmt.Printf("⏱️ Respuesta recibida en %.2f segundos\n", elapsed)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	fmt.Printf("📥 Recibiendo respuesta de Qwen... (Status: %d)\n", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			fmt.Printf("🚨 ERROR 429 DETECTADO EN QWEN - RATE LIMIT EXCEDIDO\n")
			fmt.Printf("📄 Respuesta completa: %s\n", string(body))
		}
		
		return nil, fmt.Errorf("qwen API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Parsear respuesta de Qwen
	var qwenResp struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen response: %v", err)
	}
	
	if len(qwenResp.Output.Choices) == 0 {
		return nil, fmt.Errorf("empty response from Qwen")
	}
	
	responseText := qwenResp.Output.Choices[0].Message.Content
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseText))
	
	return []byte(responseText), nil
}

// isRateLimitError verifica si un error es por rate limiting
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Detectar diferentes variantes de errores de rate limit
	rateLimitIndicators := []string{
		"429",                          // HTTP 429 Too Many Requests
		"503",                          // HTTP 503 Service Unavailable
		"rate limit",                   // Mensaje explícito
		"rate_limit",                   // Variante con guion bajo
		"quota",                        // Quota excedida
		"too many requests",            // Mensaje HTTP estándar
		"resource exhausted",           // gRPC error
		"resource_exhausted",           // Variante
		"request limit",                // Límite de requests
		"over_query_limit",            // Google Maps específico
		"insufficient_quota",          // Cuota insuficiente
		"exceeded",                     // Excedido (quota exceeded, limit exceeded)
	}
	
	for _, indicator := range rateLimitIndicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}
	
	return false
}

// ValidateResponse valida que la respuesta sea JSON válido
func (s *AIProviderService) ValidateResponse(response []byte) error {
	var data interface{}
	if err := json.Unmarshal(response, &data); err != nil {
		return fmt.Errorf("invalid JSON response: %v", err)
	}
	
	// Verificar que sea un array o un objeto
	switch data.(type) {
	case []interface{}:
		// Es un array, perfecto
		return nil
	case map[string]interface{}:
		// Es un objeto único, también es válido
		return nil
	default:
		return fmt.Errorf("response must be a JSON object or array")
	}
}

// NormalizeResponse convierte respuestas de objeto único en array
func (s *AIProviderService) NormalizeResponse(response []byte) ([]byte, error) {
	var data interface{}
	if err := json.Unmarshal(response, &data); err != nil {
		return response, err
	}
	
	// Si ya es un array, devolverlo tal cual
	if _, ok := data.([]interface{}); ok {
		return response, nil
	}
	
	// Si es un objeto, convertirlo en array
	if obj, ok := data.(map[string]interface{}); ok {
		arrayData := []interface{}{obj}
		normalized, err := json.Marshal(arrayData)
		if err != nil {
			return response, err
		}
		fmt.Printf("🔄 Respuesta normalizada: objeto convertido a array\n")
		return normalized, nil
	}
	
	return response, nil
}

