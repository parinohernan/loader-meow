package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// MessageProcessor orquesta el procesamiento de mensajes con IA
type MessageProcessor struct {
	messageStore    *MessageStore
	aiService       *AIService
	supabaseService *SupabaseService
	logger          waLog.Logger
}

// ProcessingResult representa el resultado del procesamiento de un mensaje
type ProcessingResult struct {
	ID                 int       `json:"id"`
	MessageID          string    `json:"message_id"`
	ChatJID            string    `json:"chat_jid"`
	Content            string    `json:"content"`
	SenderPhone        string    `json:"sender_phone"`
	RealPhone          string    `json:"real_phone"`
	AIResponse         string    `json:"ai_response"`
	Status             string    `json:"status"`
	ErrorMessage       string    `json:"error_message"`
	SupabaseIDs        []string  `json:"supabase_ids"`
	ProcessedAt        time.Time `json:"processed_at"`
	ProcessingAttempts int       `json:"processing_attempts"`
}

// NewMessageProcessor crea una nueva instancia del procesador de mensajes
func NewMessageProcessor(messageStore *MessageStore, logger waLog.Logger, keysManager *APIKeysManager) (*MessageProcessor, error) {
	// Inicializar servicio de IA
	aiService, err := NewAIService(keysManager)
	if err != nil {
		// Si falla por falta de configuración, crear el processor sin IA
		// Se podrá configurar luego desde la UI
		logger.Warnf("AI service not available: %v", err)
		return &MessageProcessor{
			messageStore:    messageStore,
			aiService:       nil,
			supabaseService: NewSupabaseService(),
			logger:          logger,
		}, nil
	}
	
	// Inicializar servicio de Supabase
	supabaseService := NewSupabaseService()
	
	return &MessageProcessor{
		messageStore:    messageStore,
		aiService:       aiService,
		supabaseService: supabaseService,
		logger:          logger,
	}, nil
}

// ProcessPendingMessages procesa mensajes pendientes
func (p *MessageProcessor) ProcessPendingMessages(limit int) ([]ProcessingResult, error) {
	// Obtener mensajes procesables
	messages, err := p.messageStore.GetProcessableMessages(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get processable messages: %v", err)
	}
	
	if len(messages) == 0 {
		p.logger.Infof("No hay mensajes procesables")
		return []ProcessingResult{}, nil
	}
	
	p.logger.Infof("Procesando %d mensajes", len(messages))
	
	var results []ProcessingResult
	
	for i, msg := range messages {
		p.logger.Infof("Procesando mensaje %d/%d: %s", i+1, len(messages), msg.ID)
		
		// Procesar mensaje individual
		result := p.processMessage(msg)
		results = append(results, result)
		
		// Guardar resultado en la base de datos
		if err := p.saveProcessingResult(result); err != nil {
			p.logger.Errorf("Error guardando resultado: %v", err)
		}
		
		// Manejar el resultado según el estado
		switch result.Status {
		case "success":
			// Marcar como procesado exitosamente
			if err := p.messageStore.MarkMessageAsProcessed(msg.ID, msg.ChatJID); err != nil {
				p.logger.Errorf("Error marcando mensaje como procesado: %v", err)
			}
		case "error":
			// Incrementar contador de intentos y registrar error
			if err := p.messageStore.IncrementProcessingAttempt(msg.ID, msg.ChatJID, result.ErrorMessage); err != nil {
				p.logger.Errorf("Error incrementando intentos: %v", err)
			} else {
				p.logger.Warnf("Intento fallido para mensaje %s. Error: %s", msg.ID, result.ErrorMessage)
			}
		}
		
		// Pequeña pausa entre mensajes
		time.Sleep(500 * time.Millisecond)
	}
	
	p.logger.Infof("Procesamiento completado: %d resultados", len(results))
	return results, nil
}

// processMessage procesa un mensaje individual
func (p *MessageProcessor) processMessage(msg ProcessableMessage) ProcessingResult {
	result := ProcessingResult{
		MessageID:   msg.ID,
		ChatJID:     msg.ChatJID,
		Content:     msg.Content,
		SenderPhone: msg.SenderPhone,
		RealPhone:   msg.RealPhone,
		Status:      "processing",
		ProcessedAt: time.Now(),
	}
	
	// Verificar si el servicio de IA está disponible
	if p.aiService == nil {
		result.Status = "error"
		result.ErrorMessage = "AI service not configured. Please configure API keys."
		p.logger.Errorf("AI service not available")
		return result
	}
	
	// 1. Procesar con IA
	p.logger.Infof("Llamando a IA para mensaje %s", msg.ID)
	aiResponse, err := p.aiService.ProcessMessage(msg.Content, msg.RealPhone)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("AI processing failed: %v", err)
		p.logger.Errorf("Error en procesamiento IA: %v", err)
		return result
	}
	
	result.AIResponse = string(aiResponse)
	p.logger.Infof("IA respondió para mensaje %s", msg.ID)
	
	// 2. Validar respuesta de IA
	if err := p.aiService.ValidateResponse(aiResponse); err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Invalid AI response: %v", err)
		p.logger.Errorf("Respuesta de IA inválida: %v", err)
		return result
	}
	
	// 3. Subir a Supabase
	p.logger.Infof("Subiendo a Supabase para mensaje %s", msg.ID)
	p.logger.Infof("JSON de IA para Supabase: %s", string(aiResponse))
	supabaseIDs, err := p.supabaseService.CrearCargasDesdeJSON(aiResponse)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Supabase upload failed: %v", err)
		result.AIResponse = string(aiResponse) // Guardar respuesta de IA aunque falle Supabase
		p.logger.Errorf("Error subiendo a Supabase: %v", err)
		p.logger.Errorf("JSON que causó el error: %s", string(aiResponse))
		return result
	}
	
	result.SupabaseIDs = supabaseIDs
	result.Status = "success"
	p.logger.Infof("Mensaje %s procesado exitosamente: %d cargas creadas", msg.ID, len(supabaseIDs))
	
	return result
}

