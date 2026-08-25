package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.mau.fi/whatsmeow/types"
)

// App struct
type App struct {
	ctx              context.Context
	waService        *WhatsAppService
	facebookService  *FacebookService
	messageProcessor *MessageProcessor
	devGroupAgent    *DevGroupAgent
	qrCode           string
}

// SenderInfo representa información de un remitente (alias para frontend)
type SenderInfoResponse struct {
	SenderPhone   string    `json:"sender_phone"`
	SenderName    string    `json:"sender_name"`
	RealPhone     string    `json:"real_phone"`
	MessageCount  int       `json:"message_count"`
	LastMessage   time.Time `json:"last_message"`
	LastGroupName string    `json:"last_group_name"`
	LastChatJID   string    `json:"last_chat_jid"`
}

// NewApp crea una nueva instancia de App
func NewApp() *App {
	return &App{}
}

// startup se llama cuando la app inicia
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.LogInfo(ctx, "Aplicación iniciada")
	if err := a.initConfigServices(); err != nil {
		runtime.LogError(ctx, fmt.Sprintf("Error inicializando configuración: %v", err))
	}
}

// InitWhatsApp inicializa el servicio de WhatsApp
func (a *App) InitWhatsApp() error {
	runtime.LogInfo(a.ctx, "Inicializando WhatsApp...")

	waService, err := NewWhatsAppService()
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp service: %v", err)
	}

	a.waService = waService

	// El message processor ya está inicializado en el waService
	a.messageProcessor = waService.messageProcessor

	if err := a.initDevGroupAgent(); err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Error inicializando agente de desarrollo: %v", err))
		a.devGroupAgent = nil
	} else if a.devGroupAgent != nil {
		cfg := a.devGroupAgent.GetConfig()
		runtime.LogInfo(a.ctx, fmt.Sprintf("Agente dev listo (enabled=%v, grupo=%s)", cfg.Enabled, cfg.GroupJID))
	}

	// Configurar callbacks
	waService.onMessage = func(msg ChatMessage) {
		// Formatear el remitente mostrando nombre y teléfono
		var senderInfo string
		if len(msg.SenderPhone) <= 15 {
			// Número de teléfono real
			senderInfo = fmt.Sprintf("%s (+%s)", msg.SenderName, msg.SenderPhone)
		} else {
			// LID (usuario con privacidad)
			senderInfo = fmt.Sprintf("%s (LID:%s)", msg.SenderName, msg.SenderPhone)
		}
		runtime.LogInfo(a.ctx, fmt.Sprintf("Nuevo mensaje de %s en %s [%s]: %s", senderInfo, msg.ChatName, msg.ChatJID, msg.Content))
		runtime.EventsEmit(a.ctx, "new-message", msg)

		if a.devGroupAgent == nil {
			if err := a.initDevGroupAgent(); err != nil {
				runtime.LogError(a.ctx, fmt.Sprintf("DevAgent no inicializado: %v", err))
			}
		}
		if a.devGroupAgent != nil {
			a.devGroupAgent.HandleIncomingMessage(msg)
		} else if strings.HasPrefix(strings.TrimSpace(msg.Content), "*") {
			runtime.LogWarning(a.ctx, "Mensaje con * recibido pero el agente dev no está inicializado")
		}
	}

	waService.onConnected = func() {
		runtime.LogInfo(a.ctx, "Conectado a WhatsApp!")
		runtime.EventsEmit(a.ctx, "connected")

		// Procesamiento automático desactivado - ahora es manual
		// waService.StartAutoProcessor()
	}

	waService.onQRCode = func(qr string) {
		a.qrCode = qr
		runtime.LogInfo(a.ctx, "QR Code generado")
		runtime.EventsEmit(a.ctx, "qr-code", qr)
	}

	waService.onAuthenticated = func() {
		runtime.LogInfo(a.ctx, "Autenticado vía QR")
		runtime.EventsEmit(a.ctx, "authenticated")
	}

	waService.onLoggedOut = func(reason string) {
		runtime.LogInfo(a.ctx, fmt.Sprintf("Sesión cerrada: %s", reason))
		runtime.EventsEmit(a.ctx, "logged-out", reason)
	}

	return nil
}

// ConnectWhatsApp conecta al servicio de WhatsApp
func (a *App) ConnectWhatsApp() error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	runtime.LogInfo(a.ctx, "Conectando a WhatsApp...")

	err := a.waService.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	// Esperar un momento para que se establezca la conexión
	time.Sleep(2 * time.Second)

	return nil
}

// IsConnected verifica si WhatsApp está conectado
func (a *App) IsConnected() bool {
	if a.waService == nil {
		return false
	}
	return a.waService.IsConnected()
}

// IsLoggedIn verifica si WhatsApp está autenticado
func (a *App) IsLoggedIn() bool {
	if a.waService == nil {
		return false
	}
	return a.waService.IsLoggedIn()
}

