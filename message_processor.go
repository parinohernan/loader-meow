package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// MessageProcessor orquesta el procesamiento de mensajes con IA
type MessageProcessor struct {
	messageStore      *MessageStore
	aiService         *AIService // Mantener por compatibilidad
	aiProviderService *AIProviderService // Nuevo servicio multi-proveedor
	aiConfigManager   *AIConfigManager
	supabaseService   *SupabaseService
	systemPrompt      string
	logger            waLog.Logger
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
func NewMessageProcessor(messageStore *MessageStore, logger waLog.Logger, keysManager *APIKeysManager, aiConfigManager *AIConfigManager, systemConfigManager *SystemConfigManager) (*MessageProcessor, error) {
	// Cargar system prompt
	systemPrompt, err := loadSystemPrompt()
	if err != nil {
		logger.Warnf("Failed to load system prompt: %v", err)
		systemPrompt = "Eres un asistente de IA."
	}
	
	// Inicializar servicio de IA legacy (mantener por compatibilidad)
	aiService, err := NewAIService(keysManager)
	if err != nil {
		logger.Warnf("AI service (legacy) not available: %v", err)
	}
	
	// Inicializar nuevo servicio multi-proveedor
	aiProviderService := NewAIProviderService(aiConfigManager)
	
	// Inicializar servicio de Supabase con system config manager
	supabaseService := NewSupabaseService(systemConfigManager)
	
	return &MessageProcessor{
		messageStore:      messageStore,
		aiService:         aiService,
		aiProviderService: aiProviderService,
		aiConfigManager:   aiConfigManager,
		supabaseService:   supabaseService,
		systemPrompt:      systemPrompt,
		logger:            logger,
	}, nil
}

// cleanMessageContent limpia el contenido del mensaje eliminando caracteres especiales
// que pueden interferir con el procesamiento JSON de la IA
func cleanMessageContent(content string) string {
	// Reemplazar caracteres problemáticos que pueden romper el JSON
	cleaned := strings.ReplaceAll(content, "}", "")
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "]", "")
	cleaned = strings.ReplaceAll(cleaned, "[", "")
	
	// Limpiar espacios múltiples generados por las eliminaciones
	cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	cleaned = strings.TrimSpace(cleaned)
	
	return cleaned
}

