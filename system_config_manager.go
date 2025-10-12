package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// SystemConfigManager maneja las configuraciones generales del sistema
type SystemConfigManager struct {
	db *sql.DB
	mu sync.RWMutex
}

// SystemConfig representa una configuración del sistema
type SystemConfig struct {
	ID          int       `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewSystemConfigManager crea una nueva instancia del manejador
func NewSystemConfigManager(db *sql.DB) *SystemConfigManager {
	return &SystemConfigManager{
		db: db,
	}
}

// GetConfig obtiene una configuración por su key
func (m *SystemConfigManager) GetConfig(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var value string
	err := m.db.QueryRow(`
		SELECT config_value 
		FROM system_configs 
		WHERE config_key = ?
	`, key).Scan(&value)
	
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("configuration key not found: %s", key)
	}
	
	return value, err
}

// GetAllConfigs obtiene todas las configuraciones
func (m *SystemConfigManager) GetAllConfigs() ([]SystemConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	rows, err := m.db.Query(`
		SELECT id, config_key, config_value, description, updated_at
		FROM system_configs
		ORDER BY config_key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []SystemConfig
	for rows.Next() {
		var c SystemConfig
		var desc sql.NullString
		
		err := rows.Scan(&c.ID, &c.Key, &c.Value, &desc, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		
		if desc.Valid {
			c.Description = desc.String
		}
		
		configs = append(configs, c)
	}
	
	return configs, nil
}

// SetConfig establece o actualiza una configuración
func (m *SystemConfigManager) SetConfig(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		INSERT INTO system_configs (config_key, config_value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON DUPLICATE KEY UPDATE 
			config_value = VALUES(config_value),
			updated_at = CURRENT_TIMESTAMP
	`, key, value)
	
	return err
}

// UpdateConfig actualiza una configuración existente
func (m *SystemConfigManager) UpdateConfig(key, value, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE system_configs 
		SET config_value = ?,
		    description = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE config_key = ?
	`, value, description, key)
	
	return err
}

// GetGoogleMapsAPIKey obtiene la API key de Google Maps
func (m *SystemConfigManager) GetGoogleMapsAPIKey() string {
	key, err := m.GetConfig("google_maps_api_key")
	if err != nil {
		// Fallback a variable de entorno
		return getEnvAI("GOOGLE_MAPS_API_KEY", "AIzaSyASe9Id-6Dr6lxr5mCb7O3l2HlmNrY-mRU")
	}
	return key
}

// GetSupabaseAPIKey obtiene la API key de Supabase
func (m *SystemConfigManager) GetSupabaseAPIKey() string {
	key, err := m.GetConfig("supabase_api_key")
	if err != nil {
		return getEnvAI("SUPABASE_KEY", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImlraXVzbWR0bHRha2htbWxsanNwIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzQ2MjkyMzEsImV4cCI6MjA1MDIwNTIzMX0.q6NMMUK2ONGFs-b10XZySVlQiCXSLsjZbtBZyUTiVjc")
	}
	return key
}

// GetSupabaseURL obtiene la URL de Supabase
func (m *SystemConfigManager) GetSupabaseURL() string {
	url, err := m.GetConfig("supabase_url")
	if err != nil {
		return "https://ikiusmdtltakhmmlljsp.supabase.co"
	}
	return url
}

