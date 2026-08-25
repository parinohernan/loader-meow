package main

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
)

// CanonicalProcessedImage representa la primera imagen procesada con un file_sha256 dado
type CanonicalProcessedImage struct {
	MessageID       string
	ChatJID         string
	Content         string
	ExtractedText   string
	MediaLocalPath  string
	WasDiscarded    bool
	WasDuplicate    bool
}

func normalizeCaption(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func captionsEqual(a, b string) bool {
	return normalizeCaption(a) == normalizeCaption(b)
}

func sha256Equal(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return bytes.Equal(a, b)
}

func sha256Hex(h []byte) string {
	if len(h) == 0 {
		return ""
	}
	return hex.EncodeToString(h)
}

// GetMessageFileSHA256 obtiene el hash de archivo de un mensaje
func (store *MessageStore) GetMessageFileSHA256(messageID, chatJID string) ([]byte, error) {
	var hash []byte
	err := store.db.QueryRow(
		`SELECT file_sha256 FROM messages WHERE id = ? AND chat_jid = ?`,
		messageID, chatJID,
	).Scan(&hash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found")
	}
	return hash, err
}

// FindCanonicalProcessedImage busca una imagen ya procesada con el mismo file_sha256
func (store *MessageStore) FindCanonicalProcessedImage(fileSHA256 []byte, excludeMessageID, excludeChatJID string) (*CanonicalProcessedImage, error) {
	if len(fileSHA256) == 0 {
		return nil, nil
	}

	var canonical CanonicalProcessedImage
	var extractedText, content, mediaLocalPath, lastError sql.NullString

	err := store.db.QueryRow(`
		SELECT id, chat_jid, COALESCE(content, ''), COALESCE(extracted_text, ''),
		       COALESCE(media_local_path, ''), COALESCE(last_processing_error, '')
		FROM messages
		WHERE media_type = 'image'
		  AND file_sha256 = ?
		  AND processed = 1
		  AND NOT (id = ? AND chat_jid = ?)
		  AND (
		    (extracted_text IS NOT NULL AND extracted_text != '')
		    OR last_processing_error = 'Descartado manualmente'
		    OR last_processing_error LIKE 'Duplicado de imagen%'
		  )
		ORDER BY
		  CASE
		    WHEN extracted_text IS NOT NULL AND extracted_text != '' THEN 0
		    WHEN last_processing_error = 'Descartado manualmente' THEN 1
		    ELSE 2
		  END,
		  timestamp ASC
		LIMIT 1
	`, fileSHA256, excludeMessageID, excludeChatJID).Scan(
		&canonical.MessageID, &canonical.ChatJID, &content, &extractedText,
		&mediaLocalPath, &lastError,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if content.Valid {
		canonical.Content = content.String
	}
	if extractedText.Valid {
		canonical.ExtractedText = extractedText.String
	}
	if mediaLocalPath.Valid {
		canonical.MediaLocalPath = mediaLocalPath.String
	}
	if lastError.Valid {
		errMsg := lastError.String
		canonical.WasDiscarded = errMsg == "Descartado manualmente"
		canonical.WasDuplicate = strings.HasPrefix(errMsg, "Duplicado de imagen")
	}

	return &canonical, nil
}

// CountImagesWithSHA256 cuenta cuántos mensajes comparten el mismo hash de imagen
func (store *MessageStore) CountImagesWithSHA256(fileSHA256 []byte) (int, error) {
	if len(fileSHA256) == 0 {
		return 1, nil
	}
	var count int
	err := store.db.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE media_type = 'image' AND file_sha256 = ?`,
		fileSHA256,
	).Scan(&count)
	return count, err
}

// MarkImageDuplicateNote registra que un mensaje fue resuelto como duplicado
func (store *MessageStore) MarkImageDuplicateNote(messageID, chatJID, canonicalMessageID, canonicalChatJID string, skipAI bool) error {
	note := fmt.Sprintf("Duplicado de imagen %s/%s", canonicalMessageID, canonicalChatJID)
	if skipAI {
		note += " (OCR reutilizado, IA omitida)"
	} else {
		note += " (OCR reutilizado)"
	}
	_, err := store.db.Exec(
		`UPDATE messages SET last_processing_error = ? WHERE id = ? AND chat_jid = ?`,
		note, messageID, chatJID,
	)
	return err
}

// AutoMarkSameCaptionDuplicates marca como procesados los duplicados pendientes con mismo caption
func (store *MessageStore) AutoMarkSameCaptionDuplicates(fileSHA256 []byte, canonicalID, canonicalChatJID, caption, extractedText string) (int64, error) {
	if len(fileSHA256) == 0 {
		return 0, nil
	}

	note := fmt.Sprintf("Duplicado de imagen %s/%s (OCR reutilizado, IA omitida)", canonicalID, canonicalChatJID)
	result, err := store.db.Exec(`
		UPDATE messages
		SET processed = 1,
		    extracted_text = ?,
		    last_processing_error = ?
		WHERE media_type = 'image'
		  AND processed = 0
		  AND file_sha256 = ?
		  AND NOT (id = ? AND chat_jid = ?)
		  AND LOWER(TRIM(COALESCE(content, ''))) = LOWER(TRIM(?))
	`, extractedText, note, fileSHA256, canonicalID, canonicalChatJID, caption)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
