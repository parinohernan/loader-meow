-- Migración: Agregar sistema de perfilado de teléfonos
-- Fecha: 2025-10-17
-- Descripción: Agrega columnas para perfilar a los participantes del sistema:
--   - nombre: nombre del contacto
--   - perfil: tipo de usuario (camionero, loader, desconocido)
--   - confianza: score de confiabilidad basado en comportamiento

-- Agregar columnas a la tabla phone_associations
ALTER TABLE phone_associations
ADD COLUMN nombre VARCHAR(255) DEFAULT '' COMMENT 'Nombre del contacto',
ADD COLUMN perfil ENUM('desconocido', 'loader', 'camionero') DEFAULT 'desconocido' COMMENT 'Perfil del usuario según sus mensajes',
ADD COLUMN confianza INT DEFAULT 0 COMMENT 'Score de confianza: +1 por carga válida, -1 por mensaje de camionero';

-- Crear índice para búsquedas por perfil
CREATE INDEX idx_perfil ON phone_associations(perfil);

-- Crear índice para búsquedas por confianza
CREATE INDEX idx_confianza ON phone_associations(confianza);

-- Comentario final
-- La columna 'perfil' ayuda a identificar el tipo de usuario
-- La columna 'confianza' permite rankear usuarios según su comportamiento
-- Score positivo = envía ofertas de carga (loader)
-- Score negativo = envía mensajes buscando carga (camionero)