// LogoutWhatsApp cierra la sesión actual y limpia la información local
func (a *App) LogoutWhatsApp() error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	runtime.LogInfo(a.ctx, "Cerrando sesión de WhatsApp...")

	if err := a.waService.Logout(); err != nil {
		return fmt.Errorf("failed to logout: %v", err)
	}

	return nil
}

// GetChats obtiene la lista de chats
func (a *App) GetChats() ([]Chat, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	chats, err := a.waService.messageStore.GetChats()
	if err != nil {
		return nil, fmt.Errorf("failed to get chats: %v", err)
	}

	return chats, nil
}

// GetMessages obtiene los mensajes de un chat
func (a *App) GetMessages(chatJID string) ([]ChatMessage, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	messages, err := a.waService.messageStore.GetMessages(chatJID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %v", err)
	}

	// Invertir el orden para mostrar los más antiguos primero
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// SearchMessagesInChat busca mensajes por texto dentro de un chat
func (a *App) SearchMessagesInChat(chatJID, query string) ([]ChatMessage, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	messages, err := a.waService.messageStore.SearchMessagesInChat(chatJID, query, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %v", err)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetUnprocessedMessages obtiene mensajes no procesados
func (a *App) GetUnprocessedMessages(limit int) ([]ChatMessage, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	if limit <= 0 {
		limit = 100
	}

	messages, err := a.waService.messageStore.GetUnprocessedMessages(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unprocessed messages: %v", err)
	}

	return messages, nil
}

// MarkMessageAsProcessed marca un mensaje como procesado
func (a *App) MarkMessageAsProcessed(messageID, chatJID string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.messageStore.MarkMessageAsProcessed(messageID, chatJID)
}

// GetMessageStats obtiene estadísticas de mensajes
func (a *App) GetMessageStats() (map[string]int, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	total, processed, unprocessed, err := a.waService.messageStore.GetMessageStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %v", err)
	}

	return map[string]int{
		"total":       total,
		"processed":   processed,
		"unprocessed": unprocessed,
	}, nil
}

// SendMessage envía un mensaje
func (a *App) SendMessage(recipient, message string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.SendMessage(recipient, message)
}

// GetMyPhone obtiene el número de teléfono de la cuenta conectada
func (a *App) GetMyPhone() string {
	if a.waService == nil || a.waService.client == nil || a.waService.client.Store.ID == nil {
		return ""
	}
	return a.waService.client.Store.ID.User
}

// GetGroupParticipantNumbers obtiene todos los números/IDs de participantes de un grupo
func (a *App) GetGroupParticipantNumbers(groupJID string) ([]ParticipantInfo, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	// Convertir string a types.JID
	parsedJID, err := types.ParseJID(groupJID)
	if err != nil {
		return nil, fmt.Errorf("invalid JID: %v", err)
	}

	return a.waService.ListAllParticipantNumbers(parsedJID), nil
}

// ===== FUNCIONES PARA ASOCIACIONES DE TELÉFONOS =====

// GetSendersForAssociation obtiene todos los remitentes para la pestaña de asociaciones
func (a *App) GetSendersForAssociation() ([]SenderInfoResponse, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	senders, err := a.waService.GetSendersForAssociation()
	if err != nil {
		return nil, err
	}

	// Convertir a SenderInfoResponse para el frontend
	response := make([]SenderInfoResponse, len(senders))
	for i, sender := range senders {
		response[i] = SenderInfoResponse{
			SenderPhone:   sender.SenderPhone,
			SenderName:    sender.SenderName,
			RealPhone:     sender.RealPhone,
			MessageCount:  sender.MessageCount,
			LastMessage:   sender.LastMessage,
			LastGroupName: sender.LastGroupName,
			LastChatJID:   sender.LastChatJID,
		}
	}

	return response, nil
}

// SavePhoneAssociation guarda o actualiza una asociación de teléfono
func (a *App) SavePhoneAssociation(senderPhone, realPhone, displayName string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.SavePhoneAssociation(senderPhone, realPhone, displayName)
}

// DeletePhoneAssociation elimina una asociación
func (a *App) DeletePhoneAssociation(senderPhone string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.DeletePhoneAssociation(senderPhone)
}

// GetMessagesBySenderPhone obtiene mensajes de un remitente específico
func (a *App) GetMessagesBySenderPhone(senderPhone string, limit int) ([]ChatMessage, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	if limit <= 0 {
		limit = 50
	}

	return a.waService.GetMessagesBySenderPhone(senderPhone, limit)
}

// DeleteMessage elimina un mensaje de la base de datos
func (a *App) DeleteMessage(messageID, chatJID string) error {
	if err := a.ensureConfigReady(); err != nil {
		return fmt.Errorf("failed to initialize: %v", err)
	}
	return a.waService.DeleteMessage(messageID, chatJID)
}

// GetImageMessages obtiene mensajes con imágenes para la galería
func (a *App) GetImageMessages(filter string, limit int) ([]ImageMessageInfo, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, fmt.Errorf("failed to initialize: %v", err)
	}
	return a.waService.GetImageMessages(filter, limit)
}

// GetMessageImagePreview obtiene la imagen de un mensaje como data URL
func (a *App) GetMessageImagePreview(messageID, chatJID string) (ImagePreviewResponse, error) {
	if a.waService == nil {
		return ImagePreviewResponse{Success: false, Error: "WhatsApp service not initialized"}, nil
	}
	return a.waService.GetMessageImagePreview(messageID, chatJID)
}

// DiscardImageMessage descarta una imagen sin procesarla con IA
func (a *App) DiscardImageMessage(messageID, chatJID string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}
	return a.waService.DiscardImageMessage(messageID, chatJID)
}

