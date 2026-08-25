package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// OCRConfigDB representa una configuración OCR en la base de datos
type OCRConfigDB struct {
	ID            int        `json:"id"`
	ProviderID    int        `json:"provider_id"`
	ModelID       int        `json:"model_id"`
	APIKey        string     `json:"api_key"`
	Name          string     `json:"name"`
	IsActive      bool       `json:"is_active"`
	IsEnabled     bool       `json:"is_enabled"`
	ErrorCount    int        `json:"error_count"`
	LastError     string     `json:"last_error"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	ProviderName    string `json:"provider_name"`
	ProviderDisplay string `json:"provider_display"`
	ModelName       string `json:"model_name"`
	ModelDisplay    string `json:"model_display"`
	MaxTokens       int    `json:"max_tokens"`
}

// OCRConfigManager maneja configuraciones OCR independientes de la IA de cargas
type OCRConfigManager struct {
	db                *sql.DB
	mu                sync.RWMutex
	activeConfigCache *OCRConfigDB
	cacheTime         time.Time
	cacheTTL          time.Duration
}

// NewOCRConfigManager crea una nueva instancia del manejador OCR
func NewOCRConfigManager(db *sql.DB) *OCRConfigManager {
	return &OCRConfigManager{
		db:       db,
		cacheTTL: 2 * time.Second,
	}
}

func (m *OCRConfigManager) scanConfig(row interface {
	Scan(dest ...interface{}) error
}) (*OCRConfigDB, error) {
	var c OCRConfigDB
	var lastUsedAt, lastSuccessAt sql.NullTime
	var lastError sql.NullString

	err := row.Scan(
		&c.ID, &c.ProviderID, &c.ModelID, &c.APIKey, &c.Name, &c.IsActive, &c.IsEnabled,
		&c.ErrorCount, &lastError, &lastUsedAt, &lastSuccessAt, &c.CreatedAt, &c.UpdatedAt,
		&c.ProviderName, &c.ProviderDisplay, &c.ModelName, &c.ModelDisplay, &c.MaxTokens,
	)
	if err != nil {
		return nil, err
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

const ocrConfigSelect = `
	SELECT
		c.id, c.provider_id, c.model_id, c.api_key, c.name, c.is_active, c.is_enabled,
		c.error_count, c.last_error, c.last_used_at, c.last_success_at, c.created_at, c.updated_at,
		p.name as provider_name, p.display_name as provider_display,
		m.name as model_name, m.display_name as model_display, m.max_tokens
	FROM ocr_configs c
	JOIN ai_providers p ON c.provider_id = p.id
	JOIN ai_models m ON c.model_id = m.id
`

// GetAllConfigs obtiene todas las configuraciones OCR
func (m *OCRConfigManager) GetAllConfigs() ([]OCRConfigDB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(ocrConfigSelect + " ORDER BY c.is_active DESC, c.created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []OCRConfigDB
	for rows.Next() {
		c, err := m.scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *c)
	}

	return configs, nil
}

// GetActiveConfig obtiene la configuración OCR activa
func (m *OCRConfigManager) GetActiveConfig() (*OCRConfigDB, error) {
	m.mu.RLock()
	if m.activeConfigCache != nil && time.Since(m.cacheTime) < m.cacheTTL {
		cached := m.activeConfigCache
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeConfigCache != nil && time.Since(m.cacheTime) < m.cacheTTL {
		return m.activeConfigCache, nil
	}

	config, err := m.getActiveConfigFromDB()
	if err != nil {
		return nil, err
	}

	m.activeConfigCache = config
	m.cacheTime = time.Now()
	return config, nil
}

func (m *OCRConfigManager) getActiveConfigFromDB() (*OCRConfigDB, error) {
	row := m.db.QueryRow(ocrConfigSelect + `
		WHERE c.is_active = 1 AND c.is_enabled = 1 AND p.is_enabled = 1 AND m.supports_vision = 1
		LIMIT 1
	`)

	config, err := m.scanConfig(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active OCR configuration found")
	}
	return config, err
}

// GetOCRProviders obtiene proveedores con al menos un modelo OCR/visión habilitado
func (m *OCRConfigManager) GetOCRProviders() ([]AIProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers, err := m.scanOCRProviders(`
		SELECT DISTINCT p.id, p.name, p.display_name, p.base_url, p.is_enabled, p.priority, p.created_at, p.updated_at
		FROM ai_providers p
		INNER JOIN ai_models m ON m.provider_id = p.id AND m.is_enabled = 1 AND m.supports_vision = 1
		WHERE p.is_enabled = 1
		ORDER BY p.priority DESC, p.display_name ASC
	`)
	if err == nil && len(providers) > 0 {
		return providers, nil
	}

	return m.scanOCRProviders(`
		SELECT id, name, display_name, base_url, is_enabled, priority, created_at, updated_at
		FROM ai_providers
		WHERE is_enabled = 1 AND name IN ('gemini', 'ocrspace')
		ORDER BY priority DESC, display_name ASC
	`)
}

func (m *OCRConfigManager) scanOCRProviders(query string, args ...interface{}) ([]AIProvider, error) {
	rows, err := m.db.Query(query, args...)
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

// GetVisionModels obtiene modelos con soporte de visión para un proveedor
func (m *OCRConfigManager) GetVisionModels(providerID int) ([]AIModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models, err := m.scanVisionModels(providerID, true)
	if err == nil && len(models) > 0 {
		return models, nil
	}

	var providerName string
	_ = m.db.QueryRow(`SELECT name FROM ai_providers WHERE id = ?`, providerID).Scan(&providerName)
	if providerName == "ocrspace" {
		return m.scanVisionModels(providerID, false)
	}

	if err != nil {
		return nil, err
	}
	return models, nil
}

func (m *OCRConfigManager) scanVisionModels(providerID int, visionOnly bool) ([]AIModel, error) {
	query := `
		SELECT id, provider_id, name, display_name, max_tokens, context_window, is_enabled, is_default, created_at
		FROM ai_models
		WHERE provider_id = ? AND is_enabled = 1
	`
	if visionOnly {
		query += ` AND supports_vision = 1`
	}
	query += ` ORDER BY is_default DESC, display_name ASC`

	rows, err := m.db.Query(query, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []AIModel
	for rows.Next() {
		var model AIModel
		err := rows.Scan(&model.ID, &model.ProviderID, &model.Name, &model.DisplayName, &model.MaxTokens, &model.ContextWindow, &model.IsEnabled, &model.IsDefault, &model.CreatedAt)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}

	return models, nil
}

// AddConfig agrega una nueva configuración OCR
func (m *OCRConfigManager) AddConfig(providerID, modelID int, apiKey, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var exists bool
	err := m.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM ai_models
			WHERE id = ? AND provider_id = ? AND supports_vision = 1
		)
	`, modelID, providerID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("vision model not found for provider")
	}

	_, err = m.db.Exec(`
		INSERT INTO ocr_configs (provider_id, model_id, api_key, name, is_enabled, updated_at)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, providerID, modelID, apiKey, name)

	return err
}

// AddConfigWithCustomModel agrega configuración OCR con modelo personalizado
func (m *OCRConfigManager) AddConfigWithCustomModel(providerID int, modelName, apiKey, configName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var modelID int
	err := m.db.QueryRow(`
		SELECT id FROM ai_models
		WHERE provider_id = ? AND name = ?
	`, providerID, modelName).Scan(&modelID)

	if err == sql.ErrNoRows {
		result, insertErr := m.db.Exec(`
			INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_enabled, is_default, supports_vision)
			VALUES (?, ?, ?, 8192, 131072, 1, 0, 1)
		`, providerID, modelName, modelName)
		if insertErr != nil {
			return fmt.Errorf("failed to create custom vision model: %v", insertErr)
		}
		id, insertErr := result.LastInsertId()
		if insertErr != nil {
			return fmt.Errorf("failed to get model ID: %v", insertErr)
		}
		modelID = int(id)
	} else if err != nil {
		return fmt.Errorf("failed to check if model exists: %v", err)
	} else {
		_, err = m.db.Exec(`UPDATE ai_models SET supports_vision = 1 WHERE id = ?`, modelID)
		if err != nil {
			return err
		}
	}

	_, err = m.db.Exec(`
		INSERT INTO ocr_configs (provider_id, model_id, api_key, name, is_enabled, updated_at)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, providerID, modelID, apiKey, configName)

	return err
}

