package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed prompt_dev_agent.md
var embeddedDevAgentPrompt string

const devAgentDataDelimiter = "---DEV_AGENT_DATA---"

type devAgentReminderPayload struct {
	Content string  `json:"content"`
	DueAt   *string `json:"due_at"`
	Creator string  `json:"creator"`
}

type devAgentDataPayload struct {
	Reminders []devAgentReminderPayload `json:"reminders"`
}

// DevGroupAgent asiste al grupo de desarrollo vía WhatsApp
type DevGroupAgent struct {
	store           *DevAgentStore
	messageStore    *MessageStore
	aiProvider      *AIProviderService
	aiConfigManager *AIConfigManager
	waService       *WhatsAppService
	systemPrompt    string
	logf            func(string)

	configMu sync.RWMutex
	config   DevAgentConfig

	chatMu      sync.Mutex
	lastReplyAt map[string]time.Time
}

func (a *DevGroupAgent) log(msg string) {
	if a.logf != nil {
		a.logf(msg)
	} else {
		fmt.Println(msg)
	}
}

func (a *DevGroupAgent) SetLogger(logf func(string)) {
	a.logf = logf
}

func NewDevGroupAgent(
	store *DevAgentStore,
	messageStore *MessageStore,
	aiProvider *AIProviderService,
	aiConfigManager *AIConfigManager,
	waService *WhatsAppService,
) (*DevGroupAgent, error) {
	prompt, err := loadDevAgentPrompt()
	if err != nil {
		return nil, err
	}

	agent := &DevGroupAgent{
		store:           store,
		messageStore:    messageStore,
		aiProvider:      aiProvider,
		aiConfigManager: aiConfigManager,
		waService:       waService,
		systemPrompt:    prompt,
		lastReplyAt:     make(map[string]time.Time),
	}

	if err := agent.ReloadConfig(); err != nil {
		return nil, err
	}

	return agent, nil
}

func loadDevAgentPrompt() (string, error) {
	candidates := []string{"prompt_dev_agent.md"}
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), "prompt_dev_agent.md"))
	}
	for _, path := range candidates {
		bytes, err := os.ReadFile(path)
		if err == nil && len(bytes) > 0 {
			return string(bytes), nil
		}
	}
	if strings.TrimSpace(embeddedDevAgentPrompt) != "" {
		return embeddedDevAgentPrompt, nil
	}
	return "", fmt.Errorf("prompt dev agent not available")
}

func (a *DevGroupAgent) ReloadConfig() error {
	cfg, err := a.store.LoadConfig()
	if err != nil {
		return err
	}

	a.configMu.Lock()
	a.config = cfg
	a.configMu.Unlock()

	if cfg.Enabled && cfg.GroupJID != "" {
		a.messageStore.SetExcludedChatJID(cfg.GroupJID)
	} else {
		a.messageStore.SetExcludedChatJID("")
	}

	return nil
}

func (a *DevGroupAgent) GetConfig() DevAgentConfig {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.config
}

func (a *DevGroupAgent) SetConfig(cfg DevAgentConfig) error {
	if err := a.store.SaveConfig(cfg); err != nil {
		return err
	}
	return a.ReloadConfig()
}

func (a *DevGroupAgent) UpdateDependencies(wa *WhatsAppService, messageStore *MessageStore, aiProvider *AIProviderService) {
	if wa != nil {
		a.waService = wa
	}
	if messageStore != nil {
		a.messageStore = messageStore
	}
	if aiProvider != nil {
		a.aiProvider = aiProvider
	}
}

func normalizeChatJID(jid string) string {
	return strings.TrimSpace(jid)
}

func chatJIDsMatch(configured, incoming string) bool {
	a := normalizeChatJID(configured)
	b := normalizeChatJID(incoming)
	return a != "" && a == b
}