// DeleteAllPendingImageMessages elimina todas las imágenes pendientes de la galería
func (a *App) DeleteAllPendingImageMessages() (int64, error) {
	if err := a.ensureConfigReady(); err != nil {
		return 0, fmt.Errorf("failed to initialize: %v", err)
	}
	return a.waService.DeletePendingImageMessages()
}

// DeleteImageMessages elimina mensajes con imagen mostrados en la galería
func (a *App) DeleteImageMessages(refs []ImageMessageRef) (int64, error) {
	if err := a.ensureConfigReady(); err != nil {
		return 0, fmt.Errorf("failed to initialize: %v", err)
	}
	return a.waService.DeleteImageMessages(refs)
}

// DeleteMessagesBySenderPhone elimina todos los mensajes de un remitente específico
func (a *App) DeleteMessagesBySenderPhone(senderPhone string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.DeleteMessagesBySenderPhone(senderPhone)
}

// DeleteMessagesBySendersInChat elimina mensajes de varios remitentes solo en un chat (p. ej. un grupo).
func (a *App) DeleteMessagesBySendersInChat(chatJID string, senderPhones []string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.DeleteMessagesBySendersInChat(chatJID, senderPhones)
}

// ===== FUNCIONES PARA PROCESAMIENTO CON IA =====

