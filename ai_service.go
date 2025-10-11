package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AIService maneja la integración con Gemini API
type AIService struct {
	config       *AIConfig
	systemPrompt string
	client       *http.Client
}

// GeminiRequest estructura para la API de Gemini
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig GeminiGenerationConfig `json:"generationConfig"`
}

// GeminiContent representa el contenido de la solicitud
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart representa una parte del contenido
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig configuración de generación
type GeminiGenerationConfig struct {
	Temperature      float32 `json:"temperature"`
	MaxOutputTokens  int     `json:"maxOutputTokens"`
	TopP             float32 `json:"topP"`
	TopK             int     `json:"topK"`
}

// GeminiResponse estructura de respuesta de Gemini
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

// GeminiCandidate representa una respuesta candidata
type GeminiCandidate struct {
	Content GeminiContent `json:"content"`
}

// NewAIService crea una nueva instancia del servicio de IA
func NewAIService(keysManager *APIKeysManager) (*AIService, error) {
	config := GetAIConfig(keysManager)
	
	if !config.IsConfigured() {
		return nil, fmt.Errorf("AI configuration is incomplete. Please configure API keys in the 'Procesamiento IA' tab")
	}
	
	// Cargar prompt del archivo
	systemPrompt, err := loadSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %v", err)
	}
	
	return &AIService{
		config:       config,
		systemPrompt: systemPrompt,
		client: &http.Client{
			Timeout: 120 * time.Second, // Aumentado a 2 minutos para mensajes largos
		},
	}, nil
}

// loadSystemPrompt carga el prompt desde el archivo de contexto
func loadSystemPrompt() (string, error) {
	// Intentar leer desde el archivo de contexto
	promptBytes, err := os.ReadFile("contecto_funcionalidad_ia.md")
	if err != nil {
		// Si no se puede leer el archivo, usar un prompt básico
		return getBasicPrompt(), nil
	}
	return string(promptBytes), nil
}

// getBasicPrompt retorna un prompt básico si no se puede cargar el archivo
func getBasicPrompt() string {
	return "# CONTEXTO PARA IA: GENERACIÓN DE CARGAS DE TRANSPORTE\n\n" +
		"Eres un experto en logística argentina especializado en convertir mensajes de texto en datos estructurados para cargas de transporte.\n\n" +
		"## OBJETIVO\n\n" +
		"Convertir mensajes de texto sobre cargas de transporte en un array JSON válido para el sistema CARICA.\n\n" +
		"## FORMATO DE RESPUESTA OBLIGATORIO\n\n" +
		"Debes responder ÚNICAMENTE con un array JSON válido, sin explicaciones adicionales.\n\n" +
		"## ESTRUCTURA DEL JSON\n\n" +
		"El JSON debe ser un ARRAY de objetos, donde cada objeto representa una carga con campos como material, presentacion, peso, tipoEquipo, localidadCarga, localidadDescarga, fechaCarga, fechaDescarga, telefono, correo, puntoReferencia, precio, formaDePago, observaciones.\n\n" +
		"## INSTRUCCIONES CRÍTICAS\n\n" +
		"1. SIEMPRE responde con un array JSON válido\n" +
		"2. NO incluyas explicaciones adicionales\n" +
		"3. Si el mensaje no contiene información de carga, responde con un array vacío: []\n" +
		"4. Usa valores por defecto cuando falte información específica\n" +
		"5. Formato de fechas: DD/MM/YYYY\n" +
		"6. Teléfono debe incluir código de país (+54)\n" +
		"7. Ubicaciones deben incluir ciudad, provincia y Argentina\n" +
		"8. Peso siempre como string en kilogramos\n" +
		"9. Precio siempre como string en pesos argentinos\n" +
		"10. Si no se especifica fecha, usar la fecha actual\n\n" +
		"## MATERIALES VÁLIDOS\n" +
		"Agroquímicos, Alimentos y bebidas, Fertilizante, Ganado, Girasol, Maiz, Maquinarias, Materiales construcción, Otras cargas generales, Otros cultivos, Refrigerados, Soja, Trigo\n\n" +
		"## PRESENTACIONES VÁLIDAS\n" +
		"Big Bag, Bolsa, Granel, Otros, Pallet\n\n" +
		"## TIPOS DE EQUIPO VÁLIDOS\n" +
		"Batea, Camioneta, CamionJaula, Carreton, Chasis y Acoplado, Furgon, Otros, Semi, Tolva\n\n" +
		"## FORMAS DE PAGO VÁLIDAS\n" +
		"Cheque, E-check, Efectivo, Otros, Transferencia"
}

// ProcessMessage procesa un mensaje con IA agregando el teléfono real
func (s *AIService) ProcessMessage(content string, realPhone string) ([]byte, error) {
	return s.processMessageWithRetry(content, realPhone, 0)
}

