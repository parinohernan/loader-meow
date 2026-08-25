-- Proveedor OCR.space para extracción de texto en imágenes
INSERT IGNORE INTO ai_providers (name, display_name, base_url, priority)
VALUES ('ocrspace', 'OCR.space', 'https://api.ocr.space', 50);

SET @ocrspace_provider_id = (SELECT id FROM ai_providers WHERE name = 'ocrspace');

INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_enabled, is_default, supports_vision)
VALUES
    (@ocrspace_provider_id, 'engine-2', 'OCR Engine 2 (recomendado)', 8192, 8192, 1, 1, 1),
    (@ocrspace_provider_id, 'engine-1', 'OCR Engine 1', 8192, 8192, 1, 0, 1),
    (@ocrspace_provider_id, 'engine-3', 'OCR Engine 3', 8192, 8192, 1, 0, 1);