// ProcessMessages procesa mensajes pendientes con IA
func (a *App) ProcessMessages(limit int) ([]ProcessingResult, error) {
	if a.messageProcessor == nil {
		return nil, fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.ProcessPendingMessages(limit)
}

// GetProcessingResults obtiene resultados de procesamiento
func (a *App) GetProcessingResults(limit int) ([]ProcessingResult, error) {
	if a.messageProcessor == nil {
		return nil, fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.GetProcessingResults(limit)
}

// GetProcessableMessagesCount obtiene el conteo de mensajes procesables
func (a *App) GetProcessableMessagesCount() (int, error) {
	if a.messageProcessor == nil {
		return 0, fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.GetProcessableMessagesCount()
}

// GetProcessingStats obtiene estadísticas de procesamiento
func (a *App) GetProcessingStats() (map[string]interface{}, error) {
	if a.messageProcessor == nil {
		return nil, fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.GetProcessingStats()
}

// GetCargasRepetidasCount obtiene el contador de cargas repetidas
func (a *App) GetCargasRepetidasCount() (int, error) {
	if a.messageProcessor == nil {
		return 0, fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.GetCargasRepetidasCount()
}

// ResetearCargasRepetidasCount resetea el contador de cargas repetidas a 0
func (a *App) ResetearCargasRepetidasCount() error {
	if a.messageProcessor == nil {
		return fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.ResetearCargasRepetidasCount()
}

// SimulateMessage procesa un mensaje sin guardarlo (para simulador)
func (a *App) SimulateMessage(messageContent, realPhone string) (ProcessingResult, error) {
	if a.messageProcessor == nil {
		return ProcessingResult{}, fmt.Errorf("message processor not initialized")
	}

	if realPhone == "" {
		realPhone = "+5490000000000" // Teléfono por defecto para simulación
	}

	result := a.messageProcessor.SimulateMessage(messageContent, realPhone)
	return result, nil
}

// SimulateImageMessage simula OCR + procesamiento IA desde una imagen (base64)
func (a *App) SimulateImageMessage(imageBase64, caption, realPhone string) (ProcessingResult, error) {
	if a.messageProcessor == nil {
		return ProcessingResult{}, fmt.Errorf("message processor not initialized")
	}

	if realPhone == "" {
		realPhone = "+5490000000000"
	}

	result := a.messageProcessor.SimulateImageMessage(imageBase64, caption, realPhone)
	return result, nil
}

// ===== FUNCIONES PARA GESTIÓN DE API KEYS =====

// GetGeminiKeys obtiene todas las API keys de Gemini configuradas
func (a *App) GetGeminiKeys() ([]GeminiKey, error) {
	if a.waService == nil || a.waService.messageProcessor == nil {
		return []GeminiKey{}, nil
	}

	keysManager, err := NewAPIKeysManager()
	if err != nil {
		return nil, err
	}

	return keysManager.GetAllGeminiKeys(), nil
}

// AddGeminiKey agrega una nueva API key de Gemini
func (a *App) AddGeminiKey(key, name string) error {
	keysManager, err := NewAPIKeysManager()
	if err != nil {
		return err
	}

	return keysManager.AddGeminiKey(key, name)
}

// SetActiveGeminiKey establece una API key como activa
func (a *App) SetActiveGeminiKey(index int) error {
	keysManager, err := NewAPIKeysManager()
	if err != nil {
		return err
	}

	return keysManager.SetActiveKey(index)
}

// RemoveGeminiKey elimina una API key de Gemini
func (a *App) RemoveGeminiKey(index int) error {
	keysManager, err := NewAPIKeysManager()
	if err != nil {
		return err
	}

	return keysManager.RemoveGeminiKey(index)
}

// GetAPIKeysConfig obtiene la configuración completa de API keys
func (a *App) GetAPIKeysConfig() (*APIKeysConfig, error) {
	keysManager, err := NewAPIKeysManager()
	if err != nil {
		return nil, err
	}

	return keysManager.GetConfig(), nil
}

// ===== FUNCIONES PARA GESTIÓN DE MENSAJES =====

// ReprocessMessage resetea el contador de intentos para reprocesar un mensaje
func (a *App) ReprocessMessage(messageID, chatJID string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.messageStore.ResetProcessingAttempts(messageID, chatJID)
}

// UpdateMessageContent actualiza el contenido de un mensaje
func (a *App) UpdateMessageContent(messageID, chatJID, newContent string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.messageStore.UpdateMessageContent(messageID, chatJID, newContent)
}

// GetMessageDetails obtiene los detalles completos de un mensaje para edición
func (a *App) GetMessageDetails(messageID, chatJID string) (*ChatMessage, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	// Obtener el mensaje de la base de datos
	messages, err := a.waService.messageStore.GetMessages(chatJID, 1000)
	if err != nil {
		return nil, err
	}

	// Buscar el mensaje específico
	for _, msg := range messages {
		if msg.ID == messageID {
			return &msg, nil
		}
	}

	return nil, fmt.Errorf("message not found")
}

// GetUnprocessedMessagesWithRealPhone obtiene mensajes sin procesar con teléfono real
func (a *App) GetUnprocessedMessagesWithRealPhone(limit int) ([]ProcessableMessage, error) {
	if a.waService == nil {
		return nil, fmt.Errorf("WhatsApp service not initialized")
	}

	return a.waService.messageStore.GetUnprocessedMessagesWithRealPhone(limit)
}

// GetProcessedMessagesToday obtiene mensajes procesados exitosamente hoy
func (a *App) GetProcessedMessagesToday(limit int) ([]ProcessingResult, error) {
	if a.waService == nil || a.waService.messageProcessor == nil {
		return nil, fmt.Errorf("message processor not initialized")
	}
	return a.waService.messageProcessor.GetProcessedToday(limit)
}

// GetMessagesWithErrors obtiene mensajes que tuvieron errores al procesar
func (a *App) GetMessagesWithErrors(limit int) ([]ProcessingResult, error) {
	if a.waService == nil || a.waService.messageProcessor == nil {
		return nil, fmt.Errorf("message processor not initialized")
	}
	return a.waService.messageProcessor.GetMessagesWithErrors(limit)
}

// ProcessSingleMessage procesa un solo mensaje por ID
func (a *App) ProcessSingleMessage(messageID, chatJID string) (ProcessingResult, error) {
	if a.messageProcessor == nil {
		return ProcessingResult{}, fmt.Errorf("message processor not initialized")
	}

	return a.messageProcessor.ProcessSingleMessage(messageID, chatJID)
}

// ===================================
// Gestión de Configuraciones del Sistema
// ===================================

// GetSystemConfigs obtiene todas las configuraciones del sistema
func (a *App) GetSystemConfigs() ([]SystemConfig, error) {
	if a.waService == nil || a.waService.systemConfigManager == nil {
		return nil, fmt.Errorf("system config manager not initialized")
	}
	return a.waService.systemConfigManager.GetAllConfigs()
}

// UpdateSystemConfig actualiza una configuración del sistema
func (a *App) UpdateSystemConfig(key, value, description string) error {
	if a.waService == nil || a.waService.systemConfigManager == nil {
		return fmt.Errorf("system config manager not initialized")
	}
	return a.waService.systemConfigManager.UpdateConfig(key, value, description)
}

// GetMessageBlacklistConfig obtiene la configuración del filtro de palabras
func (a *App) GetMessageBlacklistConfig() (MessageBlacklistConfig, error) {
	if err := a.ensureConfigReady(); err != nil {
		return MessageBlacklistConfig{}, err
	}
	if a.messageProcessor == nil {
		if a.waService != nil {
			a.messageProcessor = a.waService.messageProcessor
		}
	}
	if a.messageProcessor == nil {
		return MessageBlacklistConfig{}, fmt.Errorf("message processor not initialized")
	}
	return a.messageProcessor.GetBlacklistConfig(), nil
}

// SetMessageBlacklistConfig guarda la configuración del filtro de palabras
func (a *App) SetMessageBlacklistConfig(enabled bool, wordsInput string) error {
	if err := a.ensureConfigReady(); err != nil {
		return err
	}
	if a.messageProcessor == nil {
		if a.waService != nil {
			a.messageProcessor = a.waService.messageProcessor
		}
	}
	if a.messageProcessor == nil {
		return fmt.Errorf("message processor not initialized")
	}
	return a.messageProcessor.SetBlacklistConfig(enabled, wordsInput)
}

// ===================================
// Gestión de Configuración IA
// ===================================

func (a *App) ensureConfigReady() error {
	if a.waService != nil && a.waService.ocrConfigManager != nil {
		if a.devGroupAgent == nil {
			return a.initDevGroupAgent()
		}
		return nil
	}
	return a.initConfigServices()
}

func (a *App) initConfigServices() error {
	if a.waService != nil && a.waService.ocrConfigManager != nil {
		return nil
	}

	messageStore, err := NewMessageStore()
	if err != nil {
		return fmt.Errorf("failed to connect database: %v", err)
	}

	if err := messageStore.InitOCRConfigTables(); err != nil {
		return fmt.Errorf("failed to init OCR tables: %v", err)
	}
	if err := messageStore.InitAIConfigTables(); err != nil {
		return fmt.Errorf("failed to init AI tables: %v", err)
	}
	if err := messageStore.InitDevAgentTables(); err != nil {
		return fmt.Errorf("failed to init dev agent tables: %v", err)
	}

	if a.waService == nil {
		a.waService = &WhatsAppService{}
	}

	a.waService.messageStore = messageStore
	a.waService.aiConfigManager = NewAIConfigManager(messageStore.db)
	a.waService.ocrConfigManager = NewOCRConfigManager(messageStore.db)
	if a.waService.systemConfigManager == nil {
		a.waService.systemConfigManager = NewSystemConfigManager(messageStore.db)
	}

	if err := a.initDevGroupAgent(); err != nil {
		return fmt.Errorf("failed to init dev group agent: %v", err)
	}

	return nil
}

func (a *App) initDevGroupAgent() error {
	if a.waService == nil || a.waService.messageStore == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("whatsapp service not ready for dev agent")
	}

	aiProvider := NewAIProviderService(a.waService.aiConfigManager)
	if a.messageProcessor != nil && a.messageProcessor.aiProviderService != nil {
		aiProvider = a.messageProcessor.aiProviderService
	}

	store := NewDevAgentStore(a.waService.messageStore.db, a.waService.systemConfigManager)

	if a.devGroupAgent != nil {
		a.devGroupAgent.UpdateDependencies(a.waService, a.waService.messageStore, aiProvider)
		a.devGroupAgent.store = store
		a.devGroupAgent.SetLogger(func(msg string) {
			if a.ctx != nil {
				runtime.LogInfo(a.ctx, msg)
			}
		})
		return a.devGroupAgent.ReloadConfig()
	}

	agent, err := NewDevGroupAgent(
		store,
		a.waService.messageStore,
		aiProvider,
		a.waService.aiConfigManager,
		a.waService,
	)
	if err != nil {
		return err
	}
	agent.SetLogger(func(msg string) {
		if a.ctx != nil {
			runtime.LogInfo(a.ctx, msg)
		}
	})
	a.devGroupAgent = agent
	return nil
}

// GetAIProviders obtiene todos los proveedores de IA disponibles
func (a *App) GetAIProviders() ([]AIProvider, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %v", err)
	}
	if a.waService.aiConfigManager == nil {
		return nil, fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.GetAllProviders()
}

// GetAIModelsByProvider obtiene los modelos disponibles para un proveedor
func (a *App) GetAIModelsByProvider(providerID int) ([]AIModel, error) {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return nil, fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.GetModelsByProvider(providerID)
}

// GetAIConfigs obtiene todas las configuraciones de API keys
func (a *App) GetAIConfigs() ([]AIConfigDB, error) {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return nil, fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.GetAllConfigs()
}

// GetActiveAIConfig obtiene la configuración actualmente activa
func (a *App) GetActiveAIConfig() (*AIConfigDB, error) {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return nil, fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.GetActiveConfig()
}

// AddAIConfig agrega una nueva configuración de API key
func (a *App) AddAIConfig(providerID, modelID int, apiKey, name string) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.AddConfig(providerID, modelID, apiKey, name)
}

// AddAIConfigWithCustomModel agrega una configuración con un modelo personalizado
func (a *App) AddAIConfigWithCustomModel(providerID int, modelName, apiKey, configName string) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.AddConfigWithCustomModel(providerID, modelName, apiKey, configName)
}

// UpdateAIConfig actualiza una configuración existente
func (a *App) UpdateAIConfig(id int, apiKey, name string, isEnabled bool) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.UpdateConfig(id, apiKey, name, isEnabled)
}

// DeleteAIConfig elimina una configuración
func (a *App) DeleteAIConfig(id int) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.DeleteConfig(id)
}

// SetActiveAIConfig establece una configuración como activa
func (a *App) SetActiveAIConfig(id int) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.SetActiveConfig(id)
}