// UpdateConfig actualiza una configuración OCR
func (m *OCRConfigManager) UpdateConfig(id int, apiKey, name string, isEnabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE ocr_configs
		SET api_key = ?, name = ?, is_enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, apiKey, name, isEnabled, id)

	return err
}

// DeleteConfig elimina una configuración OCR
func (m *OCRConfigManager) DeleteConfig(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec("DELETE FROM ocr_configs WHERE id = ?", id)
	return err
}

// SetActiveConfig establece una configuración OCR como activa
func (m *OCRConfigManager) SetActiveConfig(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec("UPDATE ocr_configs SET is_active = 0"); err != nil {
		return err
	}

	result, err := tx.Exec(`
		UPDATE ocr_configs c
		JOIN ai_models m ON c.model_id = m.id
		SET c.is_active = 1, c.updated_at = CURRENT_TIMESTAMP
		WHERE c.id = ? AND c.is_enabled = 1 AND m.supports_vision = 1
	`, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("OCR config not found or model does not support vision")
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	m.activeConfigCache = nil
	return nil
}

// ReportError reporta un error en una configuración OCR
func (m *OCRConfigManager) ReportError(id int, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE ocr_configs
		SET error_count = error_count + 1,
		    last_error = ?,
		    last_used_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, errorMsg, id)

	return err
}

// ReportSuccess reporta un uso exitoso de una configuración OCR
func (m *OCRConfigManager) ReportSuccess(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE ocr_configs
		SET last_success_at = CURRENT_TIMESTAMP,
		    last_used_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)

	return err
}

// ResetErrorCount resetea el contador de errores
func (m *OCRConfigManager) ResetErrorCount(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE ocr_configs
		SET error_count = 0, last_error = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)

	return err
}
