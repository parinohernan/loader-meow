package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// WhatsAppService maneja la conexión y operaciones de WhatsApp
type WhatsAppService struct {
	client       *whatsmeow.Client
	messageStore *MessageStore
	logger       waLog.Logger
	qrChan       <-chan whatsmeow.QRChannelItem
	onMessage    func(ChatMessage)
	onQRCode     func(string)
	onConnected  func()
}

// ChatMessage representa un mensaje para la UI
type ChatMessage struct {
	ID        string    `json:"id"`
	ChatJID   string    `json:"chat_jid"`
	ChatName  string    `json:"chat_name"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	IsFromMe  bool      `json:"is_from_me"`
	MediaType string    `json:"media_type"`
	Filename  string    `json:"filename"`
}

// Chat representa un chat en la lista
type Chat struct {
	JID             string    `json:"jid"`
	Name            string    `json:"name"`
	LastMessageTime time.Time `json:"last_message_time"`
}

// MessageStore maneja el almacenamiento de mensajes
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore crea una nueva instancia del store de mensajes
func NewMessageStore() (*MessageStore, error) {
	if err := os.MkdirAll("store", 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %v", err)
	}

	db, err := sql.Open("sqlite3", "file:store/messages.db?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open message database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			last_message_time TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS messages (
			id TEXT,
			chat_jid TEXT,
			sender TEXT,
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return &MessageStore{db: db}, nil
}

// Close cierra la base de datos
func (store *MessageStore) Close() error {
	return store.db.Close()
}

// StoreChat guarda un chat en la base de datos
func (store *MessageStore) StoreChat(jid, name string, lastMessageTime time.Time) error {
	_, err := store.db.Exec(
		`INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)`,
		jid, name, lastMessageTime,
	)
	return err
}

// StoreMessage guarda un mensaje en la base de datos
func (store *MessageStore) StoreMessage(id, chatJID, sender, content string, timestamp time.Time, isFromMe bool,
	mediaType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	if content == "" && mediaType == "" {
		return nil
	}

	_, err := store.db.Exec(
		`INSERT OR REPLACE INTO messages 
		(id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, chatJID, sender, content, timestamp, isFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength,
	)
	return err
}