// SetSecondaryAIConfig establece la configuración secundaria para una configuración activa
func (a *App) SetSecondaryAIConfig(configID int, secondaryConfigID *int) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.SetSecondaryConfig(configID, secondaryConfigID)
}

// ResetAIConfigErrors resetea el contador de errores de una configuración
func (a *App) ResetAIConfigErrors(id int) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.ResetErrorCount(id)
}

// ToggleAIProvider habilita/deshabilita un proveedor
func (a *App) ToggleAIProvider(id int, enabled bool) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.ToggleProvider(id, enabled)
}

// ToggleAIModel habilita/deshabilita un modelo
func (a *App) ToggleAIModel(id int, enabled bool) error {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return fmt.Errorf("AI config manager not initialized")
	}
	return a.waService.aiConfigManager.ToggleModel(id, enabled)
}

// TestAIConfig prueba una configuración de IA
func (a *App) TestAIConfig(configID int) (map[string]interface{}, error) {
	if a.waService == nil || a.waService.aiConfigManager == nil {
		return nil, fmt.Errorf("AI config manager not initialized")
	}

	// Obtener la configuración
	configs, err := a.waService.aiConfigManager.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	var testConfig *AIConfigDB
	for _, c := range configs {
		if c.ID == configID {
			testConfig = &c
			break
		}
	}

	if testConfig == nil {
		return nil, fmt.Errorf("config not found")
	}

	// Mensaje de prueba simple
	testMessage := "Hola, este es un mensaje de prueba para verificar la configuración de IA."

	startTime := time.Now()

	// Intentar procesar con el proveedor específico
	providerService := NewAIProviderService(a.waService.aiConfigManager)

	// Temporalmente activar esta config para la prueba
	originalActive, _ := a.waService.aiConfigManager.GetActiveConfig()
	a.waService.aiConfigManager.SetActiveConfig(configID)

	_, err = providerService.ProcessMessage("Eres un asistente de prueba.", testMessage, "test")

	// Restaurar configuración original
	if originalActive != nil {
		a.waService.aiConfigManager.SetActiveConfig(originalActive.ID)
	}

	elapsed := time.Since(startTime).Seconds()

	result := map[string]interface{}{
		"success":  err == nil,
		"elapsed":  elapsed,
		"provider": testConfig.ProviderDisplay,
		"model":    testConfig.ModelDisplay,
	}

	if err != nil {
		result["error"] = err.Error()
	}

	return result, nil
}

