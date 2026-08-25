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

// ProcessMessage procesa un mensaje usando el proveedor activo (sin reintentos automáticos)
func (s *AIProviderService) ProcessMessage(systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Obtener configuración activa desde caché (optimizado para concurrencia)
	config, err := s.configManager.GetActiveConfig()
	if err != nil {
		return nil, fmt.Errorf("no active AI configuration: %v", err)
	}
	
	if config == nil {
		return nil, fmt.Errorf("no hay configuración de IA activa. Ve a '⚙️ Configuración IA' y activa una")
	}
	
	return s.ProcessMessageWithConfig(config, systemPrompt, userMessage, realPhone)
}

// ProcessMessageWithConfig procesa un mensaje usando una configuración específica
func (s *AIProviderService) ProcessMessageWithConfig(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	fmt.Printf("🤖 Usando: %s - %s (%s)\n", config.ProviderDisplay, config.ModelDisplay, config.Name)
	
	// Llamar al proveedor correspondiente
	var response []byte
	var err error
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
		
		// Si es error de rate limit (429, 503), detener y pedir cambio manual
		if isRateLimitError(err) {
			fmt.Printf("⚠️ RATE LIMIT DETECTADO (429/503) en %s - %s (%s)\n", config.ProviderDisplay, config.ModelDisplay, config.Name)
			fmt.Printf("🛑 PROCESAMIENTO DETENIDO - Rate limit alcanzado\n")
			fmt.Printf("💡 SOLUCIÓN: Ve a '⚙️ Configuración IA' y activa otra API key manualmente\n")
			fmt.Printf("📋 Sugerencia: Activa una key de otro proveedor (Groq, Gemini, etc.)\n")
			
			return nil, fmt.Errorf("rate limit alcanzado en %s - %s (%s). Ve a Configuración IA y activa otra key manualmente", 
				config.ProviderDisplay, config.ModelDisplay, config.Name)
		}
		
		// Si es otro tipo de error, no reintentar
		fmt.Printf("❌ Error no recuperable: %v\n", err)
		return nil, err
	}
	
	// Reportar éxito
	s.configManager.ReportSuccess(config.ID)
	fmt.Printf("✅ Procesamiento exitoso con %s - %s (%s)\n", config.ProviderDisplay, config.ModelDisplay, config.Name)
	
	return response, nil
}

// callGemini llama a la API de Gemini
func (s *AIProviderService) callGemini(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Obtener fecha actual en zona horaria argentina (UTC-3)
	argLocation, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	currentDate := time.Now().In(argLocation).Format("02/01/2006") // DD/MM/YYYY
	currentDateTime := time.Now().In(argLocation).Format("02/01/2006 15:04") // DD/MM/YYYY HH:MM
	
	// Construir el prompt completo con fecha actual
	fullPrompt := fmt.Sprintf("%s\n\n## FECHA Y HORA ACTUAL (Argentina)\n- Hoy es: %s\n- Fecha y hora actual: %s\n- Zona horaria: Argentina (UTC-3)\n- IMPORTANTE: Usa esta fecha como referencia para \"hoy\", \"mañana\", etc.\n\n## Información del Cliente\n- Teléfono: %s\n\n## Mensaje del Cliente\n%s",
		systemPrompt, currentDate, currentDateTime, realPhone, userMessage)
	
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
		normalizeGeminiModelName(config.ModelName), config.APIKey)
	
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
	// Obtener fecha actual en zona horaria argentina (UTC-3)
	argLocation, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	currentDate := time.Now().In(argLocation).Format("02/01/2006") // DD/MM/YYYY
	currentDateTime := time.Now().In(argLocation).Format("02/01/2006 15:04") // DD/MM/YYYY HH:MM
	
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
				"content": fmt.Sprintf("FECHA Y HORA ACTUAL (Argentina):\n- Hoy es: %s\n- Fecha y hora actual: %s\n- Zona horaria: Argentina (UTC-3)\n- IMPORTANTE: Usa esta fecha como referencia para \"hoy\", \"mañana\", etc.\n\nTeléfono del cliente: %s\n\n%s", currentDate, currentDateTime, realPhone, userMessage),
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

