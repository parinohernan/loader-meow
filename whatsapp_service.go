package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	whatsstore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// WhatsAppService maneja la conexión y operaciones de WhatsApp
type WhatsAppService struct {
	client                   *whatsmeow.Client
	messageStore             *MessageStore
	messageProcessor         *MessageProcessor
	aiConfigManager          *AIConfigManager
	systemConfigManager      *SystemConfigManager
	logger                   waLog.Logger
	qrChan                   <-chan whatsmeow.QRChannelItem
	qrCancel                 context.CancelFunc
	qrMu                     sync.Mutex
	onMessage                func(ChatMessage)
	onQRCode                 func(string)
	onAuthenticated          func()
	onConnected              func()
	onLoggedOut              func(string)
	onPhoneAssociationNeeded func(PhoneAssociationRequest)
}

// ChatMessage representa un mensaje para la UI
type ChatMessage struct {
	ID          string    `json:"id"`
	ChatJID     string    `json:"chat_jid"`
	ChatName    string    `json:"chat_name"`
	SenderPhone string    `json:"sender_phone"` // Número de teléfono o LID si no disponible
	SenderName  string    `json:"sender_name"`  // PushName del contacto
	Content     string    `json:"content"`
	Timestamp   time.Time `json:"timestamp"`
	IsFromMe    bool      `json:"is_from_me"`
	MediaType   string    `json:"media_type"`
	Filename    string    `json:"filename"`
	Processed   bool      `json:"processed"`
}