// ===== FUNCIONES PARA CONFIGURACIÓN OCR =====

func (a *App) GetOCRConfigs() ([]OCRConfigDB, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %v", err)
	}
	if a.waService.ocrConfigManager == nil {
		return nil, fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.GetAllConfigs()
}

func (a *App) GetActiveOCRConfig() (*OCRConfigDB, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %v", err)
	}
	if a.waService.ocrConfigManager == nil {
		return nil, fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.GetActiveConfig()
}

func (a *App) GetOCRProviders() ([]AIProvider, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %v", err)
	}
	if a.waService.ocrConfigManager == nil {
		return nil, fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.GetOCRProviders()
}

func (a *App) GetVisionModelsByProvider(providerID int) ([]AIModel, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %v", err)
	}
	if a.waService.ocrConfigManager == nil {
		return nil, fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.GetVisionModels(providerID)
}

func (a *App) AddOCRConfig(providerID, modelID int, apiKey, name string) error {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.AddConfig(providerID, modelID, apiKey, name)
}

func (a *App) AddOCRConfigWithCustomModel(providerID int, modelName, apiKey, configName string) error {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.AddConfigWithCustomModel(providerID, modelName, apiKey, configName)
}

func (a *App) UpdateOCRConfig(id int, apiKey, name string, isEnabled bool) error {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.UpdateConfig(id, apiKey, name, isEnabled)
}

func (a *App) DeleteOCRConfig(id int) error {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.DeleteConfig(id)
}

func (a *App) SetActiveOCRConfig(id int) error {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.SetActiveConfig(id)
}

func (a *App) ResetOCRConfigErrors(id int) error {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return fmt.Errorf("OCR config manager not initialized")
	}
	return a.waService.ocrConfigManager.ResetErrorCount(id)
}

func (a *App) TestOCRConfig(configID int) (map[string]interface{}, error) {
	if a.waService == nil || a.waService.ocrConfigManager == nil {
		return nil, fmt.Errorf("OCR config manager not initialized")
	}

	configs, err := a.waService.ocrConfigManager.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	var testConfig *OCRConfigDB
	for _, c := range configs {
		if c.ID == configID {
			testConfig = &c
			break
		}
	}
	if testConfig == nil {
		return nil, fmt.Errorf("OCR config not found")
	}

	ocrService := NewOCRService(a.waService.ocrConfigManager)
	startTime := time.Now()

	// Crear imagen PNG mínima 1x1 para prueba
	testImagePath := "store/ocr_test.png"
	if err := createMinimalTestPNG(testImagePath); err != nil {
		return nil, fmt.Errorf("failed to create test image: %v", err)
	}

	_, err = ocrService.ExtractTextFromImage(testConfig, testImagePath)
	elapsed := time.Since(startTime).Seconds()

	result := map[string]interface{}{
		"success":  err == nil,
		"elapsed":  elapsed,
		"provider": testConfig.ProviderDisplay,
		"model":    testConfig.ModelDisplay,
	}
	if err != nil {
		result["error"] = err.Error()
	}

	return result, nil
}

