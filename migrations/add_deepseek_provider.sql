-- Migración para agregar DeepSeek como proveedor de IA
-- Fecha: 2025-10-12

-- Agregar DeepSeek como proveedor (solo si no existe)
INSERT IGNORE INTO ai_providers (name, display_name, base_url, priority) 
VALUES ('deepseek', 'DeepSeek', 'https://api.deepseek.com/v1', 85);

-- Obtener el ID del proveedor DeepSeek
SET @deepseek_provider_id = (SELECT id FROM ai_providers WHERE name = 'deepseek');

-- Agregar modelos de DeepSeek (solo si no existen)
INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default) 
VALUES 
    (@deepseek_provider_id, 'deepseek-chat', 'DeepSeek Chat', 4096, 32768, 1),
    (@deepseek_provider_id, 'deepseek-coder', 'DeepSeek Coder', 4096, 32768, 0);

-- Verificar que se agregó correctamente
SELECT 'DeepSeek provider agregado correctamente' AS status;
SELECT * FROM ai_providers WHERE name = 'deepseek';
SELECT * FROM ai_models WHERE provider_id = @deepseek_provider_id;

