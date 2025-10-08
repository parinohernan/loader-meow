package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	waService *WhatsAppService
	qrCode    string
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

	// Configurar callbacks
	waService.onMessage = func(msg ChatMessage) {
		// Formatear el remitente para mostrar el teléfono
		senderInfo := fmt.Sprintf("+%s", msg.Sender)
		runtime.LogInfo(a.ctx, fmt.Sprintf("Nuevo mensaje de %s en %s: %s", senderInfo, msg.ChatName, msg.Content))
		runtime.EventsEmit(a.ctx, "new-message", msg)
	}

	waService.onConnected = func() {
		runtime.LogInfo(a.ctx, "Conectado a WhatsApp!")
		runtime.EventsEmit(a.ctx, "connected")
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