// saveProcessingResult guarda el resultado del procesamiento en la base de datos
func (p *MessageProcessor) saveProcessingResult(result ProcessingResult) error {
	supabaseIDsJSON, _ := json.Marshal(result.SupabaseIDs)
	
	query := `
		INSERT INTO ai_processing_results 
		(message_id, chat_jid, content, sender_phone, real_phone, ai_response, status, error_message, supabase_ids, processed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := p.messageStore.db.Exec(query,
		result.MessageID,
		result.ChatJID,
		result.Content,
		result.SenderPhone,
		result.RealPhone,
		result.AIResponse,
		result.Status,
		result.ErrorMessage,
		string(supabaseIDsJSON),
		result.ProcessedAt,
	)
	
	return err
}

// GetProcessingResults obtiene resultados de procesamiento
func (p *MessageProcessor) GetProcessingResults(limit int) ([]ProcessingResult, error) {
	query := `
		SELECT apr.id, apr.message_id, apr.chat_jid, apr.content, apr.sender_phone, apr.real_phone, 
		       apr.ai_response, apr.status, apr.error_message, apr.supabase_ids, apr.processed_at,
		       COALESCE(m.processing_attempts, 0) as processing_attempts
		FROM ai_processing_results apr
		LEFT JOIN messages m ON apr.message_id = m.id AND apr.chat_jid = m.chat_jid
		ORDER BY apr.processed_at DESC
		LIMIT ?
	`
	
	rows, err := p.messageStore.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []ProcessingResult
	for rows.Next() {
		var result ProcessingResult
		var supabaseIDsJSON sql.NullString
		
		err := rows.Scan(
			&result.ID,
			&result.MessageID,
			&result.ChatJID,
			&result.Content,
			&result.SenderPhone,
			&result.RealPhone,
			&result.AIResponse,
			&result.Status,
			&result.ErrorMessage,
			&supabaseIDsJSON,
			&result.ProcessedAt,
			&result.ProcessingAttempts,
		)
		if err != nil {
			return nil, err
		}
		
		// Parsear JSON de Supabase IDs
		if supabaseIDsJSON.Valid {
			json.Unmarshal([]byte(supabaseIDsJSON.String), &result.SupabaseIDs)
		}
		
		results = append(results, result)
	}
	
	return results, nil
}

// GetProcessingStats obtiene estadísticas de procesamiento
func (p *MessageProcessor) GetProcessingStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Contar mensajes procesables
	processableCount, err := p.messageStore.GetProcessableMessagesCount()
	if err != nil {
		return nil, err
	}
	stats["processable_count"] = processableCount
	
	// Contar resultados por estado
	statusQuery := `
		SELECT status, COUNT(*) as count
		FROM ai_processing_results
		WHERE DATE(processed_at) = CURDATE()
		GROUP BY status
	`
	
	rows, err := p.messageStore.db.Query(statusQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	statusCounts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		statusCounts[status] = count
	}
	
	stats["status_counts"] = statusCounts
	stats["total_processed_today"] = statusCounts["success"] + statusCounts["error"]
	stats["success_count"] = statusCounts["success"]
	stats["error_count"] = statusCounts["error"]
	
	return stats, nil
}

// GetProcessableMessagesCount obtiene el conteo de mensajes procesables
func (p *MessageProcessor) GetProcessableMessagesCount() (int, error) {
	return p.messageStore.GetProcessableMessagesCount()
}

// ProcessSingleMessage procesa un solo mensaje por ID
func (p *MessageProcessor) ProcessSingleMessage(messageID, chatJID string) (ProcessingResult, error) {
	// Obtener el mensaje específico
	messages, err := p.messageStore.GetProcessableMessages(1000)
	if err != nil {
		return ProcessingResult{}, fmt.Errorf("failed to get messages: %v", err)
	}
	
	// Buscar el mensaje específico
	var targetMessage *ProcessableMessage
	for _, msg := range messages {
		if msg.ID == messageID && msg.ChatJID == chatJID {
			targetMessage = &msg
			break
		}
	}
	
	if targetMessage == nil {
		return ProcessingResult{}, fmt.Errorf("message not found or not processable")
	}
	
	p.logger.Infof("Procesando mensaje individual: %s", messageID)
	
	// Procesar el mensaje
	result := p.processMessage(*targetMessage)
	
	// Guardar resultado
	if err := p.saveProcessingResult(result); err != nil {
		p.logger.Errorf("Error guardando resultado: %v", err)
	}
	
	// Manejar el resultado según el estado
	switch result.Status {
	case "success":
		if err := p.messageStore.MarkMessageAsProcessed(targetMessage.ID, targetMessage.ChatJID); err != nil {
			p.logger.Errorf("Error marcando mensaje como procesado: %v", err)
		}
	case "error":
		if err := p.messageStore.IncrementProcessingAttempt(targetMessage.ID, targetMessage.ChatJID, result.ErrorMessage); err != nil {
			p.logger.Errorf("Error incrementando intentos: %v", err)
		}
	}
	
	return result, nil
}
