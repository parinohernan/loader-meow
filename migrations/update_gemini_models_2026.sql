-- Actualizar modelos Gemini deprecados a IDs vigentes en la API (junio 2026)

UPDATE ai_models SET name = 'gemini-2.5-flash', display_name = 'Gemini 2.5 Flash', supports_vision = 1, is_default = 1
WHERE name IN ('gemini-1.5-flash-latest', 'gemini-2.0-flash-exp', 'gemini-2.0-flash', 'gemini-2.0-flash-001');

UPDATE ai_models SET name = 'gemini-2.5-flash-lite', display_name = 'Gemini 2.5 Flash Lite', supports_vision = 1
WHERE name IN ('gemini-1.5-flash-8b-latest', 'gemini-2.0-flash-lite', 'gemini-2.0-flash-lite-001');

UPDATE ai_models SET name = 'gemini-2.5-pro', display_name = 'Gemini 2.5 Pro', supports_vision = 1
WHERE name IN ('gemini-1.5-pro-latest', 'gemini-1.5-pro');

INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_enabled, is_default, supports_vision) VALUES
    (1, 'gemini-2.5-flash', 'Gemini 2.5 Flash', 8192, 1048576, 1, 1, 1),
    (1, 'gemini-2.5-flash-lite', 'Gemini 2.5 Flash Lite', 8192, 1048576, 1, 0, 1),
    (1, 'gemini-2.5-pro', 'Gemini 2.5 Pro', 8192, 2097152, 1, 0, 1),
    (1, 'gemini-3.5-flash', 'Gemini 3.5 Flash', 8192, 1048576, 1, 0, 1);