// calcularMaxTokensDeepSeek calcula max_tokens dinámicamente según la longitud del mensaje
func calcularMaxTokensDeepSeek(mensaje string, baseTokens int) int {
	// Estimar tokens: ~4 caracteres por token
	caracteresMensaje := len(mensaje)
	
	// Para mensajes largos, aumentar tokens de salida
	// Si el mensaje tiene más de 1500 caracteres, aumentar tokens proporcionalmente
	if caracteresMensaje > 1500 {
		// Aumentar tokens: base + (longitud_mensaje / 8)
		// Esto da más espacio para respuestas largas
		tokensAdicionales := caracteresMensaje / 8
		nuevoMaxTokens := baseTokens + tokensAdicionales
		
		// Límite máximo para DeepSeek: 16384 tokens de salida
		if nuevoMaxTokens > 16384 {
			return 16384
		}
		// Mínimo: asegurar al menos 4096 para mensajes largos
		if nuevoMaxTokens < 4096 && caracteresMensaje > 2000 {
			return 4096
		}
		return nuevoMaxTokens
	}
	
	return baseTokens
}

// esJSONIncompleto detecta si un JSON está incompleto (truncado)
func esJSONIncompleto(jsonStr string) bool {
	jsonStr = strings.TrimSpace(jsonStr)
	
	// Si está vacío, no es incompleto (es vacío)
	if jsonStr == "" {
		return false
	}
	
	// Si empieza con [ pero no termina con ], está incompleto
	if strings.HasPrefix(jsonStr, "[") && !strings.HasSuffix(jsonStr, "]") {
		return true
	}
	
	// Si empieza con { pero no termina con }, está incompleto
	if strings.HasPrefix(jsonStr, "{") && !strings.HasSuffix(jsonStr, "}") {
		return true
	}
	
	// Contar llaves y corchetes para verificar balance
	abrirCorchetes := strings.Count(jsonStr, "[")
	cerrarCorchetes := strings.Count(jsonStr, "]")
	abrirLlaves := strings.Count(jsonStr, "{")
	cerrarLlaves := strings.Count(jsonStr, "}")
	
	if abrirCorchetes != cerrarCorchetes || abrirLlaves != cerrarLlaves {
		return true
	}
	
	return false
}