// processMessageWithRetry procesa un mensaje con IA con manejo de reintentos para quota
func (s *AIService) processMessageWithRetry(content string, realPhone string, retryCount int) ([]byte, error) {
	// Prevenir recursión infinita (máximo 1 cambio de key)
	if retryCount > 1 {
		return nil, fmt.Errorf("too many retries, all API keys exhausted")
	}
	
	// Agregar "ALT: +5491234..." al final del mensaje
	messageWithAlt := fmt.Sprintf("%s\n\nALT: %s", content, realPhone)
	
	// Construir prompt completo con instrucción explícita de NO usar markdown
	fullPrompt := fmt.Sprintf("%s\n\n**IMPORTANTE: Responde ÚNICAMENTE con el array JSON, SIN usar bloques de código markdown (```), SIN backticks, SIN explicaciones. Solo el JSON puro.**\n\nMENSAJE:\n%s", s.systemPrompt, messageWithAlt)
	
	// Crear solicitud para Gemini
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{
						Text: fullPrompt,
					},
				},
			},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:      s.config.Temperature,
			MaxOutputTokens:  s.config.MaxTokens,
			TopP:             0.8,
			TopK:             40,
		},
	}
	
	// Serializar solicitud
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// Llamar a Gemini API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", 
		s.config.Model, s.config.APIKey)
	
	startTime := time.Now()
	fmt.Printf("🤖 [%s] Enviando request a Gemini...\n", startTime.Format("15:04:05"))
	fmt.Printf("📏 Tamaño del prompt: %d caracteres\n", len(fullPrompt))
	fmt.Printf("⏱️ Esperando respuesta (timeout: 120s)...\n")
	
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(requestBody))
	elapsed := time.Since(startTime)
	fmt.Printf("⏱️ Respuesta recibida en %.2f segundos\n", elapsed.Seconds())
	if err != nil {
		return nil, fmt.Errorf("failed to call Gemini API: %v", err)
	}
	defer resp.Body.Close()
	
	fmt.Printf("📥 Recibiendo respuesta de Gemini... (Status: %d)\n", resp.StatusCode)
	
	// Leer respuesta
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	fmt.Printf("📦 Respuesta recibida: %d bytes\n", len(responseBody))
	
	// Verificar código de estado
	if resp.StatusCode != http.StatusOK {
		// Si es error 429 (quota excedida), intentar con otra key
		if resp.StatusCode == 429 && s.config.KeysManager != nil && retryCount == 0 {
			// Intentar cambiar a la siguiente key
			newKey, err := s.config.KeysManager.TryNextKey()
			if err == nil {
				// Actualizar la key en la configuración
				s.config.APIKey = newKey
				// Reintentar con la nueva key
				return s.processMessageWithRetry(content, realPhone, retryCount+1)
			}
		}
		return nil, fmt.Errorf("Gemini API error %d: %s", resp.StatusCode, string(responseBody))
	}
	
	// Parsear respuesta
	var geminiResp GeminiResponse
	if err := json.Unmarshal(responseBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %v", err)
	}
	
	// Verificar que hay candidatos
	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini response")
	}
	
	// Extraer texto de respuesta
	responseText := ""
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		responseText += part.Text
	}
	
	// Limpiar la respuesta de IA (remover markdown si lo tiene)
	cleanedResponse := cleanAIResponse(responseText)
	
	// Validar que la respuesta es JSON válido
	var jsonArray []interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &jsonArray); err != nil {
		return nil, fmt.Errorf("invalid JSON response from AI: %v (original: %s)", err, responseText)
	}
	
	return []byte(cleanedResponse), nil
}

// cleanAIResponse limpia la respuesta de IA removiendo markdown y espacios
func cleanAIResponse(response string) string {
	// Remover bloques de código markdown (```json ... ``` o ``` ... ```)
	cleaned := response
	
	// Buscar y remover ```json al inicio
	if strings.HasPrefix(strings.TrimSpace(cleaned), "```json") {
		cleaned = strings.TrimPrefix(strings.TrimSpace(cleaned), "```json")
	} else if strings.HasPrefix(strings.TrimSpace(cleaned), "```") {
		cleaned = strings.TrimPrefix(strings.TrimSpace(cleaned), "```")
	}
	
	// Buscar y remover ``` al final
	if strings.HasSuffix(strings.TrimSpace(cleaned), "```") {
		cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), "```")
	}
	
	// Trim espacios finales
	cleaned = strings.TrimSpace(cleaned)
	
	return cleaned
}

// ValidateResponse valida que la respuesta de IA sea un JSON válido
func (s *AIService) ValidateResponse(response []byte) error {
	var jsonArray []interface{}
	return json.Unmarshal(response, &jsonArray)
}
