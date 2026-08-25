-- Índice para detectar imágenes duplicadas por file_sha256
CREATE INDEX IF NOT EXISTS idx_messages_image_sha256 ON messages (media_type, file_sha256(32));