// isPermanentError detecta si un error es permanente (no se soluciona reintentando)
// Estos errores deben marcar el mensaje como procesado inmediatamente para evitar
// consumir tokens innecesariamente en reintentos que no van a funcionar
func isPermanentError(errorMsg string) bool {
	errorLower := strings.ToLower(errorMsg)
	
	// Errores de JSON inválido - no se solucionan reintentando
	if strings.Contains(errorLower, "invalid json") || 
	   strings.Contains(errorLower, "invalid ai response") ||
	   strings.Contains(errorLower, "failed to normalize") ||
	   strings.Contains(errorLower, "json.unmarshal") {
		return true
	}
	
	// Errores de validación permanente
	if strings.Contains(errorLower, "invalid locations") {
		return true
	}
	
	return false
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
			// Todos los errores se marcan como procesados inmediatamente para evitar reintentos
			// que consumen tokens innecesariamente
			p.logger.Warnf("🚫 Error detectado para mensaje %s: %s. Marcando como procesado para evitar reintentos.", msg.ID, result.ErrorMessage)
			if err := p.messageStore.MarkMessageAsFailedAfterRetries(msg.ID, msg.ChatJID, result.ErrorMessage); err != nil {
				p.logger.Errorf("Error marcando mensaje como fallido: %v", err)
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
	
	// Verificar si hay configuración activa de IA
	activeConfig, err := p.aiConfigManager.GetActiveConfig()
	if err != nil {
		// Intentar usar el servicio legacy si está disponible
		if p.aiService == nil {
			result.Status = "error"
			result.ErrorMessage = "No active AI configuration found. Please configure AI settings."
			p.logger.Errorf("No AI service available")
			return result
		}
		p.logger.Warnf("Using legacy AI service")
	}
	
	var aiResponse []byte
	
	// Limpiar contenido del mensaje eliminando caracteres especiales problemáticos
	cleanedContent := cleanMessageContent(msg.Content)
	if cleanedContent != msg.Content {
		p.logger.Infof("🧹 Contenido limpiado: se eliminaron caracteres especiales del mensaje %s", msg.ID)
	}
	
	// 1. Procesar con IA principal
	p.logger.Infof("Llamando a IA principal para mensaje %s", msg.ID)
	
	if activeConfig != nil {
		// Usar nuevo sistema multi-proveedor con contenido limpiado
		aiResponse, err = p.aiProviderService.ProcessMessageWithConfig(activeConfig, p.systemPrompt, cleanedContent, msg.RealPhone)
	} else {
		// Usar sistema legacy con contenido limpiado
		aiResponse, err = p.aiService.ProcessMessage(cleanedContent, msg.RealPhone)
	}
	
	// Si falla con la principal y hay configuración secundaria, intentar con secundaria
	if err != nil && activeConfig != nil && activeConfig.SecondaryConfigID != nil {
		p.logger.Warnf("⚠️ Error con IA principal para mensaje %s: %v. Intentando con IA secundaria...", msg.ID, err)
		
		// Obtener configuración secundaria
		secondaryConfig, secondaryErr := p.aiConfigManager.GetSecondaryConfig(*activeConfig.SecondaryConfigID)
		if secondaryErr == nil && secondaryConfig != nil {
			p.logger.Infof("🔄 Intentando con IA secundaria: %s - %s (%s)", secondaryConfig.ProviderDisplay, secondaryConfig.ModelDisplay, secondaryConfig.Name)
			aiResponse, err = p.aiProviderService.ProcessMessageWithConfig(secondaryConfig, p.systemPrompt, cleanedContent, msg.RealPhone)
			
			if err == nil {
				p.logger.Infof("✅ IA secundaria procesó exitosamente el mensaje %s", msg.ID)
			} else {
				p.logger.Errorf("❌ IA secundaria también falló para mensaje %s: %v", msg.ID, err)
			}
		} else {
			p.logger.Errorf("❌ No se pudo obtener configuración secundaria: %v", secondaryErr)
		}
	}
	
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("AI processing failed: %v", err)
		p.logger.Errorf("Error en procesamiento IA: %v", err)
		p.logger.Errorf("🔴 PROCESAMIENTO FINALIZADO CON ERROR para mensaje %s", msg.ID)
		return result
	}
	
	result.AIResponse = string(aiResponse)
	p.logger.Infof("IA respondió para mensaje %s", msg.ID)
	
	// 2. Validar respuesta de IA
	if err := p.aiProviderService.ValidateResponse(aiResponse); err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Invalid AI response: %v", err)
		p.logger.Errorf("Respuesta de IA inválida: %v", err)
		return result
	}
	
	// 2.5. Normalizar respuesta (convertir objeto único a array si es necesario)
	normalizedResponse, err := p.aiProviderService.NormalizeResponse(aiResponse)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Failed to normalize AI response: %v", err)
		p.logger.Errorf("Error normalizando respuesta: %v", err)
		return result
	}
	
	// Actualizar AIResponse con la versión normalizada
	result.AIResponse = string(normalizedResponse)
	
	// 2.6. Verificar si el array está vacío (mensaje sin información suficiente)
	var cargasTemp []map[string]interface{}
	json.Unmarshal(normalizedResponse, &cargasTemp)
	
	if len(cargasTemp) == 0 {
		result.Status = "success"
		result.ErrorMessage = "No hay información de carga válida en el mensaje (array vacío)"
		p.logger.Infof("Mensaje %s: No contiene información de carga válida (array vacío)", msg.ID)
		
		// Actualizar perfil: probablemente es un camionero buscando carga (-1 confianza)
		if err := p.messageStore.UpdatePhoneProfiling(msg.RealPhone, false); err != nil {
			p.logger.Warnf("Error actualizando perfil para %s: %v", msg.RealPhone, err)
		} else {
			p.logger.Infof("📉 Perfil actualizado: %s (-1 confianza, posible camionero)", msg.RealPhone)
		}
		
		return result
	}
	
	// 2.7. Validar que las ubicaciones sean reales
	if err := p.validateLocations(normalizedResponse); err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Invalid locations: %v", err)
		result.AIResponse = string(normalizedResponse)
		p.logger.Warnf("Mensaje rechazado por ubicaciones inválidas: %v", err)
		return result
	}
	
	// 3. Subir a Supabase
	p.logger.Infof("Subiendo a Supabase para mensaje %s", msg.ID)
	p.logger.Infof("JSON de IA para Supabase: %s", string(normalizedResponse))
	supabaseIDs, err := p.supabaseService.CrearCargasDesdeJSON(normalizedResponse)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Supabase upload failed: %v", err)
		result.AIResponse = string(normalizedResponse) // Guardar respuesta normalizada
		p.logger.Errorf("Error subiendo a Supabase: %v", err)
		p.logger.Errorf("JSON que causó el error: %s", string(normalizedResponse))
		return result
	}
	
	result.SupabaseIDs = supabaseIDs
	result.Status = "success"
	p.logger.Infof("Mensaje %s procesado exitosamente: %d cargas creadas", msg.ID, len(supabaseIDs))
	p.logger.Infof("🟢 PROCESAMIENTO FINALIZADO EXITOSAMENTE para mensaje %s", msg.ID)
	
	// Actualizar perfil: carga válida procesada exitosamente (+1 confianza)
	if err := p.messageStore.UpdatePhoneProfiling(msg.RealPhone, true); err != nil {
		p.logger.Warnf("Error actualizando perfil para %s: %v", msg.RealPhone, err)
	} else {
		p.logger.Infof("📈 Perfil actualizado: %s (+1 confianza, loader confirmado)", msg.RealPhone)
	}
	
	return result
}