func (a *DevGroupAgent) HandleIncomingMessage(msg ChatMessage) {
	content := strings.TrimSpace(msg.Content)
	trigger := "*"
	isTrigger := content != "" && strings.HasPrefix(content, trigger)

	if err := a.ReloadConfig(); err != nil {
		a.log(fmt.Sprintf("⚠️ [DevAgent] Error recargando config: %v", err))
	}
	cfg := a.GetConfig()
	if cfg.TriggerPrefix != "" {
		trigger = cfg.TriggerPrefix
		isTrigger = content != "" && strings.HasPrefix(content, trigger)
	}

	if isTrigger {
		a.log(fmt.Sprintf("🔍 [DevAgent] Mensaje con trigger en chat=%q nombre=%q | enabled=%v configJID=%q",
			msg.ChatJID, msg.ChatName, cfg.Enabled, cfg.GroupJID))
	}

	if !cfg.Enabled {
		if isTrigger {
			a.log("⏭️ [DevAgent] Ignorado: agente deshabilitado. Activá 'Agente habilitado' y guardá.")
		}
		return
	}
	if cfg.GroupJID == "" {
		if isTrigger && strings.Contains(msg.ChatJID, "@g.us") {
			a.log(fmt.Sprintf("⏭️ [DevAgent] Ignorado: sin JID configurado. Copiá este JID en config: %s", msg.ChatJID))
		}
		return
	}
	if !chatJIDsMatch(cfg.GroupJID, msg.ChatJID) {
		if isTrigger {
			a.log(fmt.Sprintf("⏭️ [DevAgent] Ignorado: JID no coincide. Configurado=%q Recibido=%q", cfg.GroupJID, msg.ChatJID))
		}
		return
	}
	if content == "" {
		return
	}
	if !strings.HasPrefix(content, trigger) {
		return
	}

	if msg.IsFromMe {
		a.log(fmt.Sprintf("🤖 [DevAgent] Trigger detectado (mensaje propio) en %s: %s", msg.ChatJID, content))
	} else {
		a.log(fmt.Sprintf("🤖 [DevAgent] Trigger detectado de %s en %s: %s", msg.SenderName, msg.ChatJID, content))
	}

	go a.processMessage(msg)
}

func (a *DevGroupAgent) processMessage(msg ChatMessage) {
	a.chatMu.Lock()
	if last, ok := a.lastReplyAt[msg.ChatJID]; ok && time.Since(last) < 2*time.Second {
		a.chatMu.Unlock()
		a.log(fmt.Sprintf("⏳ [DevAgent] Cooldown activo en %s", msg.ChatJID))
		return
	}
	a.chatMu.Unlock()

	if a.waService == nil || !a.waService.IsConnected() {
		a.log("❌ [DevAgent] WhatsApp no conectado, no se puede responder")
		return
	}

	cfg := a.GetConfig()

	a.log(fmt.Sprintf("🔄 [DevAgent] Procesando mensaje en %s...", msg.ChatJID))
	response, remindersCreated, err := a.generateResponse(msg, cfg, false)
	if err != nil {
		a.log(fmt.Sprintf("❌ [DevAgent] Error procesando mensaje: %v", err))
		return
	}

	response = strings.TrimSpace(response)
	if response == "" {
		a.log("⚠️ [DevAgent] IA devolvió respuesta vacía")
		return
	}

	a.chatMu.Lock()
	a.lastReplyAt[msg.ChatJID] = time.Now()
	a.chatMu.Unlock()

	a.log(fmt.Sprintf("📤 [DevAgent] Enviando respuesta a %s (%d chars)", msg.ChatJID, len(response)))
	if err := a.waService.SendMessage(msg.ChatJID, response); err != nil {
		a.log(fmt.Sprintf("❌ [DevAgent] Error enviando respuesta: %v", err))
		return
	}

	_ = a.store.SaveInteraction(msg.ChatJID, msg.ID, msg.Content, response, remindersCreated)
	a.log(fmt.Sprintf("✅ [DevAgent] Respuesta enviada en %s", msg.ChatJID))
}

func (a *DevGroupAgent) TestMessage(message string) (string, error) {
	cfg := a.GetConfig()
	if cfg.GroupJID == "" {
		return "", fmt.Errorf("configura el JID del grupo de desarrollo primero")
	}

	msg := ChatMessage{
		ID:          "test",
		ChatJID:     cfg.GroupJID,
		SenderName:  "Test User",
		SenderPhone: "0000000000",
		Content:     message,
		Timestamp:   time.Now(),
	}

	response, _, err := a.generateResponse(msg, cfg, true)
	return response, err
}

func (a *DevGroupAgent) generateResponse(msg ChatMessage, cfg DevAgentConfig, dryRun bool) (string, int, error) {
	config, err := a.resolveAIConfig(cfg.AIConfigID)
	if err != nil {
		return "", 0, err
	}

	history, err := a.buildContext(cfg, msg)
	if err != nil {
		return "", 0, err
	}

	pendingReminders, err := a.store.FormatPendingReminders(cfg.GroupJID)
	if err != nil {
		return "", 0, err
	}

	argLocation, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	now := time.Now().In(argLocation)
	systemWithMeta := fmt.Sprintf("%s\n\n## FECHA Y HORA ACTUAL (Argentina)\n- Hoy es: %s\n- Fecha y hora: %s\n\n## Recordatorios pendientes\n%s",
		a.systemPrompt, now.Format("02/01/2006"), now.Format("02/01/2006 15:04"), pendingReminders)

	rawResponse, err := a.aiProvider.ChatWithHistory(config, systemWithMeta, history)
	if err != nil {
		return "", 0, err
	}

	visible, reminders := parseDevAgentResponse(rawResponse)
	if strings.TrimSpace(visible) == "" && strings.TrimSpace(rawResponse) != "" {
		visible = strings.TrimSpace(rawResponse)
	}

	remindersCreated := 0
	if !dryRun {
		for _, r := range reminders {
			reminder := DevAgentReminder{
				ChatJID:         cfg.GroupJID,
				CreatorPhone:    msg.SenderPhone,
				CreatorName:     coalesce(r.Creator, msg.SenderName),
				Content:         r.Content,
				SourceMessageID: msg.ID,
			}
			if r.DueAt != nil && *r.DueAt != "" {
				if t, err := time.Parse(time.RFC3339, *r.DueAt); err == nil {
					reminder.DueAt = &t
				}
			}
			if _, err := a.store.SaveReminder(reminder); err != nil {
				fmt.Printf("⚠️ [DevAgent] Error guardando recordatorio: %v\n", err)
				continue
			}
			remindersCreated++
		}
	}

	return strings.TrimSpace(visible), remindersCreated, nil
}

