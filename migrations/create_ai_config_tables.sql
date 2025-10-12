-- Tabla de proveedores de IA disponibles
CREATE TABLE IF NOT EXISTS ai_providers (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,   -- gemini, grok, qwen
    display_name VARCHAR(255) NOT NULL,  -- Gemini, Grok, Qwen
    base_url VARCHAR(500) NOT NULL,      -- URL base de la API
    is_enabled BOOLEAN DEFAULT 1,        -- Si está habilitado para uso
    priority INT DEFAULT 0,              -- Prioridad (mayor = más prioritario)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Tabla de modelos disponibles por proveedor
CREATE TABLE IF NOT EXISTS ai_models (
    id INT AUTO_INCREMENT PRIMARY KEY,
    provider_id INT NOT NULL,
    name VARCHAR(200) NOT NULL,          -- gemini-1.5-flash, grok-2-latest, etc.
    display_name VARCHAR(255) NOT NULL,  -- Gemini 1.5 Flash, Grok 2 Latest, etc.
    max_tokens INT DEFAULT 8192,         -- Tokens máximos de salida
    context_window INT DEFAULT 32768,    -- Ventana de contexto
    is_enabled BOOLEAN DEFAULT 1,
    is_default BOOLEAN DEFAULT 0,        -- Modelo por defecto del proveedor
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES ai_providers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Tabla de configuraciones (API keys) por modelo
CREATE TABLE IF NOT EXISTS ai_configs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    provider_id INT NOT NULL,
    model_id INT NOT NULL,
    api_key VARCHAR(500) NOT NULL,
    name VARCHAR(255) NOT NULL,          -- Nombre descriptivo (ej: "Key Principal Gemini")
    is_active BOOLEAN DEFAULT 0,         -- Si está actualmente activa
    is_enabled BOOLEAN DEFAULT 1,        -- Si está disponible para rotación
    error_count INT DEFAULT 0,           -- Contador de errores
    last_error TEXT,                     -- Último error registrado
    last_used_at TIMESTAMP NULL,         -- Última vez que se usó
    last_success_at TIMESTAMP NULL,      -- Último uso exitoso
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES ai_providers(id) ON DELETE CASCADE,
    FOREIGN KEY (model_id) REFERENCES ai_models(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insertar proveedores por defecto
INSERT IGNORE INTO ai_providers (name, display_name, base_url, priority) VALUES
    ('gemini', 'Google Gemini', 'https://generativelanguage.googleapis.com/v1beta/models', 100),
    ('groq', 'Groq', 'https://api.groq.com/openai/v1', 95),
    ('grok', 'xAI Grok', 'https://api.x.ai/v1', 90),
    ('deepseek', 'DeepSeek', 'https://api.deepseek.com/v1', 85),
    ('qwen', 'Alibaba Qwen', 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation', 80);

-- Insertar modelos disponibles para Gemini
INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default) VALUES
    (1, 'gemini-1.5-flash-latest', 'Gemini 1.5 Flash (Latest)', 8192, 1048576, 1),
    (1, 'gemini-1.5-flash-8b-latest', 'Gemini 1.5 Flash 8B', 8192, 1048576, 0),
    (1, 'gemini-1.5-pro-latest', 'Gemini 1.5 Pro (Latest)', 8192, 2097152, 0),
    (1, 'gemini-2.0-flash-exp', 'Gemini 2.0 Flash (Experimental)', 8192, 1048576, 0);

-- Insertar modelos disponibles para Groq
INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default) VALUES
    (2, 'llama-3.3-70b-versatile', 'Llama 3.3 70B', 8192, 131072, 1),
    (2, 'llama-3.1-70b-versatile', 'Llama 3.1 70B', 8192, 131072, 0),
    (2, 'mixtral-8x7b-32768', 'Mixtral 8x7B', 32768, 32768, 0),
    (2, 'gemma2-9b-it', 'Gemma 2 9B', 8192, 8192, 0);

-- Insertar modelos disponibles para Grok (xAI)
INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default) VALUES
    (3, 'grok-beta', 'Grok Beta', 8192, 131072, 1),
    (3, 'grok-2-latest', 'Grok 2 Latest', 8192, 131072, 0);

-- Insertar modelos disponibles para DeepSeek
INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default) VALUES
    (4, 'deepseek-chat', 'DeepSeek Chat', 4096, 32768, 1),
    (4, 'deepseek-coder', 'DeepSeek Coder', 4096, 32768, 0);

-- Insertar modelos disponibles para Qwen
INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default) VALUES
    (5, 'qwen-max', 'Qwen Max', 8000, 8000, 1),
    (5, 'qwen-plus', 'Qwen Plus', 8000, 32000, 0),
    (5, 'qwen-turbo', 'Qwen Turbo', 8000, 8000, 0);

-- Índices para mejorar el rendimiento
CREATE INDEX IF NOT EXISTS idx_ai_configs_provider ON ai_configs(provider_id);
CREATE INDEX IF NOT EXISTS idx_ai_configs_model ON ai_configs(model_id);
CREATE INDEX IF NOT EXISTS idx_ai_configs_active ON ai_configs(is_active);
CREATE INDEX IF NOT EXISTS idx_ai_models_provider ON ai_models(provider_id);
CREATE INDEX IF NOT EXISTS idx_ai_providers_enabled ON ai_providers(is_enabled);

-- Tabla de configuraciones generales del sistema
CREATE TABLE IF NOT EXISTS system_configs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_config_key (config_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insertar configuraciones por defecto
INSERT IGNORE INTO system_configs (config_key, config_value, description) VALUES
    ('google_maps_api_key', 'AIzaSyASe9Id-6Dr6lxr5mCb7O3l2HlmNrY-mRU', 'API Key para Google Maps Geocoding'),
    ('supabase_api_key', 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImlraXVzbWR0bHRha2htbWxsanNwIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MzQ2MjkyMzEsImV4cCI6MjA1MDIwNTIzMX0.q6NMMUK2ONGFs-b10XZySVlQiCXSLsjZbtBZyUTiVjc', 'API Key para Supabase'),
    ('supabase_url', 'https://ikiusmdtltakhmmlljsp.supabase.co', 'URL del proyecto Supabase');

