-- Columnas para almacenar texto extraído de imágenes y ruta local del archivo
ALTER TABLE messages ADD COLUMN extracted_text TEXT NULL;
ALTER TABLE messages ADD COLUMN media_local_path VARCHAR(500) NULL;
