package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.mau.fi/whatsmeow/types"
)

// App struct
type App struct {
	ctx              context.Context
	waService        *WhatsAppService
	messageProcessor *MessageProcessor
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
}

// NewApp crea una nueva instancia de App
func NewApp() *App {
	return &App{}
}

// startup se llama cuando la app inicia
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.LogInfo(ctx, "Aplicación iniciada")
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
		runtime.LogInfo(a.ctx, fmt.Sprintf("Nuevo mensaje de %s en %s: %s", senderInfo, msg.ChatName, msg.Content))
		runtime.EventsEmit(a.ctx, "new-message", msg)
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

	// Si necesita QR, escuchar el canal
	if !a.waService.IsLoggedIn() {
		go func() {
			for evt := range a.waService.qrChan {
				if evt.Event == "code" {
					runtime.LogInfo(a.ctx, "QR Code recibido")
					runtime.EventsEmit(a.ctx, "qr-code", evt.Code)
				} else if evt.Event == "success" {
					runtime.LogInfo(a.ctx, "Autenticación exitosa!")
					runtime.EventsEmit(a.ctx, "authenticated")
					break
				}
			}
		}()
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

// DeleteMessage elimina un mensaje de la base de datos
func (a *App) DeleteMessage(messageID, chatJID string) error {
	if a.waService == nil {
		return fmt.Errorf("WhatsApp service not initialized")
	}
	
	return a.waService.messageStore.DeleteMessage(messageID, chatJID)
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

// ProcessSingleMessage procesa un solo mensaje por ID
func (a *App) ProcessSingleMessage(messageID, chatJID string) (ProcessingResult, error) {
	if a.messageProcessor == nil {
		return ProcessingResult{}, fmt.Errorf("message processor not initialized")
	}
	
	return a.messageProcessor.ProcessSingleMessage(messageID, chatJID)
}




// Disconnect desconecta WhatsApp
func (a *App) Disconnect() {
	if a.waService != nil {
		a.waService.Disconnect()
	}
}

// shutdown se llama cuando la app se cierra
func (a *App) shutdown(ctx context.Context) {
	runtime.LogInfo(ctx, "Cerrando aplicación...")
	if a.waService != nil {
		a.waService.Close()
	}
}
