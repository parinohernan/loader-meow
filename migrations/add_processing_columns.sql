-- Migración: Agregar columnas de procesamiento a la tabla messages
-- Fecha: 2025-10-11
-- Descripción: Agrega columnas para manejar reintentos de procesamiento con IA

-- Verificar y agregar columna processing_attempts
ALTER TABLE messages 
ADD COLUMN IF NOT EXISTS processing_attempts INT DEFAULT 0;

-- Verificar y agregar columna last_processing_error
ALTER TABLE messages 
ADD COLUMN IF NOT EXISTS last_processing_error TEXT;

-- Verificar y agregar columna last_processing_attempt
ALTER TABLE messages 
ADD COLUMN IF NOT EXISTS last_processing_attempt TIMESTAMP NULL;

-- Mostrar estructura actualizada
DESCRIBE messages;
