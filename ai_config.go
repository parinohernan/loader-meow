package main

import (
	"os"
	"strconv"
)

// AIConfig contiene la configuración para el servicio de IA
type AIConfig struct {
	APIKey      string
	Model       string
	Temperature float32
	MaxTokens   int
	KeysManager *APIKeysManager
}

// GetAIConfig obtiene la configuración de IA desde el manager de keys
func GetAIConfig(keysManager *APIKeysManager) *AIConfig {
	temperature, _ := strconv.ParseFloat(getEnvAI("GEMINI_TEMPERATURE", "0.1"), 32)
	maxTokens, _ := strconv.Atoi(getEnvAI("GEMINI_MAX_TOKENS", "8192"))
	
	// Obtener la API key activa del manager
	apiKey := ""
	if keysManager != nil {
		apiKey, _ = keysManager.GetActiveGeminiKey()
	}
	
	// Fallback a variable de entorno si no hay key en el manager
	if apiKey == "" {
		apiKey = getEnvAI("GEMINI_API_KEY", "")
	}
	
	return &AIConfig{
		APIKey:      apiKey,
		Model:       getEnvAI("GEMINI_MODEL", "gemini-2.0-flash-exp"),
		Temperature: float32(temperature),
		MaxTokens:   maxTokens,
		KeysManager: keysManager,
	}
}

// getEnvAI obtiene una variable de entorno con un valor por defecto
func getEnvAI(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// IsConfigured verifica si la configuración de IA está completa
func (config *AIConfig) IsConfigured() bool {
	return config.APIKey != ""
}