// Disconnect desconecta WhatsApp
func (a *App) Disconnect() {
	if a.waService != nil {
		a.waService.Disconnect()
	}
}

// ===== FUNCIONES PARA FACEBOOK =====

// InitFacebook inicializa el servicio de Facebook
func (a *App) InitFacebook() error {
	runtime.LogInfo(a.ctx, "Inicializando Facebook...")
	
	// Verificar que WhatsApp service esté inicializado (para acceder a messageStore)
	if a.waService == nil || a.waService.messageStore == nil {
		return fmt.Errorf("WhatsApp service not initialized. Initialize WhatsApp first")
	}
	
	// Cargar token de acceso
	accessToken, err := LoadFacebookConfig()
	if err != nil {
		runtime.LogWarning(a.ctx, fmt.Sprintf("Facebook token not found: %v. You can add groups but they won't fetch posts until token is configured.", err))
		// Crear servicio con token vacío, se puede configurar después
		accessToken = ""
	}
	
	facebookService := NewFacebookService(accessToken, a.waService.messageStore)
	a.facebookService = facebookService
	
	runtime.LogInfo(a.ctx, "Facebook service initialized")
	return nil
}

// AddFacebookGroup agrega un nuevo grupo de Facebook
func (a *App) AddFacebookGroup(groupID, groupName, customAccessToken string) error {
	if a.facebookService == nil {
		return fmt.Errorf("Facebook service not initialized. Call InitFacebook first")
	}
	
	return a.facebookService.AddGroup(groupID, groupName, customAccessToken)
}

// RemoveFacebookGroup elimina un grupo de Facebook
func (a *App) RemoveFacebookGroup(groupID string) error {
	if a.facebookService == nil {
		return fmt.Errorf("Facebook service not initialized")
	}
	
	return a.facebookService.RemoveGroup(groupID)
}

// GetFacebookGroups obtiene todos los grupos de Facebook configurados
func (a *App) GetFacebookGroups() ([]FacebookGroup, error) {
	if a.facebookService == nil {
		return []FacebookGroup{}, nil
	}
	
	return a.facebookService.GetGroups(), nil
}

// ToggleFacebookGroup habilita/deshabilita un grupo de Facebook
func (a *App) ToggleFacebookGroup(groupID string, enabled bool) error {
	if a.facebookService == nil {
		return fmt.Errorf("Facebook service not initialized")
	}
	
	return a.facebookService.ToggleGroup(groupID, enabled)
}

// FetchFacebookGroupPosts obtiene publicaciones de un grupo específico
func (a *App) FetchFacebookGroupPosts(groupID string, limit int) error {
	if a.facebookService == nil {
		return fmt.Errorf("Facebook service not initialized")
	}
	
	runtime.LogInfo(a.ctx, fmt.Sprintf("Obteniendo publicaciones del grupo %s...", groupID))
	
	err := a.facebookService.FetchAndStorePosts(groupID, limit)
	if err != nil {
		runtime.LogError(a.ctx, fmt.Sprintf("Error obteniendo publicaciones: %v", err))
		return err
	}
	
	runtime.LogInfo(a.ctx, fmt.Sprintf("Publicaciones del grupo %s obtenidas exitosamente", groupID))
	return nil
}

// FetchAllFacebookGroupsPosts obtiene publicaciones de todos los grupos habilitados
func (a *App) FetchAllFacebookGroupsPosts(limitPerGroup int) (map[string]string, error) {
	if a.facebookService == nil {
		return nil, fmt.Errorf("Facebook service not initialized")
	}
	
	runtime.LogInfo(a.ctx, "Obteniendo publicaciones de todos los grupos...")
	
	errors := a.facebookService.FetchAllGroupsPosts(limitPerGroup)
	
	// Convertir errores a strings para el frontend
	result := make(map[string]string)
	for groupID, err := range errors {
		if err != nil {
			result[groupID] = err.Error()
			runtime.LogError(a.ctx, fmt.Sprintf("Error en grupo %s: %v", groupID, err))
		} else {
			result[groupID] = "success"
			runtime.LogInfo(a.ctx, fmt.Sprintf("Grupo %s procesado exitosamente", groupID))
		}
	}
	
	return result, nil
}

// UpdateFacebookAccessToken actualiza el token de acceso de Facebook
func (a *App) UpdateFacebookAccessToken(accessToken string) error {
	if a.facebookService == nil {
		return fmt.Errorf("Facebook service not initialized")
	}
	
	a.facebookService.accessToken = accessToken
	runtime.LogInfo(a.ctx, "Token de Facebook actualizado")
	return nil
}