// callDeepSeek llama a la API de DeepSeek
func (s *AIProviderService) callDeepSeek(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Obtener fecha actual en zona horaria argentina (UTC-3)
	argLocation, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	currentDate := time.Now().In(argLocation).Format("02/01/2006") // DD/MM/YYYY
	currentDateTime := time.Now().In(argLocation).Format("02/01/2006 15:04") // DD/MM/YYYY HH:MM
	
	// Calcular max_tokens dinámicamente para mensajes largos
	maxTokens := calcularMaxTokensDeepSeek(userMessage, config.MaxTokens)
	fmt.Printf("📊 [DeepSeek] Max tokens calculado: %d (base: %d, mensaje: %d caracteres)\n", 
		maxTokens, config.MaxTokens, len(userMessage))
	
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
				"content": fmt.Sprintf("FECHA Y HORA ACTUAL (Argentina):\n- Hoy es: %s\n- Fecha y hora actual: %s\n- Zona horaria: Argentina (UTC-3)\n- IMPORTANTE: Usa esta fecha como referencia para \"hoy\", \"mañana\", etc.\n\nTeléfono del cliente: %s\n\n%s", currentDate, currentDateTime, realPhone, userMessage),
			},
		},
		"temperature": 0.7,
		"max_tokens": maxTokens, // Usar el valor calculado dinámicamente
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
			FinishReason string `json:"finish_reason"` // "stop", "length", "content_filter"
		} `json:"choices"`
	}
	
	if err := json.Unmarshal(body, &deepseekResp); err != nil {
		return nil, fmt.Errorf("failed to parse DeepSeek response: %v", err)
	}
	
	if len(deepseekResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from DeepSeek")
	}
	
	responseText := deepseekResp.Choices[0].Message.Content
	finishReason := deepseekResp.Choices[0].FinishReason
	
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseText))
	fmt.Printf("🏁 Finish reason: %s\n", finishReason)
	
	// Detectar si está truncado
	esTruncado := finishReason == "length" || esJSONIncompleto(responseText)
	
	if esTruncado {
		fmt.Printf("⚠️ [DeepSeek] Respuesta truncada detectada (finish_reason: %s)\n", finishReason)
		
		// Si aún no hemos aumentado mucho los tokens, reintentar con más
		if maxTokens < 16384 {
			nuevoMaxTokens := maxTokens * 2
			if nuevoMaxTokens > 16384 {
				nuevoMaxTokens = 16384
			}
			fmt.Printf("🔄 [DeepSeek] Reintentando con %d tokens de salida...\n", nuevoMaxTokens)
			
			// Actualizar maxTokens en el request
			request["max_tokens"] = nuevoMaxTokens
			
			// Reintentar la llamada
			jsonDataRetry, err := json.Marshal(request)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal retry request: %v", err)
			}
			
			reqRetry, err := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(jsonDataRetry))
			if err != nil {
				return nil, fmt.Errorf("failed to create retry request: %v", err)
			}
			
			reqRetry.Header.Set("Content-Type", "application/json")
			reqRetry.Header.Set("Authorization", "Bearer "+config.APIKey)
			
			startTimeRetry := time.Now()
			respRetry, err := s.client.Do(reqRetry)
			if err != nil {
				return nil, fmt.Errorf("failed to retry request: %v", err)
			}
			defer respRetry.Body.Close()
			
			elapsedRetry := time.Since(startTimeRetry).Seconds()
			fmt.Printf("⏱️ Respuesta del reintento recibida en %.2f segundos\n", elapsedRetry)
			
			bodyRetry, err := io.ReadAll(respRetry.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read retry response: %v", err)
			}
			
			if respRetry.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("deepseek API error on retry %d: %s", respRetry.StatusCode, string(bodyRetry))
			}
			
			// Parsear respuesta del reintento
			if err := json.Unmarshal(bodyRetry, &deepseekResp); err != nil {
				return nil, fmt.Errorf("failed to parse DeepSeek retry response: %v", err)
			}
			
			if len(deepseekResp.Choices) == 0 {
				return nil, fmt.Errorf("empty response from DeepSeek retry")
			}
			
			responseText = deepseekResp.Choices[0].Message.Content
			finishReason = deepseekResp.Choices[0].FinishReason
			
			fmt.Printf("📦 Respuesta del reintento: %d bytes, finish_reason: %s\n", len(responseText), finishReason)
			
			// Si aún está truncado después del reintento, devolver error
			if finishReason == "length" || esJSONIncompleto(responseText) {
				return nil, fmt.Errorf("respuesta aún truncada después de aumentar tokens. Considera dividir el mensaje o usar un modelo con mayor capacidad")
			}
		} else {
			return nil, fmt.Errorf("respuesta truncada y ya se usó el máximo de tokens permitido (16384)")
		}
	}
	
	return []byte(responseText), nil
}

