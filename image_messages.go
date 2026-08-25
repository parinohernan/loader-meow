package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImageMessageInfo representa un mensaje con imagen para la galería
type ImageMessageInfo struct {
	ChatMessage
	RealPhone       string `json:"real_phone"`
	ExtractedText   string `json:"extracted_text"`
	MediaLocalPath  string `json:"media_local_path"`
	FileSHA256Hex   string `json:"file_sha256_hex"`
	DuplicateCount  int    `json:"duplicate_count"`
}

// ImageMessageRef referencia mínima para eliminar mensajes con imagen
type ImageMessageRef struct {
	ID      string `json:"id"`
	ChatJID string `json:"chat_jid"`
}

// ImagePreviewResponse respuesta con imagen en base64 para el frontend
type ImagePreviewResponse struct {
	Success bool   `json:"success"`
	DataURL string `json:"data_url"`
	Path    string `json:"path"`
	Error   string `json:"error"`
}

// GetImageMessages obtiene mensajes con imágenes. filter: "all", "pending", "processed"
func (store *MessageStore) GetImageMessages(filter string, limit int) ([]ImageMessageInfo, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT m.id, m.chat_jid, COALESCE(c.name, m.chat_jid) as chat_name,
		       m.sender_phone, m.sender_name, m.content, m.timestamp, m.is_from_me,
		       m.media_type, m.filename, m.processed,
		       COALESCE(pa.real_phone, ''), COALESCE(m.extracted_text, ''), COALESCE(m.media_local_path, ''),
		       COALESCE(HEX(m.file_sha256), ''),
		       CASE
		         WHEN m.file_sha256 IS NULL THEN 1
		         ELSE (
		           SELECT COUNT(*) FROM messages m2
		           WHERE m2.media_type = 'image' AND m2.file_sha256 = m.file_sha256
		         )
		       END
		FROM messages m
		LEFT JOIN chats c ON m.chat_jid = c.jid
		LEFT JOIN phone_associations pa ON m.sender_phone = pa.sender_phone
		WHERE m.media_type = 'image'
	`

	switch filter {
	case "pending":
		query += ` AND m.processed = 0`
	case "processed":
		query += ` AND m.processed = 1`
	}

	query += ` ORDER BY m.timestamp DESC LIMIT ?`

	rows, err := store.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ImageMessageInfo
	for rows.Next() {
		var msg ImageMessageInfo
		err := rows.Scan(
			&msg.ID, &msg.ChatJID, &msg.ChatName,
			&msg.SenderPhone, &msg.SenderName, &msg.Content, &msg.Timestamp, &msg.IsFromMe,
			&msg.MediaType, &msg.Filename, &msg.Processed,
			&msg.RealPhone, &msg.ExtractedText, &msg.MediaLocalPath,
			&msg.FileSHA256Hex, &msg.DuplicateCount,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// UpdateMessageMediaLocalPath guarda la ruta local del archivo de media
func (store *MessageStore) UpdateMessageMediaLocalPath(messageID, chatJID, localPath string) error {
	_, err := store.db.Exec(
		`UPDATE messages SET media_local_path = ? WHERE id = ? AND chat_jid = ?`,
		localPath, messageID, chatJID,
	)
	return err
}

// GetMessageMediaLocalPath obtiene la ruta local guardada
func (store *MessageStore) GetMessageMediaLocalPath(messageID, chatJID string) (string, error) {
	var path string
	err := store.db.QueryRow(
		`SELECT COALESCE(media_local_path, '') FROM messages WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	).Scan(&path)
	return path, err
}

// DiscardImageMessage marca una imagen como descartada (procesada sin IA)
func (store *MessageStore) DiscardImageMessage(messageID, chatJID string) error {
	_, err := store.db.Exec(
		`UPDATE messages SET processed = 1, last_processing_error = 'Descartado manualmente' WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	)
	return err
}

// DeleteImageMessages elimina una lista concreta de mensajes con imagen
func (store *MessageStore) DeleteImageMessages(refs []ImageMessageRef) (int64, error) {
	if len(refs) == 0 {
		return 0, nil
	}

	var deleted int64
	for _, ref := range refs {
		if ref.ID == "" || ref.ChatJID == "" {
			continue
		}
		if err := store.DeleteMessage(ref.ID, ref.ChatJID); err != nil {
			return deleted, fmt.Errorf("failed to delete message %s: %v", ref.ID, err)
		}
		deleted++
	}
	return deleted, nil
}

// DeletePendingImageMessages elimina todos los mensajes con imagen pendientes
func (store *MessageStore) DeletePendingImageMessages() (int64, error) {
	rows, err := store.db.Query(
		`SELECT id, chat_jid FROM messages WHERE media_type = 'image' AND processed = 0`,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var messageID, chatJID string
		if err := rows.Scan(&messageID, &chatJID); err != nil {
			return 0, err
		}
		store.deleteMessageMediaFile(messageID, chatJID)
	}

	result, err := store.db.Exec(`DELETE FROM messages WHERE media_type = 'image' AND processed = 0`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteMessageMediaFile elimina el archivo local asociado a un mensaje
func (store *MessageStore) deleteMessageMediaFile(messageID, chatJID string) {
	localPath, err := store.GetMessageMediaLocalPath(messageID, chatJID)
	if err == nil && localPath != "" {
		_ = os.Remove(localPath)
		return
	}

	_, filename, _, _, _, _, _, err := store.GetMediaInfo(messageID, chatJID)
	if err != nil || filename == "" {
		return
	}
	localPath = filepath.Join("store", strings.ReplaceAll(chatJID, ":", "_"), filename)
	_ = os.Remove(localPath)
}

func imagePathToDataURL(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := "image/jpeg"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		mime = "image/png"
	case ".webp":
		mime = "image/webp"
	case ".gif":
		mime = "image/gif"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}
