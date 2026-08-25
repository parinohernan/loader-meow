package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// AIConfigManager maneja la configuración de IA desde la base de datos
type AIConfigManager struct {
	db                *sql.DB
	mu                sync.RWMutex
	
	// Caché de configuración activa (optimización para concurrencia)
	activeConfigCache *AIConfigDB
	cacheTime         time.Time
	cacheTTL          time.Duration // Time-to-live del caché (por defecto 2 segundos)
}

// AIProvider representa un proveedor de IA
type AIProvider struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	BaseURL     string    `json:"base_url"`
	IsEnabled   bool      `json:"is_enabled"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIModel representa un modelo de IA
type AIModel struct {
	ID            int       `json:"id"`
	ProviderID    int       `json:"provider_id"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name"`
	MaxTokens     int       `json:"max_tokens"`
	ContextWindow int       `json:"context_window"`
	IsEnabled     bool      `json:"is_enabled"`
	IsDefault     bool      `json:"is_default"`
	CreatedAt     time.Time `json:"created_at"`
}

// AIConfigDB representa una configuración de API key en la base de datos
type AIConfigDB struct {
	ID                int       `json:"id"`
	ProviderID        int       `json:"provider_id"`
	ModelID           int       `json:"model_id"`
	APIKey            string    `json:"api_key"`
	Name              string    `json:"name"`
	IsActive          bool      `json:"is_active"`
	IsEnabled         bool      `json:"is_enabled"`
	SecondaryConfigID *int      `json:"secondary_config_id"` // ID de la configuración secundaria (fallback)
	ErrorCount        int       `json:"error_count"`
	LastError         string    `json:"last_error"`
	LastUsedAt        *time.Time `json:"last_used_at"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	
	// Campos adicionales para facilitar el uso en el frontend
	ProviderName    string `json:"provider_name"`
	ProviderDisplay string `json:"provider_display"`
	ModelName       string `json:"model_name"`
	ModelDisplay    string `json:"model_display"`
	MaxTokens       int    `json:"max_tokens"`
}

// NewAIConfigManager crea una nueva instancia del manejador
func NewAIConfigManager(db *sql.DB) *AIConfigManager {
	return &AIConfigManager{
		db:       db,
		cacheTTL: 2 * time.Second, // Caché de config activa por 2 segundos (optimiza concurrencia)
	}
}

// GetAllProviders obtiene todos los proveedores de IA
func (m *AIConfigManager) GetAllProviders() ([]AIProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	rows, err := m.db.Query(`
		SELECT id, name, display_name, base_url, is_enabled, priority, created_at, updated_at
		FROM ai_providers
		ORDER BY priority DESC, display_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var providers []AIProvider
	for rows.Next() {
		var p AIProvider
		err := rows.Scan(&p.ID, &p.Name, &p.DisplayName, &p.BaseURL, &p.IsEnabled, &p.Priority, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	
	return providers, nil
}

// GetModelsByProvider obtiene todos los modelos de un proveedor
func (m *AIConfigManager) GetModelsByProvider(providerID int) ([]AIModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	rows, err := m.db.Query(`
		SELECT id, provider_id, name, display_name, max_tokens, context_window, is_enabled, is_default, created_at
		FROM ai_models
		WHERE provider_id = ?
		ORDER BY is_default DESC, display_name ASC
	`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var models []AIModel
	for rows.Next() {
		var m AIModel
		err := rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.DisplayName, &m.MaxTokens, &m.ContextWindow, &m.IsEnabled, &m.IsDefault, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	
	return models, nil
}

// GetAllConfigs obtiene todas las configuraciones de API keys
func (m *AIConfigManager) GetAllConfigs() ([]AIConfigDB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	rows, err := m.db.Query(`
		SELECT 
			c.id, c.provider_id, c.model_id, c.api_key, c.name, c.is_active, c.is_enabled,
			c.secondary_config_id, c.error_count, c.last_error, c.last_used_at, c.last_success_at, c.created_at, c.updated_at,
			p.name as provider_name, p.display_name as provider_display,
			m.name as model_name, m.display_name as model_display, m.max_tokens
		FROM ai_configs c
		JOIN ai_providers p ON c.provider_id = p.id
		JOIN ai_models m ON c.model_id = m.id
		ORDER BY c.is_active DESC, c.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var configs []AIConfigDB
	for rows.Next() {
		var c AIConfigDB
		var lastUsedAt, lastSuccessAt sql.NullTime
		var lastError sql.NullString
		var secondaryConfigID sql.NullInt64
		
		err := rows.Scan(
			&c.ID, &c.ProviderID, &c.ModelID, &c.APIKey, &c.Name, &c.IsActive, &c.IsEnabled,
			&secondaryConfigID, &c.ErrorCount, &lastError, &lastUsedAt, &lastSuccessAt, &c.CreatedAt, &c.UpdatedAt,
			&c.ProviderName, &c.ProviderDisplay, &c.ModelName, &c.ModelDisplay, &c.MaxTokens,
		)
		if err != nil {
			return nil, err
		}
		
		if secondaryConfigID.Valid {
			id := int(secondaryConfigID.Int64)
			c.SecondaryConfigID = &id
		}
		if lastError.Valid {
			c.LastError = lastError.String
		}
		if lastUsedAt.Valid {
			t := lastUsedAt.Time
			c.LastUsedAt = &t
		}
		if lastSuccessAt.Valid {
			t := lastSuccessAt.Time
			c.LastSuccessAt = &t
		}
		
		configs = append(configs, c)
	}
	
	return configs, nil
}

// GetActiveConfig obtiene la configuración actualmente activa (con caché para optimizar concurrencia)
func (m *AIConfigManager) GetActiveConfig() (*AIConfigDB, error) {
	// Paso 1: Intentar leer del caché (solo RLock, permite múltiples lecturas simultáneas)
	m.mu.RLock()
	if m.activeConfigCache != nil && time.Since(m.cacheTime) < m.cacheTTL {
		cached := m.activeConfigCache
		m.mu.RUnlock()
		// fmt.Printf("🚀 Caché hit - usando config en memoria (sin DB query)\n")
		return cached, nil
	}
	m.mu.RUnlock()
	
	// Paso 2: Caché expirado o vacío, necesitamos Lock exclusivo para actualizar
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check: otro goroutine pudo haber actualizado el caché mientras esperábamos el Lock
	if m.activeConfigCache != nil && time.Since(m.cacheTime) < m.cacheTTL {
		// fmt.Printf("🚀 Caché hit (double-check) - otro goroutine ya actualizó\n")
		return m.activeConfigCache, nil
	}
	
	// Paso 3: Leer de BD y actualizar caché
	// fmt.Printf("💾 Caché miss - leyendo config activa desde BD\n")
	config, err := m.getActiveConfigFromDB()
	if err != nil {
		return nil, err
	}
	
	// Actualizar caché
	m.activeConfigCache = config
	m.cacheTime = time.Now()
	
	return config, nil
}

// getActiveConfigFromDB lee la configuración activa desde la base de datos (función interna)
// NOTA: Esta función debe llamarse solo cuando ya se tiene el Lock (m.mu.Lock)
func (m *AIConfigManager) getActiveConfigFromDB() (*AIConfigDB, error) {
	var c AIConfigDB
	var lastUsedAt, lastSuccessAt sql.NullTime
	var lastError sql.NullString
	var secondaryConfigID sql.NullInt64
	
	err := m.db.QueryRow(`
		SELECT 
			c.id, c.provider_id, c.model_id, c.api_key, c.name, c.is_active, c.is_enabled,
			c.secondary_config_id, c.error_count, c.last_error, c.last_used_at, c.last_success_at, c.created_at, c.updated_at,
			p.name as provider_name, p.display_name as provider_display,
			m.name as model_name, m.display_name as model_display, m.max_tokens
		FROM ai_configs c
		JOIN ai_providers p ON c.provider_id = p.id
		JOIN ai_models m ON c.model_id = m.id
		WHERE c.is_active = 1 AND c.is_enabled = 1 AND p.is_enabled = 1
		LIMIT 1
	`).Scan(
		&c.ID, &c.ProviderID, &c.ModelID, &c.APIKey, &c.Name, &c.IsActive, &c.IsEnabled,
		&secondaryConfigID, &c.ErrorCount, &lastError, &lastUsedAt, &lastSuccessAt, &c.CreatedAt, &c.UpdatedAt,
		&c.ProviderName, &c.ProviderDisplay, &c.ModelName, &c.ModelDisplay, &c.MaxTokens,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active AI configuration found")
	}
	if err != nil {
		return nil, err
	}
	
	if secondaryConfigID.Valid {
		id := int(secondaryConfigID.Int64)
		c.SecondaryConfigID = &id
	}
	if lastError.Valid {
		c.LastError = lastError.String
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		c.LastUsedAt = &t
	}
	if lastSuccessAt.Valid {
		t := lastSuccessAt.Time
		c.LastSuccessAt = &t
	}
	
	return &c, nil
}

// AddConfig agrega una nueva configuración de API key
func (m *AIConfigManager) AddConfig(providerID, modelID int, apiKey, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Verificar que el proveedor y modelo existan
	var exists bool
	err := m.db.QueryRow("SELECT EXISTS(SELECT 1 FROM ai_providers WHERE id = ?)", providerID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("provider not found")
	}
	
	err = m.db.QueryRow("SELECT EXISTS(SELECT 1 FROM ai_models WHERE id = ? AND provider_id = ?)", modelID, providerID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("model not found")
	}
	
	// Insertar la nueva configuración
	_, err = m.db.Exec(`
		INSERT INTO ai_configs (provider_id, model_id, api_key, name, is_enabled, updated_at)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, providerID, modelID, apiKey, name)
	
	return err
}

// AddConfigWithCustomModel agrega una configuración con un modelo personalizado
func (m *AIConfigManager) AddConfigWithCustomModel(providerID int, modelName, apiKey, configName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Verificar que el proveedor exista
	var exists bool
	err := m.db.QueryRow("SELECT EXISTS(SELECT 1 FROM ai_providers WHERE id = ?)", providerID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("provider not found")
	}
	
	// Buscar si ya existe un modelo con ese nombre para ese proveedor
	var modelID int
	err = m.db.QueryRow(`
		SELECT id FROM ai_models 
		WHERE provider_id = ? AND name = ?
	`, providerID, modelName).Scan(&modelID)
	
	if err == sql.ErrNoRows {
		// El modelo no existe, crearlo
		result, err := m.db.Exec(`
			INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_enabled, is_default)
			VALUES (?, ?, ?, 8192, 131072, 1, 0)
		`, providerID, modelName, modelName)
		
		if err != nil {
			return fmt.Errorf("failed to create custom model: %v", err)
		}
		
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get model ID: %v", err)
		}
		modelID = int(id)
	} else if err != nil {
		return fmt.Errorf("failed to check if model exists: %v", err)
	}
	
	// Insertar la nueva configuración
	_, err = m.db.Exec(`
		INSERT INTO ai_configs (provider_id, model_id, api_key, name, is_enabled, updated_at)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, providerID, modelID, apiKey, configName)
	
	return err
}

// UpdateConfig actualiza una configuración existente
func (m *AIConfigManager) UpdateConfig(id int, apiKey, name string, isEnabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE ai_configs 
		SET api_key = ?, name = ?, is_enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, apiKey, name, isEnabled, id)
	
	return err
}

// DeleteConfig elimina una configuración
func (m *AIConfigManager) DeleteConfig(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec("DELETE FROM ai_configs WHERE id = ?", id)
	return err
}

// GetConfigByID obtiene una configuración de IA por ID
func (m *AIConfigManager) GetConfigByID(configID int) (*AIConfigDB, error) {
	return m.GetSecondaryConfig(configID)
}

// GetSecondaryConfig obtiene la configuración secundaria por ID
func (m *AIConfigManager) GetSecondaryConfig(configID int) (*AIConfigDB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var c AIConfigDB
	var lastUsedAt, lastSuccessAt sql.NullTime
	var lastError sql.NullString
	var secondaryConfigID sql.NullInt64
	
	err := m.db.QueryRow(`
		SELECT 
			c.id, c.provider_id, c.model_id, c.api_key, c.name, c.is_active, c.is_enabled,
			c.secondary_config_id, c.error_count, c.last_error, c.last_used_at, c.last_success_at, c.created_at, c.updated_at,
			p.name as provider_name, p.display_name as provider_display,
			m.name as model_name, m.display_name as model_display, m.max_tokens
		FROM ai_configs c
		JOIN ai_providers p ON c.provider_id = p.id
		JOIN ai_models m ON c.model_id = m.id
		WHERE c.id = ? AND c.is_enabled = 1 AND p.is_enabled = 1
		LIMIT 1
	`, configID).Scan(
		&c.ID, &c.ProviderID, &c.ModelID, &c.APIKey, &c.Name, &c.IsActive, &c.IsEnabled,
		&secondaryConfigID, &c.ErrorCount, &lastError, &lastUsedAt, &lastSuccessAt, &c.CreatedAt, &c.UpdatedAt,
		&c.ProviderName, &c.ProviderDisplay, &c.ModelName, &c.ModelDisplay, &c.MaxTokens,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("secondary config not found")
	}
	if err != nil {
		return nil, err
	}
	
	if secondaryConfigID.Valid {
		id := int(secondaryConfigID.Int64)
		c.SecondaryConfigID = &id
	}
	if lastError.Valid {
		c.LastError = lastError.String
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		c.LastUsedAt = &t
	}
	if lastSuccessAt.Valid {
		t := lastSuccessAt.Time
		c.LastSuccessAt = &t
	}
	
	return &c, nil
}

// SetSecondaryConfig establece la configuración secundaria para una configuración activa
func (m *AIConfigManager) SetSecondaryConfig(configID int, secondaryConfigID *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var err error
	if secondaryConfigID == nil {
		_, err = m.db.Exec(`
			UPDATE ai_configs 
			SET secondary_config_id = NULL, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, configID)
	} else {
		_, err = m.db.Exec(`
			UPDATE ai_configs 
			SET secondary_config_id = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, *secondaryConfigID, configID)
	}
	
	if err != nil {
		return err
	}
	
	// Invalidar caché
	m.activeConfigCache = nil
	fmt.Printf("🔄 Caché invalidado (SetSecondaryConfig)\n")
	
	return nil
}

// SetActiveConfig establece una configuración como activa
func (m *AIConfigManager) SetActiveConfig(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Desactivar todas las configuraciones
	_, err = tx.Exec("UPDATE ai_configs SET is_active = 0")
	if err != nil {
		return err
	}
	
	// Activar la seleccionada
	_, err = tx.Exec("UPDATE ai_configs SET is_active = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return err
	}
	
	// Commit de la transacción
	if err := tx.Commit(); err != nil {
		return err
	}
	
	// Invalidar caché para que se recargue en la próxima lectura
	m.activeConfigCache = nil
	fmt.Printf("🔄 Caché invalidado (SetActiveConfig)\n")
	
	return nil
}

// RotateToNextConfig cambia a la siguiente configuración disponible
func (m *AIConfigManager) RotateToNextConfig() (*AIConfigDB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	fmt.Printf("🔍 Buscando siguiente configuración disponible...\n")
	
	// Obtener la configuración actualmente activa
	var currentID int
	m.db.QueryRow("SELECT id FROM ai_configs WHERE is_active = 1 LIMIT 1").Scan(&currentID)
	fmt.Printf("📌 Configuración actual ID: %d\n", currentID)
	
	// Obtener TODAS las configuraciones disponibles
	rows, err := m.db.Query(`
		SELECT 
			c.id, c.provider_id, c.model_id, c.api_key, c.name, c.is_active, c.is_enabled,
			c.error_count, c.last_error, c.last_used_at, c.last_success_at, c.created_at, c.updated_at,
			p.name as provider_name, p.display_name as provider_display,
			m.name as model_name, m.display_name as model_display, m.max_tokens
		FROM ai_configs c
		JOIN ai_providers p ON c.provider_id = p.id
		JOIN ai_models m ON c.model_id = m.id
		WHERE c.is_enabled = 1 AND p.is_enabled = 1
		ORDER BY 
			p.priority DESC,
			c.error_count ASC,
			CASE WHEN c.last_used_at IS NULL THEN 0 ELSE 1 END ASC,
			c.last_used_at ASC
	`)
	
	if err != nil {
		fmt.Printf("❌ Error ejecutando query: %v\n", err)
		return nil, err
	}
	defer rows.Close()
	
	var configs []AIConfigDB
	for rows.Next() {
		var c AIConfigDB
		var lastUsedAt, lastSuccessAt sql.NullTime
		var lastError sql.NullString
		
		err := rows.Scan(
			&c.ID, &c.ProviderID, &c.ModelID, &c.APIKey, &c.Name, &c.IsActive, &c.IsEnabled,
			&c.ErrorCount, &lastError, &lastUsedAt, &lastSuccessAt, &c.CreatedAt, &c.UpdatedAt,
			&c.ProviderName, &c.ProviderDisplay, &c.ModelName, &c.ModelDisplay, &c.MaxTokens,
		)
		if err != nil {
			continue
		}
		
		if lastError.Valid {
			c.LastError = lastError.String
		}
		if lastUsedAt.Valid {
			t := lastUsedAt.Time
			c.LastUsedAt = &t
		}
		if lastSuccessAt.Valid {
			t := lastSuccessAt.Time
			c.LastSuccessAt = &t
		}
		
		configs = append(configs, c)
	}
	
	fmt.Printf("📋 Encontradas %d configuraciones disponibles:\n", len(configs))
	for i, cfg := range configs {
		activeMarker := ""
		if cfg.ID == currentID {
			activeMarker = " ← ACTUAL"
		}
		fmt.Printf("   [%d] ID=%d, %s - %s (%s), Errores=%d%s\n", 
			i+1, cfg.ID, cfg.ProviderDisplay, cfg.ModelDisplay, cfg.Name, cfg.ErrorCount, activeMarker)
	}
	
	if len(configs) == 0 {
		fmt.Printf("❌ No hay configuraciones disponibles\n")
		return nil, fmt.Errorf("no configurations available")
	}
	
	// Buscar la siguiente configuración (diferente de la actual)
	var nextConfig *AIConfigDB
	for _, cfg := range configs {
		if cfg.ID != currentID {
			nextConfig = &cfg
			fmt.Printf("✅ Seleccionada siguiente configuración: ID=%d, %s - %s (%s)\n", 
				cfg.ID, cfg.ProviderDisplay, cfg.ModelDisplay, cfg.Name)
			break
		}
	}
	
	// Si no encontramos otra diferente, usar la primera (puede ser la misma si solo hay 1)
	if nextConfig == nil {
		nextConfig = &configs[0]
		fmt.Printf("⚠️ Solo hay 1 configuración disponible, usando la misma: ID=%d\n", nextConfig.ID)
	}
	
	// Establecer la nueva configuración como activa
	fmt.Printf("🔄 Activando nueva configuración ID=%d...\n", nextConfig.ID)
	fmt.Printf("   📋 Detalles: %s - %s (%s)\n", nextConfig.ProviderDisplay, nextConfig.ModelDisplay, nextConfig.Name)
	
	activateErr := m.SetActiveConfig(nextConfig.ID)
	if activateErr != nil {
		fmt.Printf("❌ ERROR AL ACTIVAR: %v\n", activateErr)
		fmt.Printf("❌ Tipo de error: %T\n", activateErr)
		return nil, fmt.Errorf("error activando configuración ID=%d: %v", nextConfig.ID, activateErr)
	}
	
	fmt.Printf("✅ SetActiveConfig completado sin errores\n")
	nextConfig.IsActive = true
	
	// Actualizar caché con la nueva configuración activa
	m.activeConfigCache = nextConfig
	m.cacheTime = time.Now()
	fmt.Printf("🔄 Caché actualizado con nueva configuración\n")
	
	fmt.Printf("✅ Configuración activada exitosamente: %s - %s (%s)\n", 
		nextConfig.ProviderDisplay, nextConfig.ModelDisplay, nextConfig.Name)
	fmt.Printf("🔙 Retornando nextConfig a ai_provider_service...\n")
	
	return nextConfig, nil
}

// ReportError reporta un error en una configuración
func (m *AIConfigManager) ReportError(id int, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE ai_configs 
		SET error_count = error_count + 1,
		    last_error = ?,
		    last_used_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, errorMsg, id)
	
	return err
}

// ReportSuccess reporta un uso exitoso de una configuración
func (m *AIConfigManager) ReportSuccess(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE ai_configs 
		SET last_used_at = CURRENT_TIMESTAMP,
		    last_success_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	
	return err
}

// ResetErrorCount resetea el contador de errores de una configuración
func (m *AIConfigManager) ResetErrorCount(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE ai_configs 
		SET error_count = 0,
		    last_error = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	
	return err
}

// ToggleProvider habilita/deshabilita un proveedor
func (m *AIConfigManager) ToggleProvider(id int, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE ai_providers 
		SET is_enabled = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, enabled, id)
	
	return err
}

// ToggleModel habilita/deshabilita un modelo
func (m *AIConfigManager) ToggleModel(id int, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	_, err := m.db.Exec(`
		UPDATE ai_models 
		SET is_enabled = ?
		WHERE id = ?
	`, enabled, id)
	
	return err
}