// GetDevAgentConfig obtiene la configuración del agente de desarrollo
func (a *App) GetDevAgentConfig() (DevAgentConfig, error) {
	if err := a.ensureConfigReady(); err != nil {
		return DevAgentConfig{}, err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return DevAgentConfig{}, err
		}
	}
	return a.devGroupAgent.GetConfig(), nil
}

// SetDevAgentConfig guarda la configuración del agente de desarrollo
func (a *App) SetDevAgentConfig(enabled bool, groupJID string, contextMessages int, triggerPrefix string, aiConfigID int) error {
	if err := a.ensureConfigReady(); err != nil {
		return err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return err
		}
	}
	cfg := DevAgentConfig{
		Enabled:         enabled,
		GroupJID:        strings.TrimSpace(groupJID),
		ContextMessages: contextMessages,
		TriggerPrefix:   triggerPrefix,
		AIConfigID:      aiConfigID,
	}
	if err := a.devGroupAgent.SetConfig(cfg); err != nil {
		return err
	}
	runtime.LogInfo(a.ctx, fmt.Sprintf("DevAgent guardado: enabled=%v group=%s", enabled, cfg.GroupJID))
	return nil
}

// EnableDevAgent activa o desactiva el agente sin cambiar el resto de la config
func (a *App) EnableDevAgent(enabled bool) error {
	if err := a.ensureConfigReady(); err != nil {
		return err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return err
		}
	}
	cfg := a.devGroupAgent.GetConfig()
	cfg.Enabled = enabled
	if err := a.devGroupAgent.SetConfig(cfg); err != nil {
		return err
	}
	runtime.LogInfo(a.ctx, fmt.Sprintf("DevAgent enabled=%v", enabled))
	return nil
}

// GetDevAgentReminders obtiene recordatorios del agente
func (a *App) GetDevAgentReminders(status string) ([]DevAgentReminder, error) {
	if err := a.ensureConfigReady(); err != nil {
		return nil, err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return nil, err
		}
	}
	return a.devGroupAgent.store.GetReminders(status, 50)
}

// MarkDevReminderDone marca un recordatorio como completado
func (a *App) MarkDevReminderDone(id int) error {
	if err := a.ensureConfigReady(); err != nil {
		return err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return err
		}
	}
	return a.devGroupAgent.store.MarkReminderDone(id)
}

// TestDevAgent simula una respuesta del agente sin enviar a WhatsApp
func (a *App) TestDevAgent(message string) (string, error) {
	if err := a.ensureConfigReady(); err != nil {
		return "", err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return "", err
		}
	}
	return a.devGroupAgent.TestMessage(message)
}

// DevAgentDiagnostics estado de diagnóstico del agente
type DevAgentDiagnostics struct {
	AgentReady      bool   `json:"agent_ready"`
	Enabled         bool   `json:"enabled"`
	GroupJID        string `json:"group_jid"`
	TriggerPrefix   string `json:"trigger_prefix"`
	AIConfigID      int    `json:"ai_config_id"`
	ActiveAIConfig  string `json:"active_ai_config"`
	WhatsAppConnected bool `json:"whatsapp_connected"`
}

// GetDevAgentDiagnostics devuelve estado del agente para depuración
func (a *App) GetDevAgentDiagnostics() (DevAgentDiagnostics, error) {
	diag := DevAgentDiagnostics{}
	if a.waService != nil {
		diag.WhatsAppConnected = a.waService.IsConnected()
	}
	if err := a.ensureConfigReady(); err != nil {
		return diag, err
	}
	if a.devGroupAgent == nil {
		if err := a.initDevGroupAgent(); err != nil {
			return diag, err
		}
	}
	diag.AgentReady = a.devGroupAgent != nil
	if a.devGroupAgent == nil {
		return diag, nil
	}
	cfg := a.devGroupAgent.GetConfig()
	diag.Enabled = cfg.Enabled
	diag.GroupJID = cfg.GroupJID
	diag.TriggerPrefix = cfg.TriggerPrefix
	diag.AIConfigID = cfg.AIConfigID
	if cfg.AIConfigID > 0 {
		if c, err := a.waService.aiConfigManager.GetConfigByID(cfg.AIConfigID); err == nil {
			diag.ActiveAIConfig = fmt.Sprintf("%s - %s (%s)", c.ProviderDisplay, c.ModelDisplay, c.Name)
		}
	} else if c, err := a.waService.aiConfigManager.GetActiveConfig(); err == nil {
		diag.ActiveAIConfig = fmt.Sprintf("%s - %s (%s)", c.ProviderDisplay, c.ModelDisplay, c.Name)
	}
	return diag, nil
}

// shutdown se llama cuando la app se cierra
func (a *App) shutdown(ctx context.Context) {
	runtime.LogInfo(ctx, "Cerrando aplicación...")
	if a.waService != nil {
		a.waService.Close()
	}
}