// ProcessableMessage representa un mensaje que puede ser procesado por IA
type ProcessableMessage struct {
	ChatMessage
	RealPhone string `json:"real_phone"` // Teléfono real asociado
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
	// Obtener configuración de la base de datos
	config := GetDatabaseConfig()

	// Conectar a MySQL
	db, err := sql.Open("mysql", config.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL database: %v", err)
	}

	// Verificar conexión
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL database: %v", err)
	}

	// Configurar pool de conexiones para MySQL
	db.SetMaxOpenConns(25) // MySQL puede manejar múltiples conexiones
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Crear tablas (compatible con MariaDB y MySQL)
	tables := []string{
		`CREATE TABLE IF NOT EXISTS chats (
			jid VARCHAR(255) PRIMARY KEY,
			name VARCHAR(500),
			last_message_time TIMESTAMP NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS messages (
			id VARCHAR(255),
			chat_jid VARCHAR(255),
			sender_phone VARCHAR(100),
			sender_name VARCHAR(500),
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN DEFAULT FALSE,
			media_type VARCHAR(100),
			filename VARCHAR(500),
			url VARCHAR(1000),
			media_key LONGBLOB,
			file_sha256 LONGBLOB,
			file_enc_sha256 LONGBLOB,
			file_length BIGINT,
			processed BOOLEAN DEFAULT FALSE,
			processing_attempts INT DEFAULT 0,
			last_processing_error TEXT,
			last_processing_attempt TIMESTAMP NULL,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS phone_associations (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sender_phone VARCHAR(100) NOT NULL UNIQUE,
			real_phone VARCHAR(50),
			display_name VARCHAR(500),
			nombre VARCHAR(255) DEFAULT '',
			perfil ENUM('desconocido', 'loader', 'camionero') DEFAULT 'desconocido',
			confianza INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_sender_phone (sender_phone),
			INDEX idx_real_phone (real_phone),
			INDEX idx_perfil (perfil),
			INDEX idx_confianza (confianza)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS ai_processing_results (
			id INT AUTO_INCREMENT PRIMARY KEY,
			message_id VARCHAR(255),
			chat_jid VARCHAR(255),
			content TEXT,
			sender_phone VARCHAR(100),
			real_phone VARCHAR(50),
			ai_response TEXT,
			status VARCHAR(50),
			error_message TEXT,
			supabase_ids TEXT,
			processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_message_chat (message_id, chat_jid),
			INDEX idx_status (status),
			INDEX idx_processed_at (processed_at),
			FOREIGN KEY (message_id, chat_jid) REFERENCES messages(id, chat_jid) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	// Crear cada tabla individualmente
	for _, query := range tables {
		if _, err = db.Exec(query); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create table: %v", err)
		}
	}

	// Crear índices (compatibles con MariaDB - solo si no existen)
	indices := []string{
		`CREATE INDEX IF NOT EXISTS idx_messages_duplicate_phone ON messages(sender_phone, content(100), timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_sender_name ON messages(sender_name, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_processed ON messages(processed, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat_timestamp ON messages(chat_jid, timestamp)`,
	}

	// Crear cada índice, ignorar errores si ya existe
	for _, query := range indices {
		_, err = db.Exec(query)
		// Ignorar error si el índice ya existe
		if err != nil && !isIndexExistsError(err) {
			db.Close()
			return nil, fmt.Errorf("failed to create index: %v", err)
		}
	}

	// Ejecutar migraciones para agregar columnas nuevas si no existen
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &MessageStore{db: db}, nil
}

// InitAIConfigTables inicializa las tablas de configuración de IA
func (store *MessageStore) InitAIConfigTables() error {
	// Leer el SQL desde el archivo de migración
	sqlFile := "migrations/create_ai_config_tables.sql"
	sqlBytes, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("failed to read AI config migration file: %v", err)
	}

	sqlContent := string(sqlBytes)

	// Dividir en statements individuales
	statements := splitSQLStatements(sqlContent)

	// Ejecutar cada statement
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		_, err := store.db.Exec(stmt)
		if err != nil {
			// Log pero no fallar si la tabla ya existe
			if !strings.Contains(err.Error(), "already exists") &&
				!strings.Contains(err.Error(), "Duplicate") {
				return fmt.Errorf("failed to execute AI config migration: %v\nStatement: %s", err, stmt)
			}
		}
	}

	return nil
}

// splitSQLStatements divide un archivo SQL en statements individuales
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder

	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Ignorar comentarios
		if strings.HasPrefix(trimmed, "--") {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		// Si termina con ; , es el final de un statement
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	// Agregar el último statement si hay uno
	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}

// isIndexExistsError verifica si el error es porque el índice ya existe
func isIndexExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// MariaDB/MySQL error codes para índice duplicado
	return strings.Contains(errMsg, "Duplicate key name") ||
		strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "Error 1061")
}

// runMigrations ejecuta migraciones de base de datos
func runMigrations(db *sql.DB) error {
	// Verificar si las columnas ya existen
	columns := []struct {
		name       string
		definition string
	}{
		{"processing_attempts", "INT DEFAULT 0"},
		{"last_processing_error", "TEXT"},
		{"last_processing_attempt", "TIMESTAMP NULL"},
	}

	for _, col := range columns {
		// Intentar agregar la columna
		alterQuery := fmt.Sprintf("ALTER TABLE messages ADD COLUMN %s %s", col.name, col.definition)
		_, err := db.Exec(alterQuery)

		// Ignorar error si la columna ya existe
		if err != nil && !isColumnExistsError(err) {
			return fmt.Errorf("failed to add column %s: %v", col.name, err)
		}
	}

	return nil
}

// isColumnExistsError verifica si el error es porque la columna ya existe
func isColumnExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// MariaDB/MySQL error codes para columna duplicada
	return strings.Contains(errMsg, "Duplicate column name") ||
		strings.Contains(errMsg, "column already exists") ||
		strings.Contains(errMsg, "Error 1060")
}

// Close cierra la base de datos
func (store *MessageStore) Close() error {
	return store.db.Close()
}

// StoreChat guarda un chat en la base de datos
func (store *MessageStore) StoreChat(jid, name string, lastMessageTime time.Time) error {
	_, err := store.db.Exec(
		`INSERT INTO chats (jid, name, last_message_time) VALUES (?, ?, ?) 
		 ON DUPLICATE KEY UPDATE name = VALUES(name), last_message_time = VALUES(last_message_time)`,
		jid, name, lastMessageTime,
	)
	return err
}

// StoreMessage guarda un mensaje en la base de datos solo si no existe un duplicado
// Un duplicado se define como un mensaje con el mismo sender_phone, content en las últimas 48 horas
func (store *MessageStore) StoreMessage(id, chatJID, senderPhone, senderName, content string, timestamp time.Time, isFromMe bool,
	mediaType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	if content == "" && mediaType == "" {
		return nil
	}

	// Verificar si ya existe un mensaje duplicado (mismo sender_phone + content en las últimas 24 horas)
	// GLOBAL: No importa en qué grupo fue enviado
	var exists int
	err := store.db.QueryRow(`
		SELECT COUNT(*) FROM messages 
		WHERE sender_phone = ? 
		AND content = ?
		AND timestamp >= DATE_SUB(?, INTERVAL 24 HOUR)
		AND timestamp <= DATE_ADD(?, INTERVAL 24 HOUR)
	`, senderPhone, content, timestamp, timestamp).Scan(&exists)

	if err != nil {
		return fmt.Errorf("error verificando duplicado: %v", err)
	}

	if exists > 0 {
		// Mensaje duplicado detectado, no insertar
		fmt.Printf("⚠️ Se encontró mensaje duplicado - Sender: %s, Content: %.50s...\n", senderPhone, content)
		return nil
	}

	// Insertar el mensaje nuevo con processed = false por defecto
	_, err = store.db.Exec(
		`INSERT INTO messages 
		(id, chat_jid, sender_phone, sender_name, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length, processed) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		id, chatJID, senderPhone, senderName, content, timestamp, isFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength,
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
		`SELECT id, sender_phone, sender_name, content, timestamp, is_from_me, media_type, filename, processed 
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
		err := rows.Scan(&msg.ID, &msg.SenderPhone, &msg.SenderName, &msg.Content, &msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename, &msg.Processed)
		if err != nil {
			return nil, err
		}
		msg.ChatJID = chatJID
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetMessagesBySenderPhone obtiene mensajes de un remitente específico
func (store *MessageStore) GetMessagesBySenderPhone(senderPhone string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	
	rows, err := store.db.Query(
		`SELECT id, chat_jid, sender_phone, sender_name, content, timestamp, is_from_me, media_type, filename, processed 
		FROM messages 
		WHERE sender_phone = ? 
		ORDER BY timestamp DESC 
		LIMIT ?`,
		senderPhone, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(&msg.ID, &msg.ChatJID, &msg.SenderPhone, &msg.SenderName, &msg.Content, &msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename, &msg.Processed)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetUnprocessedMessages obtiene todos los mensajes no procesados
func (store *MessageStore) GetUnprocessedMessages(limit int) ([]ChatMessage, error) {
	rows, err := store.db.Query(
		`SELECT id, chat_jid, sender_phone, sender_name, content, timestamp, is_from_me, media_type, filename, processed 
		FROM messages 
		WHERE processed = 0 
		ORDER BY timestamp ASC 
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(&msg.ID, &msg.ChatJID, &msg.SenderPhone, &msg.SenderName, &msg.Content, &msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename, &msg.Processed)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetMessageByID obtiene un mensaje específico por ID y chatJID
// Permite obtener cualquier mensaje, sin importar su estado (procesado, con errores, etc.)
func (store *MessageStore) GetMessageByID(messageID, chatJID string) (*ProcessableMessage, error) {
	query := `
		SELECT m.id, m.chat_jid, m.sender_phone, m.sender_name, m.content, 
		       m.timestamp, m.is_from_me, m.media_type, m.filename, m.processed,
		       pa.real_phone
		FROM messages m
		INNER JOIN phone_associations pa ON m.sender_phone = pa.sender_phone
		WHERE m.id = ? 
		  AND m.chat_jid = ?
		  AND m.content IS NOT NULL 
		  AND m.content != ''
		  AND pa.real_phone IS NOT NULL
		  AND pa.real_phone != ''
		LIMIT 1
	`

	var msg ProcessableMessage
	err := store.db.QueryRow(query, messageID, chatJID).Scan(
		&msg.ID, &msg.ChatJID, &msg.SenderPhone, &msg.SenderName, &msg.Content,
		&msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename, &msg.Processed,
		&msg.RealPhone,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Mensaje no encontrado o sin asociación de teléfono
	}
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

// GetProcessableMessages obtiene mensajes que pueden ser procesados por IA
func (store *MessageStore) GetProcessableMessages(limit int) ([]ProcessableMessage, error) {
	query := `
		SELECT m.id, m.chat_jid, m.sender_phone, m.sender_name, m.content, 
		       m.timestamp, m.is_from_me, m.media_type, m.filename, m.processed,
		       pa.real_phone
		FROM messages m
		INNER JOIN phone_associations pa ON m.sender_phone = pa.sender_phone
		WHERE m.processed = 0 
		  AND m.content IS NOT NULL 
		  AND m.content != ''
		  AND pa.real_phone IS NOT NULL
		  AND pa.real_phone != ''
		  AND (m.processing_attempts < 3 OR m.processing_attempts IS NULL)
		  -- Filtrar mensajes de texto cortos (menos de 20 caracteres) para descongestionar la API
		  -- Solo aplicar este filtro a mensajes de texto (sin media_type o media_type vacío)
		  AND (
		       (m.media_type IS NOT NULL AND m.media_type != '') 
		       OR 
		       (m.media_type IS NULL OR m.media_type = '') AND LENGTH(m.content) >= 20
		  )
		ORDER BY m.timestamp ASC
		LIMIT ?
	`

	rows, err := store.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ProcessableMessage
	for rows.Next() {
		var msg ProcessableMessage
		err := rows.Scan(&msg.ID, &msg.ChatJID, &msg.SenderPhone, &msg.SenderName, &msg.Content,
			&msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename, &msg.Processed, &msg.RealPhone)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetProcessableMessagesCount obtiene el conteo de mensajes procesables
func (store *MessageStore) GetProcessableMessagesCount() (int, error) {
	query := `
		SELECT COUNT(*)
		FROM messages m
		INNER JOIN phone_associations pa ON m.sender_phone = pa.sender_phone
		WHERE m.processed = 0 
		  AND m.content IS NOT NULL 
		  AND m.content != ''
		  AND pa.real_phone IS NOT NULL
		  AND pa.real_phone != ''
		  -- Filtrar mensajes de texto cortos (menos de 20 caracteres) para descongestionar la API
		  -- Solo aplicar este filtro a mensajes de texto (sin media_type o media_type vacío)
		  AND (
		       (m.media_type IS NOT NULL AND m.media_type != '') 
		       OR 
		       (m.media_type IS NULL OR m.media_type = '') AND LENGTH(m.content) >= 20
		  )
	`

	var count int
	err := store.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetUnprocessedMessagesWithRealPhone obtiene mensajes sin procesar con teléfono real
func (store *MessageStore) GetUnprocessedMessagesWithRealPhone(limit int) ([]ProcessableMessage, error) {
	query := `
		SELECT m.id, m.chat_jid, m.sender_phone, m.sender_name, m.content, 
		       m.timestamp, m.is_from_me, m.media_type, m.filename, m.processed,
		       pa.real_phone, COALESCE(m.processing_attempts, 0) as processing_attempts
		FROM messages m
		INNER JOIN phone_associations pa ON m.sender_phone = pa.sender_phone
		WHERE m.processed = 0 
		  AND m.content IS NOT NULL 
		  AND m.content != ''
		  AND pa.real_phone IS NOT NULL
		  AND pa.real_phone != ''
		  -- Filtrar mensajes de texto cortos (menos de 20 caracteres) para descongestionar la API
		  -- Solo aplicar este filtro a mensajes de texto (sin media_type o media_type vacío)
		  AND (
		       (m.media_type IS NOT NULL AND m.media_type != '') 
		       OR 
		       (m.media_type IS NULL OR m.media_type = '') AND LENGTH(m.content) >= 20
		  )
		ORDER BY m.timestamp DESC
		LIMIT ?
	`

	rows, err := store.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ProcessableMessage
	for rows.Next() {
		var msg ProcessableMessage
		var attempts int

		err := rows.Scan(&msg.ID, &msg.ChatJID, &msg.SenderPhone, &msg.SenderName, &msg.Content,
			&msg.Timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename, &msg.Processed,
			&msg.RealPhone, &attempts)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// MarkMessageAsProcessed marca un mensaje como procesado
func (store *MessageStore) MarkMessageAsProcessed(messageID, chatJID string) error {
	_, err := store.db.Exec(
		`UPDATE messages SET processed = 1 WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	)
	return err
}

// IncrementProcessingAttempt incrementa el contador de intentos de procesamiento
func (store *MessageStore) IncrementProcessingAttempt(messageID, chatJID, errorMsg string) error {
	_, err := store.db.Exec(
		`UPDATE messages 
		 SET processing_attempts = processing_attempts + 1,
		     last_processing_error = ?,
		     last_processing_attempt = NOW()
		 WHERE id = ? AND chat_jid = ?`,
		errorMsg, messageID, chatJID,
	)
	return err
}

// MarkMessageAsFailedAfterRetries marca un mensaje como procesado después de múltiples fallos
func (store *MessageStore) MarkMessageAsFailedAfterRetries(messageID, chatJID string) error {
	_, err := store.db.Exec(
		`UPDATE messages 
		 SET processed = 1,
		     last_processing_error = 'Demasiados intentos fallidos'
		 WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	)
	return err
}

// ResetProcessingAttempts resetea el contador de intentos para reprocesar un mensaje
func (store *MessageStore) ResetProcessingAttempts(messageID, chatJID string) error {
	_, err := store.db.Exec(
		`UPDATE messages 
		 SET processing_attempts = 0,
		     processed = 0,
		     last_processing_error = NULL,
		     last_processing_attempt = NULL
		 WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	)
	return err
}

// DeleteMessage elimina un mensaje de la base de datos
func (store *MessageStore) DeleteMessage(messageID, chatJID string) error {
	_, err := store.db.Exec(
		`DELETE FROM messages WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	)
	return err
}

// DeleteMessagesBySenderPhone elimina todos los mensajes de un remitente específico
func (store *MessageStore) DeleteMessagesBySenderPhone(senderPhone string) error {
	_, err := store.db.Exec(
		`DELETE FROM messages WHERE sender_phone = ?`,
		senderPhone,
	)
	return err
}

// UpdateMessageContent actualiza el contenido de un mensaje
func (store *MessageStore) UpdateMessageContent(messageID, chatJID, newContent string) error {
	_, err := store.db.Exec(
		`UPDATE messages 
		 SET content = ?,
		     processing_attempts = 0,
		     processed = 0,
		     last_processing_error = NULL
		 WHERE id = ? AND chat_jid = ?`,
		newContent, messageID, chatJID,
	)
	return err
}

// MarkMessagesAsProcessed marca múltiples mensajes como procesados (útil para lotes)
func (store *MessageStore) MarkMessagesAsProcessed(messageIDs []string, chatJID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE messages SET processed = 1 WHERE id = ? AND chat_jid = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msgID := range messageIDs {
		_, err := stmt.Exec(msgID, chatJID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMessageStats obtiene estadísticas de mensajes
func (store *MessageStore) GetMessageStats() (total, processed, unprocessed int, err error) {
	err = store.db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN processed = 1 THEN 1 ELSE 0 END) as processed,
			SUM(CASE WHEN processed = 0 THEN 1 ELSE 0 END) as unprocessed
		FROM messages
	`).Scan(&total, &processed, &unprocessed)
	return
}

// UpdatePhoneProfiling actualiza el perfil y confianza de un número de teléfono
func (store *MessageStore) UpdatePhoneProfiling(realPhone string, isValidLoad bool) error {
	// Determinar el cambio en confianza
	// +1 si envió una carga válida (es loader)
	// -1 si envió mensaje de camionero buscando carga (array vacío)
	confianzaDelta := 1
	if !isValidLoad {
		confianzaDelta = -1
	}

	// Actualizar confianza
	_, err := store.db.Exec(`
		UPDATE phone_associations 
		SET confianza = confianza + ?,
		    updated_at = NOW()
		WHERE real_phone = ?
	`, confianzaDelta, realPhone)

	if err != nil {
		return fmt.Errorf("failed to update trust score: %v", err)
	}

	// Actualizar perfil basado en el score de confianza
	_, err = store.db.Exec(`
		UPDATE phone_associations 
		SET perfil = CASE 
			WHEN confianza > 0 THEN 'loader'
			WHEN confianza < 0 THEN 'camionero'
			ELSE 'desconocido'
		END,
		updated_at = NOW()
		WHERE real_phone = ?
	`, realPhone)

	if err != nil {
		return fmt.Errorf("failed to update profile: %v", err)
	}

	return nil
}

// NewWhatsAppService crea una nueva instancia del servicio
func NewWhatsAppService() (*WhatsAppService, error) {
	whatsstore.SetOSInfo("Carica Loader", [3]uint32{1, 0, 0})

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

	// Inicializar tablas de configuración de IA
	if err := messageStore.InitAIConfigTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize AI config tables: %v", err)
	}

	// Inicializar AI config manager
	aiConfigManager := NewAIConfigManager(messageStore.db)

	// Inicializar System config manager
	systemConfigManager := NewSystemConfigManager(messageStore.db)

	// Inicializar API keys manager (mantener por compatibilidad)
	keysManager, err := NewAPIKeysManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create API keys manager: %v", err)
	}

	// Inicializar message processor con el nuevo sistema
	messageProcessor, err := NewMessageProcessor(messageStore, logger, keysManager, aiConfigManager, systemConfigManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create message processor: %v", err)
	}

	service := &WhatsAppService{
		client:              client,
		messageStore:        messageStore,
		messageProcessor:    messageProcessor,
		aiConfigManager:     aiConfigManager,
		systemConfigManager: systemConfigManager,
		logger:              logger,
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
		s.handleLoggedOut(v)
	}
}

// handleLoggedOut se ejecuta cuando el servidor desconecta definitivamente el dispositivo
func (s *WhatsAppService) handleLoggedOut(evt *events.LoggedOut) {
	reason := evt.Reason.String()
	if reason == "" {
		reason = "sin razón reportada"
	}

	if evt.OnConnect {
		s.logger.Warnf("🔌 Logout recibido durante la conexión: %s", reason)
	} else {
		s.logger.Warnf("🔌 Dispositivo removido por el servidor: %s", reason)
	}

	s.resetQRChannel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.client.Store.Delete(ctx); err != nil {
		s.logger.Warnf("No se pudo limpiar la sesión tras el logout: %v", err)
	}

	s.client.Disconnect()

	if s.onLoggedOut != nil {
		s.onLoggedOut(reason)
	} else {
		go func() {
			time.Sleep(2 * time.Second)
			if err := s.Connect(); err != nil {
				s.logger.Errorf("Error al reconectar después del logout: %v", err)
			}
		}()
	}
}

// resetQRChannel detiene la escucha del canal de QR actual
func (s *WhatsAppService) resetQRChannel() {
	s.qrMu.Lock()
	defer s.qrMu.Unlock()

	if s.qrCancel != nil {
		s.qrCancel()
		s.qrCancel = nil
	}
	s.qrChan = nil
}

// initQRChannel crea un nuevo canal de QR y comienza a reenviar eventos a la UI
func (s *WhatsAppService) initQRChannel() error {
	s.qrMu.Lock()
	defer s.qrMu.Unlock()

	if s.qrCancel != nil {
		s.qrCancel()
		s.qrCancel = nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	qrChan, err := s.client.GetQRChannel(ctx)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get QR channel: %w", err)
	}

	s.qrChan = qrChan
	s.qrCancel = cancel

	go s.consumeQRChannel(qrChan)
	return nil
}

// consumeQRChannel reenvía eventos del canal QR hacia los callbacks configurados
func (s *WhatsAppService) consumeQRChannel(qrChan <-chan whatsmeow.QRChannelItem) {
	if qrChan == nil {
		return
	}

	for evt := range qrChan {
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			s.logger.Infof("🔐 QR recibido; expira en %s", evt.Timeout)
			if s.onQRCode != nil && evt.Code != "" {
				s.onQRCode(evt.Code)
			}
		case whatsmeow.QRChannelEventError:
			s.logger.Errorf("❌ Error en emparejamiento: %v", evt.Error)
		case "success":
			s.logger.Infof("✅ QR escaneado correctamente, esperando 'connected'")
			if s.onAuthenticated != nil {
				s.onAuthenticated()
			}
		case "timeout":
			s.logger.Warnf("⌛ QR expirado, esperando renovación del servidor")
		default:
			s.logger.Warnf("Evento de QR no manejado: %s", evt.Event)
		}
	}

	s.qrMu.Lock()
	if s.qrChan == qrChan {
		s.qrChan = nil
		s.qrCancel = nil
	}
	s.qrMu.Unlock()
}

// handleMessage procesa un mensaje recibido
func (s *WhatsAppService) handleMessage(msg *events.Message) {
	chatJID := msg.Info.Chat.String()

	// ========== LOGGING DETALLADO PARA DEBUG ==========
	// Log simplificado del mensaje
	s.logger.Infof("📨 Nuevo mensaje: %s en %s", msg.Info.PushName, msg.Info.Chat.String())

	// Extraer contenido del mensaje
	content := extractTextContent(msg.Message)

	// Extraer número de teléfono Y nombre del remitente
	var senderPhone string
	var senderName string

	if msg.Info.IsFromMe {
		// Si es nuestro mensaje
		senderPhone = s.client.Store.ID.User
		senderName = "Yo"
	} else {
		// Para mensajes entrantes
		senderJID := msg.Info.Sender

		// SIEMPRE intentar obtener el nombre primero
		senderName = s.getSenderName(msg)

		// Determinar el "número de teléfono" o identificador según el tipo de servidor
		switch senderJID.Server {
		case "lid":
			// Usuario con LID - intentar obtener número real de asociaciones
			// s.logger.Infof("🔍 Detectado LID: %s", senderJID.User)
			realPhone := s.GetRealPhone(senderJID.User)
			if realPhone != "" {
				senderPhone = realPhone
				s.logger.Infof("✅ Número real encontrado: %s para %s", senderPhone, senderName)
			} else {
				s.requestPhoneAssociation(senderJID.User, senderName, msg.Info.Chat.String())
				senderPhone = senderJID.User // Usar LID temporalmente hasta que se asocie
				s.logger.Infof("🔗 Solicitando asociación para: %s (LID: %s)", senderName, senderJID.User)
			}

		case "s.whatsapp.net":
			// Usuario normal - tiene número real
			senderPhone = senderJID.User

		default:
			// Otro tipo de servidor
			senderPhone = senderJID.User
		}

		// Si no hay nombre, usar el phone como nombre
		if senderName == "" {
			senderName = senderPhone
		}
	}

	// Log final eliminado para simplificar

	// Obtener nombre del chat
	name := s.getChatName(msg.Info.Chat, chatJID, senderPhone)

	// Actualizar chat en la base de datos
	err := s.messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
	if err != nil {
		s.logger.Warnf("Failed to store chat: %v", err)
	}

	// Extraer info de media
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	if content == "" && mediaType == "" {
		return
	}

	// Guardar mensaje con teléfono Y nombre separados
	err = s.messageStore.StoreMessage(
		msg.Info.ID,
		chatJID,
		senderPhone,
		senderName,
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
			ID:          msg.Info.ID,
			ChatJID:     chatJID,
			ChatName:    name,
			SenderPhone: senderPhone,
			SenderName:  senderName,
			Content:     content,
			Timestamp:   msg.Info.Timestamp,
			IsFromMe:    msg.Info.IsFromMe,
			MediaType:   mediaType,
			Filename:    filename,
		})
	}
}

// ParticipantInfo contiene toda la información disponible de un participante
type ParticipantInfo struct {
	Index                int
	DisplayName          string
	JID                  string
	LID                  string
	PhoneNumber          string
	PhoneSource          string
	PhoneFromLID         string
	ResolvedPhone        string
	ResolvedPhoneFromLID string
	ContactName          string
	PushName             string
}

// PhoneAssociation representa una asociación entre sender_phone y número real
type PhoneAssociation struct {
	ID          int       `json:"id"`
	SenderPhone string    `json:"sender_phone"`
	RealPhone   string    `json:"real_phone"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// SenderInfo representa información de un remitente para la pestaña de asociaciones
type SenderInfo struct {
	SenderPhone   string    `json:"sender_phone"`
	SenderName    string    `json:"sender_name"`
	RealPhone     string    `json:"real_phone"`
	MessageCount  int       `json:"message_count"`
	LastMessage   time.Time `json:"last_message"`
	LastGroupName string    `json:"last_group_name"`
}

// PhoneAssociationRequest representa una solicitud de asociación
type PhoneAssociationRequest struct {
	LID         string    `json:"lid"`
	DisplayName string    `json:"display_name"`
	GroupJID    string    `json:"group_jid"`
	Timestamp   time.Time `json:"timestamp"`
}

// ListAllParticipantNumbers obtiene TODOS los números/IDs disponibles de todos los participantes
func (s *WhatsAppService) ListAllParticipantNumbers(groupJID types.JID) []ParticipantInfo {
	var participants []ParticipantInfo

	groupInfo, err := s.client.GetGroupInfo(context.Background(), groupJID)
	if err != nil {
		s.logger.Errorf("Error obteniendo info del grupo: %v", err)
		return participants
	}

	s.logger.Infof("📋 Listando números de %d participantes del grupo '%s'", len(groupInfo.Participants), groupInfo.Name)

	for i, participant := range groupInfo.Participants {
		info := ParticipantInfo{
			Index:       i + 1,
			DisplayName: participant.DisplayName,
			JID:         participant.JID.String(),
			LID:         participant.LID.String(),
		}

		// Pequeño delay para evitar rate limits
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		// Intentar obtener número real del JID
		if participant.JID.Server == "s.whatsapp.net" {
			info.PhoneNumber = participant.JID.User
			info.PhoneSource = "JID"
		}

		// Intentar obtener número real del LID
		if participant.LID.Server == "s.whatsapp.net" {
			info.PhoneFromLID = participant.LID.User
		}

		// Intentar resolver con GetUserInfo solo si es necesario
		if participant.JID.Server == "lid" {
			realPhone := s.resolvePhoneFromUserInfo(participant.JID)
			if realPhone != "" {
				info.ResolvedPhone = realPhone
				info.PhoneSource = "GetUserInfo"
			}
		}

		// Solo intentar resolver LID si es diferente al JID
		if participant.LID.Server == "lid" && participant.LID.User != participant.JID.User {
			realPhoneFromLID := s.resolvePhoneFromUserInfo(participant.LID)
			if realPhoneFromLID != "" {
				info.ResolvedPhoneFromLID = realPhoneFromLID
			}
		}

		// Obtener información adicional
		contact, err := s.client.Store.Contacts.GetContact(context.Background(), participant.JID)
		if err == nil {
			info.ContactName = contact.FullName
			info.PushName = contact.PushName
		}

		participants = append(participants, info)

		// Log detallado
		s.logger.Infof("--- Participante #%d: %s ---", info.Index, info.DisplayName)
		s.logger.Infof("  📱 Número principal: %s (fuente: %s)", info.PhoneNumber, info.PhoneSource)
		s.logger.Infof("  🔒 LID: %s", info.LID)
		s.logger.Infof("  📞 Número desde LID: %s", info.PhoneFromLID)
		s.logger.Infof("  ✅ Número resuelto: %s", info.ResolvedPhone)
		s.logger.Infof("  🔍 Resuelto desde LID: %s", info.ResolvedPhoneFromLID)
		s.logger.Infof("  👤 Nombre contacto: %s", info.ContactName)
		s.logger.Infof("  🏷️ PushName: %s", info.PushName)
		s.logger.Infof("")
	}

	return participants
}

// resolvePhoneFromUserInfo intenta obtener el número real usando GetUserInfo
func (s *WhatsAppService) resolvePhoneFromUserInfo(lidJID types.JID) string {
	s.logger.Infof("📞 Llamando GetUserInfo para LID: %s", lidJID.String())

	// Intentar obtener información del usuario
	users, err := s.client.GetUserInfo(context.Background(), []types.JID{lidJID})
	if err != nil {
		s.logger.Warnf("Error en GetUserInfo: %v", err)
		return ""
	}

	// Verificar si obtuvimos información
	if len(users) == 0 {
		s.logger.Warnf("GetUserInfo no devolvió información")
		return ""
	}

	// Examinar la información del usuario
	for jid, info := range users {
		s.logger.Infof("UserInfo recibido:")
		s.logger.Infof("  - JID key: %s", jid.String())
		s.logger.Infof("  - VerifiedName: %s", info.VerifiedName)
		s.logger.Infof("  - Status: %s", info.Status)
		s.logger.Infof("  - PictureID: %s", info.PictureID)
		s.logger.Infof("  - Devices: %v", info.Devices)

		// Si el JID de respuesta es diferente al LID, es el número real
		if jid.Server == "s.whatsapp.net" && jid.User != lidJID.User {
			s.logger.Infof("✅✅✅ Número real encontrado en GetUserInfo: %s", jid.User)
			return jid.User
		}
	}

	s.logger.Warnf("GetUserInfo no reveló el número real")
	return ""
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
		groupInfo, err := s.client.GetGroupInfo(context.Background(), jid)
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
		if err := s.initQRChannel(); err != nil {
			return fmt.Errorf("failed to prepare QR channel: %w", err)
		}
	}

	if err := s.client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	return nil
}

// Logout cierra la sesión actual y limpia la información del dispositivo
func (s *WhatsAppService) Logout() error {
	s.logger.Infof("🔓 Logout solicitado manualmente")
	s.resetQRChannel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.Logout(ctx); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	s.client.Disconnect()

	if s.onLoggedOut != nil {
		go s.onLoggedOut("Logout manual solicitado")
	}

	return nil
}

// StartAutoProcessor inicia el procesamiento automático en background
func (s *WhatsAppService) StartAutoProcessor() {
	if s.messageProcessor == nil {
		s.logger.Warnf("Message processor not initialized, cannot start auto processing")
		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		s.logger.Infof("🤖 Iniciando procesamiento automático cada 5 minutos")

		for range ticker.C {
			// Procesar hasta 10 mensajes cada 5 minutos
			results, err := s.messageProcessor.ProcessPendingMessages(10)
			if err != nil {
				s.logger.Errorf("Error en procesamiento automático: %v", err)
				continue
			}

			if len(results) > 0 {
				successCount := 0
				errorCount := 0

				for _, result := range results {
					if result.Status == "success" {
						successCount++
					} else {
						errorCount++
					}
				}

				s.logger.Infof("🤖 Procesamiento automático completado: %d exitosos, %d errores",
					successCount, errorCount)
			}
		}
	}()
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

// ===== FUNCIONES PARA ASOCIACIONES DE TELÉFONOS =====

// GetSendersForAssociation obtiene todos los remitentes únicos con sus asociaciones
// Incluye tanto remitentes con mensajes como aquellos solo en phone_associations
func (s *WhatsAppService) GetSendersForAssociation() ([]SenderInfo, error) {
	query := `
		WITH all_senders AS (
			-- Remitentes con mensajes (incluyendo grupo más reciente)
			SELECT 
				m.sender_phone,
				m.sender_name,
				COUNT(*) as message_count,
				MAX(m.timestamp) as last_message,
				-- Obtener el nombre del grupo del mensaje más reciente
				(SELECT c.name 
				 FROM messages m2 
				 JOIN chats c ON m2.chat_jid = c.jid 
				 WHERE m2.sender_phone = m.sender_phone 
				 ORDER BY m2.timestamp DESC 
				 LIMIT 1) as last_group_name
			FROM messages m
			WHERE m.sender_phone != '' AND m.sender_phone IS NOT NULL
			GROUP BY m.sender_phone, m.sender_name
			
			UNION
			
			-- Remitentes solo en phone_associations (sin mensajes recientes)
			SELECT 
				pa.sender_phone,
				pa.display_name as sender_name,
				0 as message_count,
				NULL as last_message,
				'' as last_group_name
			FROM phone_associations pa
			WHERE pa.sender_phone NOT IN (
				SELECT DISTINCT sender_phone FROM messages WHERE sender_phone IS NOT NULL
			)
		)
		SELECT 
			s.sender_phone,
			COALESCE(s.sender_name, '') as sender_name,
			COALESCE(pa.real_phone, '') as real_phone,
			s.message_count,
			COALESCE(s.last_message, '') as last_message,
			COALESCE(s.last_group_name, '') as last_group_name
		FROM all_senders s
		LEFT JOIN phone_associations pa ON s.sender_phone = pa.sender_phone
		ORDER BY s.message_count DESC, s.last_message DESC
	`

	rows, err := s.messageStore.db.Query(query)
	if err != nil {
		s.logger.Errorf("Error en consulta SQL: %v", err)
		// Retornar array vacío en lugar de error para mejor experiencia
		return []SenderInfo{}, nil
	}
	defer rows.Close()

	var senders []SenderInfo
	for rows.Next() {
		var sender SenderInfo
		var lastMessageStr string

		err := rows.Scan(
			&sender.SenderPhone,
			&sender.SenderName,
			&sender.RealPhone,
			&sender.MessageCount,
			&lastMessageStr,
			&sender.LastGroupName,
		)
		if err != nil {
			s.logger.Errorf("Error al escanear sender: %v", err)
			continue // Saltar este registro pero continuar con los demás
		}

		// Convertir string a time.Time
		if lastMessageStr != "" {
			lastMessage, err := time.Parse("2006-01-02 15:04:05", lastMessageStr)
			if err != nil {
				// Intentar otro formato común
				lastMessage, err = time.Parse(time.RFC3339, lastMessageStr)
				if err != nil {
					s.logger.Warnf("No se pudo parsear fecha para %s: %v", sender.SenderPhone, err)
					lastMessage = time.Time{} // Usar fecha vacía
				}
			}
			sender.LastMessage = lastMessage
		} else {
			// Sin mensajes, usar fecha vacía
			sender.LastMessage = time.Time{}
		}

		senders = append(senders, sender)
	}

	s.logger.Infof("📋 Obtenidos %d remitentes para asociaciones", len(senders))
	return senders, nil
}

// SavePhoneAssociation guarda o actualiza una asociación de teléfono
func (s *WhatsAppService) SavePhoneAssociation(senderPhone, realPhone, displayName string) error {
	_, err := s.messageStore.db.Exec(`
		INSERT INTO phone_associations 
		(sender_phone, real_phone, display_name, updated_at) 
		VALUES (?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE 
		real_phone = VALUES(real_phone), 
		display_name = VALUES(display_name), 
		updated_at = NOW()
	`, senderPhone, realPhone, displayName)

	if err != nil {
		return fmt.Errorf("failed to save phone association: %v", err)
	}

	s.logger.Infof("✅ Asociación guardada: %s -> %s (%s)", senderPhone, realPhone, displayName)
	return nil
}

// DeletePhoneAssociation elimina una asociación
func (s *WhatsAppService) DeletePhoneAssociation(senderPhone string) error {
	result, err := s.messageStore.db.Exec("DELETE FROM phone_associations WHERE sender_phone = ?", senderPhone)
	if err != nil {
		return fmt.Errorf("failed to delete phone association: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("phone association not found")
	}

	s.logger.Infof("🗑️ Asociación eliminada: %s", senderPhone)
	return nil
}

// GetMessagesBySenderPhone obtiene mensajes de un remitente específico
func (s *WhatsAppService) GetMessagesBySenderPhone(senderPhone string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.messageStore.GetMessagesBySenderPhone(senderPhone, limit)
}

// DeleteMessage elimina un mensaje de la base de datos
func (s *WhatsAppService) DeleteMessage(messageID, chatJID string) error {
	return s.messageStore.DeleteMessage(messageID, chatJID)
}

// DeleteMessagesBySenderPhone elimina todos los mensajes de un remitente específico
func (s *WhatsAppService) DeleteMessagesBySenderPhone(senderPhone string) error {
	return s.messageStore.DeleteMessagesBySenderPhone(senderPhone)
}

// GetRealPhone obtiene el número real asociado a un sender_phone
func (s *WhatsAppService) GetRealPhone(senderPhone string) string {
	var realPhone string
	err := s.messageStore.db.QueryRow(
		"SELECT real_phone FROM phone_associations WHERE sender_phone = ?",
		senderPhone,
	).Scan(&realPhone)

	if err != nil {
		return "" // No encontrado
	}

	return realPhone
}

// UpdatePhoneProfiling actualiza el perfil y confianza de un número de teléfono
func (s *WhatsAppService) UpdatePhoneProfiling(realPhone string, isValidLoad bool) error {
	// Determinar el cambio en confianza
	// +1 si envió una carga válida (es loader)
	// -1 si envió mensaje de camionero buscando carga
	confianzaDelta := 1
	if !isValidLoad {
		confianzaDelta = -1
	}

	// Actualizar confianza
	_, err := s.messageStore.db.Exec(`
		UPDATE phone_associations 
		SET confianza = confianza + ?,
		    updated_at = NOW()
		WHERE real_phone = ?
	`, confianzaDelta, realPhone)

	if err != nil {
		return fmt.Errorf("failed to update trust score: %v", err)
	}

	// Actualizar perfil basado en el score de confianza
	_, err = s.messageStore.db.Exec(`
		UPDATE phone_associations 
		SET perfil = CASE 
			WHEN confianza > 0 THEN 'loader'
			WHEN confianza < 0 THEN 'camionero'
			ELSE 'desconocido'
		END,
		updated_at = NOW()
		WHERE real_phone = ?
	`, realPhone)

	if err != nil {
		return fmt.Errorf("failed to update profile: %v", err)
	}

	s.logger.Infof("📊 Perfil actualizado para %s: confianza %+d", realPhone, confianzaDelta)
	return nil
}

// UpdatePhoneName actualiza el nombre de un contacto
func (s *WhatsAppService) UpdatePhoneName(realPhone, nombre string) error {
	_, err := s.messageStore.db.Exec(`
		UPDATE phone_associations 
		SET nombre = ?,
		    updated_at = NOW()
		WHERE real_phone = ?
	`, nombre, realPhone)

	if err != nil {
		return fmt.Errorf("failed to update phone name: %v", err)
	}

	s.logger.Infof("✏️ Nombre actualizado para %s: %s", realPhone, nombre)
	return nil
}

// requestPhoneAssociation solicita una asociación de teléfono
func (s *WhatsAppService) requestPhoneAssociation(lid, displayName, groupJID string) {
	if s.onPhoneAssociationNeeded != nil {
		s.onPhoneAssociationNeeded(PhoneAssociationRequest{
			LID:         lid,
			DisplayName: displayName,
			GroupJID:    groupJID,
			Timestamp:   time.Now(),
		})
		s.logger.Infof("🔗 Solicitando asociación: LID %s (%s) en grupo %s", lid, displayName, groupJID)
	}
}

// ===== FUNCIONES SIMPLIFICADAS =====
