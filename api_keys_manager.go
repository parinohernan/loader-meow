package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// APIKeysManager maneja el pool de API keys
type APIKeysManager struct {
	mu          sync.RWMutex
	configFile  string
	config      *APIKeysConfig
}

// APIKeysConfig estructura de configuración de API keys
type APIKeysConfig struct {
	GeminiKeys []GeminiKey `json:"gemini_keys"`
	ActiveKeyIndex int     `json:"active_key_index"`
	SupabaseKey string     `json:"supabase_key"`
	GoogleMapsKey string   `json:"google_maps_key"`
}

// GeminiKey representa una API key de Gemini con su estado
type GeminiKey struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	IsActive    bool   `json:"is_active"`
	ErrorCount  int    `json:"error_count"`
	LastError   string `json:"last_error"`
}

// NewAPIKeysManager crea una nueva instancia del manager
func NewAPIKeysManager() (*APIKeysManager, error) {
	configFile := "api-keys-config.json"
	
	manager := &APIKeysManager{
		configFile: configFile,
	}
	
	// Intentar cargar configuración existente
	if err := manager.Load(); err != nil {
		// Si no existe, crear configuración por defecto
		manager.config = manager.createDefaultConfig()
		if err := manager.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %v", err)
		}
	}
	
	return manager, nil
}

// createDefaultConfig crea una configuración por defecto
func (m *APIKeysManager) createDefaultConfig() *APIKeysConfig {
	// Intentar leer desde ai-config.env
	geminiKey := getEnvAI("GEMINI_API_KEY", "")
	supabaseKey := getEnvAI("SUPABASE_KEY", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImlraXVzbWR0bHRha2htbWxsanNwIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzQ2MjkyMzEsImV4cCI6MjA1MDIwNTIzMX0.q6NMMUK2ONGFs-b10XZySVlQiCXSLsjZbtBZyUTiVjc")
	googleMapsKey := getEnvAI("GOOGLE_MAPS_API_KEY", "AIzaSyASe9Id-6Dr6lxr5mCb7O3l2HlmNrY-mRU")
	
	config := &APIKeysConfig{
		GeminiKeys: []GeminiKey{},
		ActiveKeyIndex: 0,
		SupabaseKey: supabaseKey,
		GoogleMapsKey: googleMapsKey,
	}
	
	// Si hay una key en el entorno, agregarla
	if geminiKey != "" {
		config.GeminiKeys = append(config.GeminiKeys, GeminiKey{
			Key:      geminiKey,
			Name:     "Key desde ai-config.env",
			IsActive: true,
		})
	}
	
	return config
}

// Load carga la configuración desde el archivo
func (m *APIKeysManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	data, err := os.ReadFile(m.configFile)
	if err != nil {
		return err
	}
	
	var config APIKeysConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	
	m.config = &config
	return nil
}

// Save guarda la configuración en el archivo
func (m *APIKeysManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(m.configFile, data, 0644)
}

// GetActiveGeminiKey obtiene la API key activa de Gemini
func (m *APIKeysManager) GetActiveGeminiKey() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.config.GeminiKeys) == 0 {
		return "", fmt.Errorf("no Gemini API keys configured")
	}
	
	if m.config.ActiveKeyIndex >= len(m.config.GeminiKeys) {
		m.config.ActiveKeyIndex = 0
	}
	
	key := m.config.GeminiKeys[m.config.ActiveKeyIndex]
	return key.Key, nil
}

// GetAllGeminiKeys obtiene todas las API keys de Gemini
func (m *APIKeysManager) GetAllGeminiKeys() []GeminiKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.config.GeminiKeys
}

// AddGeminiKey agrega una nueva API key de Gemini
func (m *APIKeysManager) AddGeminiKey(key, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Verificar si la key ya existe
	for _, k := range m.config.GeminiKeys {
		if k.Key == key {
			return fmt.Errorf("key already exists")
		}
	}
	
	m.config.GeminiKeys = append(m.config.GeminiKeys, GeminiKey{
		Key:      key,
		Name:     name,
		IsActive: false,
	})
	
	return m.Save()
}

// SetActiveKey establece una key como activa
func (m *APIKeysManager) SetActiveKey(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if index < 0 || index >= len(m.config.GeminiKeys) {
		return fmt.Errorf("invalid key index")
	}
	
	// Desactivar todas las keys
	for i := range m.config.GeminiKeys {
		m.config.GeminiKeys[i].IsActive = false
	}
	
	// Activar la seleccionada
	m.config.GeminiKeys[index].IsActive = true
	m.config.ActiveKeyIndex = index
	
	return m.Save()
}

// RemoveGeminiKey elimina una API key de Gemini
func (m *APIKeysManager) RemoveGeminiKey(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if index < 0 || index >= len(m.config.GeminiKeys) {
		return fmt.Errorf("invalid key index")
	}
	
	// Eliminar la key
	m.config.GeminiKeys = append(m.config.GeminiKeys[:index], m.config.GeminiKeys[index+1:]...)
	
	// Ajustar el índice activo si es necesario
	if m.config.ActiveKeyIndex >= len(m.config.GeminiKeys) && len(m.config.GeminiKeys) > 0 {
		m.config.ActiveKeyIndex = 0
		m.config.GeminiKeys[0].IsActive = true
	}
	
	return m.Save()
}

// ReportError reporta un error con una key específica
func (m *APIKeysManager) ReportError(keyIndex int, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if keyIndex < 0 || keyIndex >= len(m.config.GeminiKeys) {
		return fmt.Errorf("invalid key index")
	}
	
	m.config.GeminiKeys[keyIndex].ErrorCount++
	m.config.GeminiKeys[keyIndex].LastError = errorMsg
	
	return m.Save()
}

// TryNextKey intenta usar la siguiente key disponible
func (m *APIKeysManager) TryNextKey() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.config.GeminiKeys) == 0 {
		return "", fmt.Errorf("no Gemini API keys configured")
	}
	
	// Intentar con la siguiente key
	nextIndex := (m.config.ActiveKeyIndex + 1) % len(m.config.GeminiKeys)
	
	// Desactivar todas las keys
	for i := range m.config.GeminiKeys {
		m.config.GeminiKeys[i].IsActive = false
	}
	
	// Activar la siguiente
	m.config.GeminiKeys[nextIndex].IsActive = true
	m.config.ActiveKeyIndex = nextIndex
	
	if err := m.Save(); err != nil {
		return "", err
	}
	
	return m.config.GeminiKeys[nextIndex].Key, nil
}

// GetSupabaseKey obtiene la API key de Supabase
func (m *APIKeysManager) GetSupabaseKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.config.SupabaseKey
}

// GetGoogleMapsKey obtiene la API key de Google Maps
func (m *APIKeysManager) GetGoogleMapsKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.config.GoogleMapsKey
}

// UpdateSupabaseKey actualiza la API key de Supabase
func (m *APIKeysManager) UpdateSupabaseKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.config.SupabaseKey = key
	return m.Save()
}

// UpdateGoogleMapsKey actualiza la API key de Google Maps
func (m *APIKeysManager) UpdateGoogleMapsKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.config.GoogleMapsKey = key
	return m.Save()
}

// GetConfig obtiene la configuración completa
func (m *APIKeysManager) GetConfig() *APIKeysConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.config
}
