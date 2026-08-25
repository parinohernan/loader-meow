-- Soporte de visión en modelos existentes
ALTER TABLE ai_models ADD COLUMN supports_vision BOOLEAN DEFAULT 0;

-- Marcar modelos Gemini con soporte de visión
UPDATE ai_models SET supports_vision = 1
WHERE name LIKE 'gemini-%' OR name LIKE 'gemini_%';

-- Tabla de configuraciones OCR (independiente de ai_configs)
CREATE TABLE IF NOT EXISTS ocr_configs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    provider_id INT NOT NULL,
    model_id INT NOT NULL,
    api_key VARCHAR(500) NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT 0,
    is_enabled BOOLEAN DEFAULT 1,
    error_count INT DEFAULT 0,
    last_error TEXT,
    last_used_at TIMESTAMP NULL,
    last_success_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES ai_providers(id) ON DELETE CASCADE,
    FOREIGN KEY (model_id) REFERENCES ai_models(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX IF NOT EXISTS idx_ocr_configs_active ON ocr_configs(is_active);
CREATE INDEX IF NOT EXISTS idx_ocr_configs_provider ON ocr_configs(provider_id);
CREATE INDEX IF NOT EXISTS idx_ai_models_vision ON ai_models(supports_vision);