// SimulateMessage procesa un mensaje sin guardarlo en la base de datos (para simulador)
func (p *MessageProcessor) SimulateMessage(messageContent, realPhone string) ProcessingResult {
	// Crear un mensaje simulado
	simulatedMsg := ProcessableMessage{
		ChatMessage: ChatMessage{
			ID:          "simulation-" + fmt.Sprintf("%d", time.Now().Unix()),
			ChatJID:     "simulation@chat",
			Content:     messageContent,
			SenderPhone: "simulation-sender",
			IsFromMe:    false,
		},
		RealPhone: realPhone,
	}
	
	// Procesar el mensaje (sin guardar en BD ni subir a Supabase)
	result := ProcessingResult{
		MessageID:   simulatedMsg.ID,
		ChatJID:     simulatedMsg.ChatJID,
		Content:     simulatedMsg.Content,
		SenderPhone: simulatedMsg.SenderPhone,
		RealPhone:   simulatedMsg.RealPhone,
		Status:      "processing",
		ProcessedAt: time.Now(),
	}
	
	// Verificar si hay configuración activa de IA
	activeConfig, err := p.aiConfigManager.GetActiveConfig()
	if err != nil {
		if p.aiService == nil {
			result.Status = "error"
			result.ErrorMessage = "No active AI configuration found. Please configure AI settings."
			p.logger.Errorf("No AI service available")
			return result
		}
		p.logger.Warnf("Using legacy AI service")
	}
	
	var aiResponse []byte
	
	// Limpiar contenido del mensaje eliminando caracteres especiales problemáticos
	cleanedContent := cleanMessageContent(messageContent)
	if cleanedContent != messageContent {
		p.logger.Infof("🧹 Contenido limpiado: se eliminaron caracteres especiales en simulación")
	}
	
	// Procesar con IA
	p.logger.Infof("Simulando procesamiento de mensaje")
	
	if activeConfig != nil {
		aiResponse, err = p.aiProviderService.ProcessMessage(p.systemPrompt, cleanedContent, realPhone)
	} else {
		if p.aiService == nil {
			result.Status = "error"
			result.ErrorMessage = "No AI service available"
			return result
		}
		aiResponse, err = p.aiService.ProcessMessage(cleanedContent, realPhone)
	}
	
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("AI processing failed: %v", err)
		p.logger.Errorf("Error en procesamiento IA (simulación): %v", err)
		return result
	}
	
	result.AIResponse = string(aiResponse)
	
	// Validar respuesta de IA
	if err := p.aiProviderService.ValidateResponse(aiResponse); err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Invalid AI response: %v", err)
		p.logger.Errorf("Respuesta de IA inválida (simulación): %v", err)
		return result
	}
	
	// Normalizar respuesta
	normalizedResponse, err := p.aiProviderService.NormalizeResponse(aiResponse)
	if err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Failed to normalize AI response: %v", err)
		p.logger.Errorf("Error normalizando respuesta (simulación): %v", err)
		return result
	}
	
	result.AIResponse = string(normalizedResponse)
	
	// Verificar si el array está vacío
	var cargasTemp []map[string]interface{}
	json.Unmarshal(normalizedResponse, &cargasTemp)
	
	if len(cargasTemp) == 0 {
		result.Status = "success"
		result.ErrorMessage = "No hay información de carga válida en el mensaje (array vacío)"
		return result
	}
	
	// Validar ubicaciones (sin subir a Supabase)
	if err := p.validateLocations(normalizedResponse); err != nil {
		result.Status = "error"
		result.ErrorMessage = fmt.Sprintf("Invalid locations: %v", err)
		result.AIResponse = string(normalizedResponse)
		return result
	}
	
	// En simulación, no subimos a Supabase, solo devolvemos el resultado
	result.Status = "success"
	result.ErrorMessage = fmt.Sprintf("Simulación exitosa: %d carga(s) detectada(s)", len(cargasTemp))
	
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
	
	// Contar total de cargas subidas hoy
	cargasQuery := `
		SELECT SUM(JSON_LENGTH(supabase_ids)) as total_cargas
		FROM ai_processing_results
		WHERE DATE(processed_at) = CURDATE()
		AND status = 'success'
		AND supabase_ids IS NOT NULL
		AND supabase_ids != '[]'
	`
	
	var totalCargas sql.NullInt64
	err = p.messageStore.db.QueryRow(cargasQuery).Scan(&totalCargas)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	
	if totalCargas.Valid {
		stats["total_cargas"] = int(totalCargas.Int64)
	} else {
		stats["total_cargas"] = 0
	}
	
	// Obtener contador de cargas repetidas
	repetidasCount, err := p.supabaseService.ObtenerContadorCargasRepetidas()
	if err != nil {
		// Si hay error, usar 0 como valor por defecto
		stats["cargas_repetidas"] = 0
	} else {
		stats["cargas_repetidas"] = repetidasCount
	}
	
	return stats, nil
}