// callGrok llama a la API de Grok (xAI)
func (s *AIProviderService) callGrok(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Obtener fecha actual en zona horaria argentina (UTC-3)
	argLocation, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	currentDate := time.Now().In(argLocation).Format("02/01/2006") // DD/MM/YYYY
	currentDateTime := time.Now().In(argLocation).Format("02/01/2006 15:04") // DD/MM/YYYY HH:MM
	
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
				"content": fmt.Sprintf("FECHA Y HORA ACTUAL (Argentina):\n- Hoy es: %s\n- Fecha y hora actual: %s\n- Zona horaria: Argentina (UTC-3)\n- IMPORTANTE: Usa esta fecha como referencia para \"hoy\", \"mañana\", etc.\n\nTeléfono del cliente: %s\n\n%s", currentDate, currentDateTime, realPhone, userMessage),
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
	// Obtener fecha actual en zona horaria argentina (UTC-3)
	argLocation, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	currentDate := time.Now().In(argLocation).Format("02/01/2006") // DD/MM/YYYY
	currentDateTime := time.Now().In(argLocation).Format("02/01/2006 15:04") // DD/MM/YYYY HH:MM
	
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
					"content": fmt.Sprintf("FECHA Y HORA ACTUAL (Argentina):\n- Hoy es: %s\n- Fecha y hora actual: %s\n- Zona horaria: Argentina (UTC-3)\n- IMPORTANTE: Usa esta fecha como referencia para \"hoy\", \"mañana\", etc.\n\nTeléfono: %s\n\n%s", currentDate, currentDateTime, realPhone, userMessage),
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

// CleanMarkdownCodeBlocks limpia bloques de código markdown de la respuesta
// Algunos modelos (DeepSeek, Gemini) devuelven JSON envuelto en ```json ... ```
func (s *AIProviderService) CleanMarkdownCodeBlocks(response []byte) []byte {
	text := string(response)
	
	// Eliminar bloques de código markdown: ```json ... ``` o ``` ... ```
	text = strings.TrimSpace(text)
	
	// Patrón 1: ```json\n{...}\n```
	if strings.HasPrefix(text, "```json") && strings.HasSuffix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
		fmt.Printf("🧹 Limpiando markdown: ```json detectado\n")
	}
	
	// Patrón 2: ```\n{...}\n```
	if strings.HasPrefix(text, "```") && strings.HasSuffix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
		fmt.Printf("🧹 Limpiando markdown: ``` detectado\n")
	}
	
	// Patrón 3: Remover lenguaje específico después de ```
	// Ejemplo: ```javascript, ```typescript, etc.
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "```") {
		lines = lines[1:]
		text = strings.Join(lines, "\n")
		fmt.Printf("🧹 Limpiando markdown: primera línea con ``` detectada\n")
	}
	
	return []byte(strings.TrimSpace(text))
}