// GetChats obtiene todos los chats
func (store *MessageStore) GetChats() ([]Chat, error) {
	rows, err := store.db.Query(`SELECT jid, name, last_message_time FROM chats ORDER BY last_message_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var chat Chat
		err := rows.Scan(&chat.JID, &chat.Name, &chat.LastMessageTime)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}

	return chats, nil
}

// GetMessages obtiene mensajes de un chat
func (store *MessageStore) GetMessages(chatJID string, limit int) ([]ChatMessage, error) {
	rows, err := store.db.Query(
		`SELECT id, sender, content, timestamp, is_from_me, media_type, filename 
		FROM messages WHERE chat_jid = ? ORDER BY timestamp DESC LIMIT ?`,
		chatJID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(&msg.ID, &msg.Sender, &msg.Content, &msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename)
		if err != nil {
			return nil, err
		}
		msg.ChatJID = chatJID
		messages = append(messages, msg)
	}

	return messages, nil
}

// NewWhatsAppService crea una nueva instancia del servicio
func NewWhatsAppService() (*WhatsAppService, error) {
	logger := waLog.Stdout("WhatsApp", "INFO", true)
	dbLog := waLog.Stdout("Database", "INFO", true)

	if err := os.MkdirAll("store", 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %v", err)
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store/whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			deviceStore = container.NewDevice()
		} else {
			return nil, fmt.Errorf("failed to get device: %v", err)
		}
	}

	client := whatsmeow.NewClient(deviceStore, logger)
	if client == nil {
		return nil, fmt.Errorf("failed to create WhatsApp client")
	}

	messageStore, err := NewMessageStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize message store: %v", err)
	}

	service := &WhatsAppService{
		client:       client,
		messageStore: messageStore,
		logger:       logger,
	}

	// Configurar event handler
	client.AddEventHandler(func(evt interface{}) {
		service.eventHandler(evt)
	})

	return service, nil
}

// extractTextContent extrae el contenido de texto de un mensaje
func extractTextContent(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}

	if text := msg.GetConversation(); text != "" {
		return text
	} else if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetText()
	}

	return ""
}

// extractMediaInfo extrae información de medios de un mensaje
func extractMediaInfo(msg *waProto.Message) (mediaType string, filename string, url string, mediaKey []byte, fileSHA256 []byte, fileEncSHA256 []byte, fileLength uint64) {
	if msg == nil {
		return "", "", "", nil, nil, nil, 0
	}

	if img := msg.GetImageMessage(); img != nil {
		return "image", "image_" + time.Now().Format("20060102_150405") + ".jpg",
			img.GetURL(), img.GetMediaKey(), img.GetFileSHA256(), img.GetFileEncSHA256(), img.GetFileLength()
	}

	if vid := msg.GetVideoMessage(); vid != nil {
		return "video", "video_" + time.Now().Format("20060102_150405") + ".mp4",
			vid.GetURL(), vid.GetMediaKey(), vid.GetFileSHA256(), vid.GetFileEncSHA256(), vid.GetFileLength()
	}

	if aud := msg.GetAudioMessage(); aud != nil {
		return "audio", "audio_" + time.Now().Format("20060102_150405") + ".ogg",
			aud.GetURL(), aud.GetMediaKey(), aud.GetFileSHA256(), aud.GetFileEncSHA256(), aud.GetFileLength()
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		filename := doc.GetFileName()
		if filename == "" {
			filename = "document_" + time.Now().Format("20060102_150405")
		}
		return "document", filename,
			doc.GetURL(), doc.GetMediaKey(), doc.GetFileSHA256(), doc.GetFileEncSHA256(), doc.GetFileLength()
	}

	return "", "", "", nil, nil, nil, 0
}

// eventHandler maneja los eventos de WhatsApp
func (s *WhatsAppService) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		s.handleMessage(v)
	case *events.Connected:
		s.logger.Infof("Connected to WhatsApp")
		if s.onConnected != nil {
			s.onConnected()
		}
	case *events.LoggedOut:
		s.logger.Warnf("Device logged out")
	}
}

// handleMessage procesa un mensaje recibido
func (s *WhatsAppService) handleMessage(msg *events.Message) {
	chatJID := msg.Info.Chat.String()
	
	// Extraer número de teléfono del remitente
	var senderPhone string
	var senderName string
	
	if msg.Info.IsFromMe {
		// Si es nuestro mensaje
		senderPhone = s.client.Store.ID.User
		senderName = "Yo"
	} else {
		// Para mensajes entrantes
		// Primero intentamos obtener el número de teléfono válido
		senderPhone = s.extractValidPhone(msg)
		
		// Intentar obtener el nombre del contacto
		senderName = s.getSenderName(msg)
		
		// Si no pudimos obtener un nombre, usar el teléfono
		if senderName == "" {
			senderName = senderPhone
		}
	}
	
	// Log para debugging
	s.logger.Infof("Mensaje en grupo: Chat=%s, Sender.User=%s, Sender=%s, Phone extraído=%s, Nombre=%s",
		msg.Info.Chat.String(), msg.Info.Sender.User, msg.Info.Sender.String(), senderPhone, senderName)
	
	// Obtener nombre del chat
	name := s.getChatName(msg.Info.Chat, chatJID, senderPhone)
	
	// Actualizar chat en la base de datos
	err := s.messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
	if err != nil {
		s.logger.Warnf("Failed to store chat: %v", err)
	}

	// Extraer contenido
	content := extractTextContent(msg.Message)
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	if content == "" && mediaType == "" {
		return
	}

	// Guardar mensaje
	err = s.messageStore.StoreMessage(
		msg.Info.ID,
		chatJID,
		senderPhone,
		content,
		msg.Info.Timestamp,
		msg.Info.IsFromMe,
		mediaType,
		filename,
		url,
		mediaKey,
		fileSHA256,
		fileEncSHA256,
		fileLength,
	)

	if err != nil {
		s.logger.Warnf("Failed to store message: %v", err)
		return
	}

	// Notificar a la UI
	if s.onMessage != nil {
		s.onMessage(ChatMessage{
			ID:        msg.Info.ID,
			ChatJID:   chatJID,
			ChatName:  name,
			Sender:    senderPhone,
			Content:   content,
			Timestamp: msg.Info.Timestamp,
			IsFromMe:  msg.Info.IsFromMe,
			MediaType: mediaType,
			Filename:  filename,
		})
	}
}

// extractValidPhone extrae un número de teléfono válido del mensaje
// Maneja correctamente grupos y chats individuales
func (s *WhatsAppService) extractValidPhone(msg *events.Message) string {
	senderJID := msg.Info.Sender
	userPart := senderJID.User
	
	// En WhatsApp, los números de teléfono válidos:
	// - Tienen entre 7 y 15 dígitos
	// - Empiezan con el código de país (1-3 dígitos)
	// - No empiezan con 0
	
	// Si el User parece un número de teléfono válido
	if len(userPart) >= 7 && len(userPart) <= 15 {
		// Verificar que no sea un ID de grupo (generalmente más largo o empieza con códigos específicos)
		// Los códigos de país válidos son de 1 a 3 dígitos
		firstDigit := string(userPart[0])
		
		// Si empieza con 0 o tiene más de 15 dígitos, probablemente no es un número válido
		if firstDigit != "0" && len(userPart) <= 15 {
			// Validación adicional: verificar el código de país
			// Códigos de país comunes en América: 1, 52, 54, 55, 56, 57, 58, 591, 593, etc.
			if len(userPart) >= 10 {
				return userPart
			}
		}
	}
	
	// Si no es válido, intentar obtener de otra forma
	// En grupos, a veces el Server da pistas
	if senderJID.Server == "s.whatsapp.net" {
		// Este es el formato normal de usuarios
		return userPart
	}
	
	// Como último recurso, devolver lo que tenemos
	return userPart
}

// getSenderName intenta obtener el nombre del contacto
func (s *WhatsAppService) getSenderName(msg *events.Message) string {
	senderJID := msg.Info.Sender
	
	// Intentar obtener el nombre del contacto guardado
	contact, err := s.client.Store.Contacts.GetContact(context.Background(), senderJID)
	if err == nil {
		if contact.FullName != "" {
			return contact.FullName
		}
		if contact.FirstName != "" {
			return contact.FirstName
		}
		if contact.PushName != "" {
			return contact.PushName
		}
	}
	
	// Si no hay contacto guardado, intentar obtener el PushName del mensaje
	if msg.Info.PushName != "" {
		return msg.Info.PushName
	}
	
	return ""
}

// getChatName obtiene el nombre de un chat
func (s *WhatsAppService) getChatName(jid types.JID, chatJID string, sender string) string {
	if jid.User == "status" {
		return "Status Updates"
	}

	// Verificar si ya existe en la BD
	var existingName string
	err := s.messageStore.db.QueryRow(`SELECT name FROM chats WHERE jid = ?`, chatJID).Scan(&existingName)
	if err == nil && existingName != "" {
		return existingName
	}

	var name string
	if jid.Server == "g.us" {
		// Grupo
		groupInfo, err := s.client.GetGroupInfo(jid)
		if err == nil && groupInfo.Name != "" {
			name = groupInfo.Name
		} else {
			name = fmt.Sprintf("Group %s", jid.User)
		}
	} else {
		// Contacto individual
		contact, err := s.client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.FullName != "" {
			name = contact.FullName
		} else if sender != "" {
			name = sender
		} else {
			name = jid.User
		}
	}
	return name
}

// Connect conecta al cliente de WhatsApp
func (s *WhatsAppService) Connect() error {
	if s.client.Store.ID == nil {
		// Necesita escanear QR
		qrChan, _ := s.client.GetQRChannel(context.Background())
		s.qrChan = qrChan
		
		err := s.client.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		
		return nil
	}
	
	// Ya está autenticado, solo conectar
	err := s.client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	
	return nil
}

// IsConnected verifica si el cliente está conectado
func (s *WhatsAppService) IsConnected() bool {
	return s.client.IsConnected()
}

// IsLoggedIn verifica si el cliente está autenticado
func (s *WhatsAppService) IsLoggedIn() bool {
	return s.client.Store.ID != nil
}

// Disconnect desconecta el cliente
func (s *WhatsAppService) Disconnect() {
	s.client.Disconnect()
}

// Close cierra el servicio
func (s *WhatsAppService) Close() error {
	s.Disconnect()
	return s.messageStore.Close()
}

// SendMessage envía un mensaje
func (s *WhatsAppService) SendMessage(recipient, message string) error {
	if !s.IsConnected() {
		return fmt.Errorf("not connected to WhatsApp")
	}

	recipientJID, err := types.ParseJID(recipient)
	if err != nil {
		// Intentar como número de teléfono
		recipientJID = types.JID{
			User:   recipient,
			Server: "s.whatsapp.net",
		}
	}

	msg := &waProto.Message{
		Conversation: &message,
	}

	_, err = s.client.SendMessage(context.Background(), recipientJID, msg)
	return err
}