// GetProcessableMessagesCount obtiene el conteo de mensajes procesables
func (p *MessageProcessor) GetProcessableMessagesCount() (int, error) {
	return p.messageStore.GetProcessableMessagesCount()
}

// GetCargasRepetidasCount obtiene el contador de cargas repetidas
func (p *MessageProcessor) GetCargasRepetidasCount() (int, error) {
	return p.supabaseService.ObtenerContadorCargasRepetidas()
}

// ResetearCargasRepetidasCount resetea el contador de cargas repetidas a 0
func (p *MessageProcessor) ResetearCargasRepetidasCount() error {
	return p.supabaseService.ResetearContadorCargasRepetidas()
}

// ProcessSingleMessage procesa un solo mensaje por ID
func (p *MessageProcessor) ProcessSingleMessage(messageID, chatJID string) (ProcessingResult, error) {
	// Buscar el mensaje directamente por ID, sin importar su estado
	// Esto permite reprocesar mensajes ya procesados o con errores
	targetMessage, err := p.messageStore.GetMessageByID(messageID, chatJID)
	if err != nil {
		return ProcessingResult{}, fmt.Errorf("failed to get message: %v", err)
	}
	
	if targetMessage == nil {
		return ProcessingResult{}, fmt.Errorf("message not found")
	}
	
	p.logger.Infof("Procesando mensaje individual: %s (permitir reprocesar)", messageID)
	
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
		// Todos los errores se marcan como procesados inmediatamente para evitar reintentos
		// que consumen tokens innecesariamente
		p.logger.Warnf("🚫 Error detectado para mensaje %s: %s. Marcando como procesado para evitar reintentos.", targetMessage.ID, result.ErrorMessage)
		if err := p.messageStore.MarkMessageAsFailedAfterRetries(targetMessage.ID, targetMessage.ChatJID, result.ErrorMessage); err != nil {
			p.logger.Errorf("Error marcando mensaje como fallido: %v", err)
		}
	}
	
	p.logger.Infof("🏁 ProcessSingleMessage finalizando - devolviendo resultado con status: %s", result.Status)
	return result, nil
}