// ValidateResponse valida que la respuesta sea JSON válido
func (s *AIProviderService) ValidateResponse(response []byte) error {
	// Limpiar bloques de código markdown antes de parsear
	cleanResponse := s.CleanMarkdownCodeBlocks(response)
	
	var data interface{}
	if err := json.Unmarshal(cleanResponse, &data); err != nil {
		// Mostrar los primeros 200 caracteres para debug
		preview := string(cleanResponse)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("❌ Error parseando JSON. Primeros caracteres: %s\n", preview)
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
	// Limpiar bloques de código markdown antes de parsear
	cleanResponse := s.CleanMarkdownCodeBlocks(response)
	
	var data interface{}
	if err := json.Unmarshal(cleanResponse, &data); err != nil {
		return cleanResponse, err
	}
	
	// Si ya es un array, devolverlo tal cual
	if _, ok := data.([]interface{}); ok {
		return cleanResponse, nil
	}
	
	// Si es un objeto, convertirlo en array
	if obj, ok := data.(map[string]interface{}); ok {
		arrayData := []interface{}{obj}
		normalized, err := json.Marshal(arrayData)
		if err != nil {
			return cleanResponse, err
		}
		fmt.Printf("🔄 Respuesta normalizada: objeto convertido a array\n")
		return normalized, nil
	}
	
	return cleanResponse, nil
}

// ChatTurn representa un turno en una conversación multi-turno
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatWithHistory procesa una conversación multi-turno y devuelve texto libre
func (s *AIProviderService) ChatWithHistory(config *AIConfigDB, systemPrompt string, history []ChatTurn) (string, error) {
	if config == nil {
		return "", fmt.Errorf("no AI configuration provided")
	}

	fmt.Printf("💬 [DevAgent] Usando: %s - %s (%s)\n", config.ProviderDisplay, config.ModelDisplay, config.Name)

	var response string
	var err error
	switch config.ProviderName {
	case "gemini":
		response, err = s.chatGemini(config, systemPrompt, history)
	case "groq":
		response, err = s.chatOpenAICompatible(config, systemPrompt, history, "https://api.groq.com/openai/v1/chat/completions", "Groq", false)
	case "grok":
		response, err = s.chatOpenAICompatible(config, systemPrompt, history, "https://api.x.ai/v1/chat/completions", "Grok", false)
	case "deepseek":
		response, err = s.chatOpenAICompatible(config, systemPrompt, history, "https://api.deepseek.com/v1/chat/completions", "DeepSeek", false)
	case "qwen":
		response, err = s.chatQwen(config, systemPrompt, history)
	default:
		return "", fmt.Errorf("unsupported provider for chat: %s", config.ProviderName)
	}

	if err != nil {
		s.configManager.ReportError(config.ID, err.Error())
		return "", err
	}

	s.configManager.ReportSuccess(config.ID)
	return response, nil
}

func (s *AIProviderService) chatOpenAICompatible(config *AIConfigDB, systemPrompt string, history []ChatTurn, url, label string, jsonMode bool) (string, error) {
	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}
	for _, turn := range history {
		messages = append(messages, map[string]interface{}{
			"role":    turn.Role,
			"content": turn.Content,
		})
	}

	request := map[string]interface{}{
		"model":       config.ModelName,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  config.MaxTokens,
	}
	if jsonMode {
		request["response_format"] = map[string]string{"type": "json_object"}
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	fmt.Printf("💬 [%s] Enviando chat con %d turnos...\n", label, len(history))

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s API error %d: %s", strings.ToLower(label), resp.StatusCode, string(body))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse %s response: %v", label, err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from %s", label)
	}
	return chatResp.Choices[0].Message.Content, nil
}

func (s *AIProviderService) chatGemini(config *AIConfigDB, systemPrompt string, history []ChatTurn) (string, error) {
	contents := []map[string]interface{}{}
	for _, turn := range history {
		role := "user"
		if turn.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{"text": turn.Content},
			},
		})
	}

	request := map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": systemPrompt},
			},
		},
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": config.MaxTokens,
			"topP":            0.95,
			"topK":            40,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		normalizeGeminiModelName(config.ModelName), config.APIKey)

	fmt.Printf("💬 [Gemini] Enviando chat con %d turnos...\n", len(history))

	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API error %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse Gemini response: %v", err)
	}
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}
	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (s *AIProviderService) chatQwen(config *AIConfigDB, systemPrompt string, history []ChatTurn) (string, error) {
	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}
	for _, turn := range history {
		messages = append(messages, map[string]interface{}{
			"role":    turn.Role,
			"content": turn.Content,
		})
	}

	request := map[string]interface{}{
		"model": config.ModelName,
		"input": map[string]interface{}{
			"messages": messages,
		},
		"parameters": map[string]interface{}{
			"result_format": "message",
			"temperature":   0.7,
			"max_tokens":    config.MaxTokens,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	fmt.Printf("💬 [Qwen] Enviando chat con %d turnos...\n", len(history))

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qwen API error %d: %s", resp.StatusCode, string(body))
	}

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
		return "", fmt.Errorf("failed to parse Qwen response: %v", err)
	}
	if len(qwenResp.Output.Choices) == 0 {
		return "", fmt.Errorf("empty response from Qwen")
	}
	return qwenResp.Output.Choices[0].Message.Content, nil
}

