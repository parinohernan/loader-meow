-- Migración: Agregar columna secondary_config_id a ai_configs
-- Esta columna permite configurar una IA secundaria como fallback

-- Agregar columna si no existe
ALTER TABLE ai_configs 
ADD COLUMN IF NOT EXISTS secondary_config_id INT NULL 
AFTER is_enabled;

-- Agregar foreign key si no existe
-- Nota: MySQL/MariaDB no soporta IF NOT EXISTS para foreign keys, 
-- así que se debe ejecutar manualmente si es necesario
-- ALTER TABLE ai_configs 
-- ADD CONSTRAINT fk_secondary_config 
-- FOREIGN KEY (secondary_config_id) REFERENCES ai_configs(id) ON DELETE SET NULL;