func (a *DevGroupAgent) resolveAIConfig(configID int) (*AIConfigDB, error) {
	if configID > 0 {
		return a.aiConfigManager.GetConfigByID(configID)
	}
	return a.aiConfigManager.GetActiveConfig()
}

func (a *DevGroupAgent) buildContext(cfg DevAgentConfig, currentMsg ChatMessage) ([]ChatTurn, error) {
	limit := cfg.ContextMessages
	if limit <= 0 {
		limit = 30
	}

	messages, err := a.messageStore.GetMessages(cfg.GroupJID, limit)
	if err != nil {
		return nil, err
	}

	// GetMessages returns DESC; reverse to chronological
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	var turns []ChatTurn
	for _, m := range messages {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		role := "user"
		speaker := m.SenderName
		if m.IsFromMe {
			role = "assistant"
			speaker = "Bot"
		}
		ts := m.Timestamp.In(time.FixedZone("ART", -3*3600)).Format("15:04")
		content := fmt.Sprintf("[%s] %s: %s", ts, speaker, m.Content)
		turns = append(turns, ChatTurn{Role: role, Content: content})
	}

	// Ensure current message is last (may not be in DB yet due to timing)
	if len(turns) == 0 || turns[len(turns)-1].Content != fmt.Sprintf("[%s] %s: %s",
		currentMsg.Timestamp.In(time.FixedZone("ART", -3*3600)).Format("15:04"),
		currentMsg.SenderName, currentMsg.Content) {
		ts := currentMsg.Timestamp.In(time.FixedZone("ART", -3*3600)).Format("15:04")
		turns = append(turns, ChatTurn{
			Role:    "user",
			Content: fmt.Sprintf("[%s] %s: %s", ts, currentMsg.SenderName, currentMsg.Content),
		})
	}

	return normalizeChatHistory(turns), nil
}

// normalizeChatHistory asegura historial compatible con APIs de chat (Gemini exige empezar con user)
func normalizeChatHistory(turns []ChatTurn) []ChatTurn {
	if len(turns) == 0 {
		return turns
	}

	var merged []ChatTurn
	for _, t := range turns {
		if len(merged) > 0 && merged[len(merged)-1].Role == t.Role {
			merged[len(merged)-1].Content += "\n" + t.Content
			continue
		}
		merged = append(merged, t)
	}

	if merged[0].Role == "assistant" {
		merged = append([]ChatTurn{{Role: "user", Content: "(inicio de conversación del grupo)"}}, merged...)
	}

	return merged
}

func parseDevAgentResponse(raw string) (string, []devAgentReminderPayload) {
	raw = strings.TrimSpace(raw)
	idx := strings.Index(raw, devAgentDataDelimiter)
	if idx >= 0 {
		visible := strings.TrimSpace(raw[:idx])
		dataPart := strings.TrimSpace(raw[idx+len(devAgentDataDelimiter):])
		dataPart = strings.TrimPrefix(dataPart, "```json")
		dataPart = strings.TrimPrefix(dataPart, "```")
		dataPart = strings.TrimSuffix(dataPart, "```")
		dataPart = strings.TrimSpace(dataPart)

		var payload devAgentDataPayload
		if err := json.Unmarshal([]byte(dataPart), &payload); err == nil {
			return visible, payload.Reminders
		}
		return visible, nil
	}

	// Fallback: try to find trailing JSON block
	if jsonStart := strings.LastIndex(raw, `{"reminders"`); jsonStart > 0 {
		visible := strings.TrimSpace(raw[:jsonStart])
		jsonPart := strings.TrimSpace(raw[jsonStart:])
		var payload devAgentDataPayload
		if err := json.Unmarshal([]byte(jsonPart), &payload); err == nil {
			return visible, payload.Reminders
		}
	}

	return raw, nil
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