// validateLocations valida que las ubicaciones en la respuesta sean válidas
func (p *MessageProcessor) validateLocations(jsonData []byte) error {
	var cargas []map[string]interface{}
	if err := json.Unmarshal(jsonData, &cargas); err != nil {
		return fmt.Errorf("failed to parse JSON for validation: %v", err)
	}
	
	// Palabras inválidas que indican ubicación desconocida
	invalidTerms := []string{
		"desconocida",
		"desconocido",
		"unknown",
		"sin especificar",
		"no especificado",
		"n/a",
		"no disponible",
		"sin datos",
	}
	
	// Ciudades argentinas que contienen nombres de países en su nombre (excepciones)
	argentineCitiesWithCountryNames := []string{
		"concepción del uruguay",  // Entre Ríos, Argentina
		"concepcion del uruguay",
		"chilecito",               // La Rioja, Argentina (contiene "chile")
		"perúgorría",              // Corrientes, Argentina (contiene "perú")
		"perugorria",              // Corrientes, Argentina (sin tilde)
		"perugorría",              // Corrientes, Argentina (variante)
		// Agregar más excepciones aquí si es necesario
	}
	
	// Países NO permitidos (solo Argentina está permitida)
	forbiddenCountries := []string{
		"brasil", "brazil",
		"chile",
		"uruguay",
		"paraguay",
		"bolivia",
		"perú", "peru",
		"ecuador",
		"colombia",
		"venezuela",
		"mexico", "méxico",
	}
	
	for i, carga := range cargas {
		// Validar localidad de carga
		localidadCarga, _ := carga["localidadCarga"].(string)
		if localidadCarga == "" {
			return fmt.Errorf("carga %d: localidadCarga está vacía", i+1)
		}
		
		localidadCargaLower := strings.ToLower(localidadCarga)
		
		// Verificar que sea de Argentina
		if !strings.Contains(localidadCargaLower, "argentina") {
			return fmt.Errorf("carga %d: localidadCarga '%s' no contiene 'Argentina' - solo se procesan ubicaciones argentinas", i+1, localidadCarga)
		}
		
		// Verificar si es una ciudad argentina con nombre de país (excepción)
		isException := false
		for _, cityException := range argentineCitiesWithCountryNames {
			if strings.Contains(localidadCargaLower, cityException) {
				isException = true
				break
			}
		}
		
		// Verificar que NO contenga países prohibidos (solo si no es excepción)
		if !isException {
			for _, country := range forbiddenCountries {
				if strings.Contains(localidadCargaLower, country) {
					return fmt.Errorf("carga %d: localidadCarga contiene '%s' - solo se procesan ubicaciones de Argentina", i+1, country)
				}
			}
		}
		
		// Verificar términos inválidos
		for _, term := range invalidTerms {
			if strings.Contains(localidadCargaLower, term) {
				return fmt.Errorf("carga %d: localidadCarga contiene '%s' - el mensaje no tiene información de ubicación válida", i+1, term)
			}
		}
		
		// Validar localidad de descarga
		localidadDescarga, _ := carga["localidadDescarga"].(string)
		if localidadDescarga == "" {
			return fmt.Errorf("carga %d: localidadDescarga está vacía", i+1)
		}
		
		localidadDescargaLower := strings.ToLower(localidadDescarga)
		
		// Verificar que sea de Argentina
		if !strings.Contains(localidadDescargaLower, "argentina") {
			return fmt.Errorf("carga %d: localidadDescarga '%s' no contiene 'Argentina' - solo se procesan ubicaciones argentinas", i+1, localidadDescarga)
		}
		
		// Verificar si es una ciudad argentina con nombre de país (excepción)
		isExceptionDescarga := false
		for _, cityException := range argentineCitiesWithCountryNames {
			if strings.Contains(localidadDescargaLower, cityException) {
				isExceptionDescarga = true
				break
			}
		}
		
		// Verificar que NO contenga países prohibidos (solo si no es excepción)
		if !isExceptionDescarga {
			for _, country := range forbiddenCountries {
				if strings.Contains(localidadDescargaLower, country) {
					return fmt.Errorf("carga %d: localidadDescarga contiene '%s' - solo se procesan ubicaciones de Argentina", i+1, country)
				}
			}
		}
		
		// Verificar términos inválidos
		for _, term := range invalidTerms {
			if strings.Contains(localidadDescargaLower, term) {
				return fmt.Errorf("carga %d: localidadDescarga contiene '%s' - el mensaje no tiene información de ubicación válida", i+1, term)
			}
		}
	}
	
	return nil
}

