package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DevAgentConfig configuración del agente de desarrollo
type DevAgentConfig struct {
	Enabled         bool   `json:"enabled"`
	GroupJID        string `json:"group_jid"`
	ContextMessages int    `json:"context_messages"`
	TriggerPrefix   string `json:"trigger_prefix"`
	AIConfigID      int    `json:"ai_config_id"`
}

// DevAgentReminder recordatorio del agente
type DevAgentReminder struct {
	ID              int        `json:"id"`
	ChatJID         string     `json:"chat_jid"`
	CreatorPhone    string     `json:"creator_phone"`
	CreatorName     string     `json:"creator_name"`
	Content         string     `json:"content"`
	DueAt           *time.Time `json:"due_at"`
	Status          string     `json:"status"`
	SourceMessageID string     `json:"source_message_id"`
	CreatedAt       time.Time  `json:"created_at"`
}

// DevAgentStore persiste recordatorios e interacciones del agente
type DevAgentStore struct {
	db                    *sql.DB
	systemConfigManager   *SystemConfigManager
}

func NewDevAgentStore(db *sql.DB, systemConfigManager *SystemConfigManager) *DevAgentStore {
	return &DevAgentStore{
		db:                  db,
		systemConfigManager: systemConfigManager,
	}
}

func (s *DevAgentStore) LoadConfig() (DevAgentConfig, error) {
	cfg := DevAgentConfig{
		ContextMessages: 30,
		TriggerPrefix:   "*",
	}

	if val, err := s.systemConfigManager.GetConfig("dev_agent_enabled"); err == nil {
		cfg.Enabled = parseConfigBool(val)
	}
	if val, err := s.systemConfigManager.GetConfig("dev_agent_group_jid"); err == nil {
		cfg.GroupJID = strings.TrimSpace(val)
	}
	if val, err := s.systemConfigManager.GetConfig("dev_agent_context_messages"); err == nil {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			cfg.ContextMessages = n
		}
	}
	if val, err := s.systemConfigManager.GetConfig("dev_agent_trigger_prefix"); err == nil && val != "" {
		cfg.TriggerPrefix = val
	}
	if val, err := s.systemConfigManager.GetConfig("dev_agent_ai_config_id"); err == nil && val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.AIConfigID = n
		}
	}

	return cfg, nil
}

func (s *DevAgentStore) SaveConfig(cfg DevAgentConfig) error {
	cfg.GroupJID = strings.TrimSpace(cfg.GroupJID)
	if err := s.systemConfigManager.SetConfig("dev_agent_enabled", boolToString(cfg.Enabled)); err != nil {
		return err
	}
	if err := s.systemConfigManager.SetConfig("dev_agent_group_jid", cfg.GroupJID); err != nil {
		return err
	}
	if cfg.ContextMessages <= 0 {
		cfg.ContextMessages = 30
	}
	if err := s.systemConfigManager.SetConfig("dev_agent_context_messages", strconv.Itoa(cfg.ContextMessages)); err != nil {
		return err
	}
	if cfg.TriggerPrefix == "" {
		cfg.TriggerPrefix = "*"
	}
	if err := s.systemConfigManager.SetConfig("dev_agent_trigger_prefix", cfg.TriggerPrefix); err != nil {
		return err
	}
	aiConfigID := ""
	if cfg.AIConfigID > 0 {
		aiConfigID = strconv.Itoa(cfg.AIConfigID)
	}
	return s.systemConfigManager.SetConfig("dev_agent_ai_config_id", aiConfigID)
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func parseConfigBool(val string) bool {
	val = strings.TrimSpace(strings.ToLower(val))
	return val == "true" || val == "1" || val == "yes" || val == "on"
}

func (s *DevAgentStore) SaveReminder(r DevAgentReminder) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO dev_agent_reminders
			(chat_jid, creator_phone, creator_name, content, due_at, status, source_message_id)
		VALUES (?, ?, ?, ?, ?, 'pending', ?)
	`, r.ChatJID, r.CreatorPhone, r.CreatorName, r.Content, r.DueAt, r.SourceMessageID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *DevAgentStore) GetReminders(status string, limit int) ([]DevAgentReminder, error) {
	if limit <= 0 {
		limit = 50
	}
	if status == "" {
		status = "pending"
	}

	rows, err := s.db.Query(`
		SELECT id, chat_jid, creator_phone, creator_name, content, due_at, status, source_message_id, created_at
		FROM dev_agent_reminders
		WHERE status = ?
		ORDER BY COALESCE(due_at, created_at) ASC
		LIMIT ?
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []DevAgentReminder
	for rows.Next() {
		var r DevAgentReminder
		var dueAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.ChatJID, &r.CreatorPhone, &r.CreatorName, &r.Content,
			&dueAt, &r.Status, &r.SourceMessageID, &r.CreatedAt); err != nil {
			return nil, err
		}
		if dueAt.Valid {
			r.DueAt = &dueAt.Time
		}
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func (s *DevAgentStore) MarkReminderDone(id int) error {
	result, err := s.db.Exec(`UPDATE dev_agent_reminders SET status = 'done' WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("reminder not found: %d", id)
	}
	return nil
}

func (s *DevAgentStore) SaveInteraction(chatJID, messageID, userMessage, aiResponse string, remindersCreated int) error {
	_, err := s.db.Exec(`
		INSERT INTO dev_agent_interactions
			(chat_jid, trigger_message_id, user_message, ai_response, reminders_created)
		VALUES (?, ?, ?, ?, ?)
	`, chatJID, messageID, userMessage, aiResponse, remindersCreated)
	return err
}

func (s *DevAgentStore) FormatPendingReminders(chatJID string) (string, error) {
	rows, err := s.db.Query(`
		SELECT id, creator_name, content, due_at
		FROM dev_agent_reminders
		WHERE chat_jid = ? AND status = 'pending'
		ORDER BY COALESCE(due_at, created_at) ASC
		LIMIT 20
	`, chatJID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var id int
		var creator, content string
		var dueAt sql.NullTime
		if err := rows.Scan(&id, &creator, &content, &dueAt); err != nil {
			return "", err
		}
		dueStr := "sin fecha"
		if dueAt.Valid {
			dueStr = dueAt.Time.Format("02/01/2006 15:04")
		}
		lines = append(lines, fmt.Sprintf("- [#%d] %s (de %s, vence: %s)", id, content, creator, dueStr))
	}
	if len(lines) == 0 {
		return "No hay recordatorios pendientes.", nil
	}
	result := "Recordatorios pendientes:\n"
	for _, line := range lines {
		result += line + "\n"
	}
	return result, nil
}
