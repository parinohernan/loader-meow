-- Tablas para el agente de desarrollo de WhatsApp

CREATE TABLE IF NOT EXISTS dev_agent_reminders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    chat_jid VARCHAR(255) NOT NULL,
    creator_phone VARCHAR(50),
    creator_name VARCHAR(200),
    content TEXT NOT NULL,
    due_at TIMESTAMP NULL,
    status ENUM('pending','done','cancelled') DEFAULT 'pending',
    source_message_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_reminders_status_due (status, due_at)
);

CREATE TABLE IF NOT EXISTS dev_agent_interactions (
    id INT AUTO_INCREMENT PRIMARY KEY,
    chat_jid VARCHAR(255) NOT NULL,
    trigger_message_id VARCHAR(255),
    user_message TEXT,
    ai_response TEXT,
    reminders_created INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO system_configs (config_key, config_value, description, updated_at)
VALUES
    ('dev_agent_enabled', 'false', 'Activa/desactiva el agente de desarrollo', CURRENT_TIMESTAMP),
    ('dev_agent_group_jid', '', 'JID del grupo de WhatsApp de desarrollo', CURRENT_TIMESTAMP),
    ('dev_agent_context_messages', '30', 'Cantidad de mensajes de contexto para el agente', CURRENT_TIMESTAMP),
    ('dev_agent_trigger_prefix', '*', 'Prefijo para activar respuesta del agente', CURRENT_TIMESTAMP),
    ('dev_agent_ai_config_id', '', 'ID de config IA dedicada (vacío = activa)', CURRENT_TIMESTAMP)
ON DUPLICATE KEY UPDATE config_key = config_key;