// GetProcessedToday obtiene mensajes procesados exitosamente hoy
func (p *MessageProcessor) GetProcessedToday(limit int) ([]ProcessingResult, error) {
	query := `
		SELECT 
			id, message_id, chat_jid, content, sender_phone, real_phone, 
			ai_response, status, error_message, supabase_ids, processed_at
		FROM ai_processing_results
		WHERE status = 'success' 
		  AND DATE(processed_at) = CURDATE()
		ORDER BY processed_at DESC
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
		var errorMsg sql.NullString
		var aiResponse sql.NullString
		
		err := rows.Scan(
			&result.ID, &result.MessageID, &result.ChatJID, &result.Content,
			&result.SenderPhone, &result.RealPhone, &aiResponse, &result.Status,
			&errorMsg, &supabaseIDsJSON, &result.ProcessedAt,
		)
		if err != nil {
			return nil, err
		}
		
		if aiResponse.Valid {
			result.AIResponse = aiResponse.String
		}
		if errorMsg.Valid {
			result.ErrorMessage = errorMsg.String
		}
		if supabaseIDsJSON.Valid && supabaseIDsJSON.String != "" {
			json.Unmarshal([]byte(supabaseIDsJSON.String), &result.SupabaseIDs)
		}
		
		results = append(results, result)
	}
	
	return results, nil
}

// GetMessagesWithErrors obtiene mensajes que tuvieron errores
func (p *MessageProcessor) GetMessagesWithErrors(limit int) ([]ProcessingResult, error) {
	query := `
		SELECT 
			id, message_id, chat_jid, content, sender_phone, real_phone, 
			ai_response, status, error_message, supabase_ids, processed_at
		FROM ai_processing_results
		WHERE status = 'error'
		ORDER BY processed_at DESC
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
		var errorMsg sql.NullString
		var aiResponse sql.NullString
		
		err := rows.Scan(
			&result.ID, &result.MessageID, &result.ChatJID, &result.Content,
			&result.SenderPhone, &result.RealPhone, &aiResponse, &result.Status,
			&errorMsg, &supabaseIDsJSON, &result.ProcessedAt,
		)
		if err != nil {
			return nil, err
		}
		
		if aiResponse.Valid {
			result.AIResponse = aiResponse.String
		}
		if errorMsg.Valid {
			result.ErrorMessage = errorMsg.String
		}
		if supabaseIDsJSON.Valid && supabaseIDsJSON.String != "" {
			json.Unmarshal([]byte(supabaseIDsJSON.String), &result.SupabaseIDs)
		}
		
		results = append(results, result)
	}
	
	return results, nil
}

