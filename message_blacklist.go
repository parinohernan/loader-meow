package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MessageBlacklistConfig configuración del filtro de palabras
type MessageBlacklistConfig struct {
	Enabled bool     `json:"enabled"`
	Words   []string `json:"words"`
}

// MessageBlacklistManager gestiona el filtro configurable de palabras
type MessageBlacklistManager struct {
	systemConfigManager *SystemConfigManager
	mu                  sync.RWMutex
	cached              MessageBlacklistConfig
}

func NewMessageBlacklistManager(systemConfigManager *SystemConfigManager) *MessageBlacklistManager {
	m := &MessageBlacklistManager{systemConfigManager: systemConfigManager}
	_, _ = m.LoadConfig()
	return m
}

func (m *MessageBlacklistManager) LoadConfig() (MessageBlacklistConfig, error) {
	cfg := MessageBlacklistConfig{
		Enabled: true,
		Words:   defaultBlacklistWords(),
	}

	if val, err := m.systemConfigManager.GetConfig("message_blacklist_enabled"); err == nil {
		cfg.Enabled = parseConfigBool(val)
	}
	if val, err := m.systemConfigManager.GetConfig("message_blacklist_words"); err == nil && strings.TrimSpace(val) != "" {
		var words []string
		if err := json.Unmarshal([]byte(val), &words); err == nil && len(words) > 0 {
			cfg.Words = normalizeWordList(words)
		}
	}

	m.mu.Lock()
	m.cached = cfg
	m.mu.Unlock()

	return cfg, nil
}

func defaultBlacklistWords() []string {
	return []string{
		"uruguay", "chile", "brasil", "paraguay", "bolivia", "peru",
		"ecuador", "colombia", "venezuela", "mexico", "brazil",
	}
}

func (m *MessageBlacklistManager) GetConfig() MessageBlacklistConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cached
}

func (m *MessageBlacklistManager) SaveConfig(enabled bool, words []string) error {
	words = normalizeWordList(words)
	if err := m.systemConfigManager.SetConfig("message_blacklist_enabled", boolToString(enabled)); err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(words)
	if err != nil {
		return fmt.Errorf("failed to marshal blacklist words: %v", err)
	}
	if err := m.systemConfigManager.SetConfig("message_blacklist_words", string(jsonBytes)); err != nil {
		return err
	}
	_, err = m.LoadConfig()
	return err
}

func parseWordsInput(input string) []string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	})
	return normalizeWordList(parts)
}

func normalizeWordList(words []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, w := range words {
		w = strings.TrimSpace(strings.ToLower(w))
		if w == "" {
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		result = append(result, w)
	}
	return result
}

// Match devuelve la palabra blacklist que coincide, o false si no hay match.
func (m *MessageBlacklistManager) Match(content string) (string, bool) {
	cfg := m.GetConfig()
	if !cfg.Enabled || len(cfg.Words) == 0 {
		return "", false
	}

	normalized := stripLocationExceptions(content)
	for _, word := range cfg.Words {
		if strings.Contains(normalized, normalizeForMatching(word)) {
			return word, true
		}
	}
	return "", false
}
