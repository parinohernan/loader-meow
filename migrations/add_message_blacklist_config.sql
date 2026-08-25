INSERT INTO system_configs (config_key, config_value, description, updated_at)
VALUES
    ('message_blacklist_enabled', 'true', 'Activa filtro de palabras blacklist en procesamiento de cargas', CURRENT_TIMESTAMP),
    ('message_blacklist_words', '["uruguay","chile","brasil","paraguay","bolivia","peru","ecuador","colombia","venezuela","mexico","brazil"]', 'Palabras blacklist JSON array', CURRENT_TIMESTAMP)
ON DUPLICATE KEY UPDATE config_key = config_key;
