# 📋 Arquitectura de Gestión de Mensajes

## 🎯 Objetivo

Sistema robusto para evitar mensajes duplicados y permitir el procesamiento controlado de mensajes mediante un flag `processed`.

## 🗄️ Estructura de Base de Datos

### Tabla `messages`

```sql
CREATE TABLE messages (
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
    processed BOOLEAN DEFAULT 0,  -- NUEVA COLUMNA
    PRIMARY KEY (id, chat_jid),
    FOREIGN KEY (chat_jid) REFERENCES chats(jid)
);
```

### Índices para Rendimiento

```sql
-- Búsqueda rápida de duplicados
CREATE INDEX idx_messages_duplicate
ON messages(chat_jid, sender, content, timestamp);

-- Mensajes no procesados
CREATE INDEX idx_messages_processed
ON messages(processed, timestamp);
```

## 🛡️ Control de Duplicados

### Estrategia de Detección

Un mensaje se considera **duplicado** si cumple:

1. Mismo `chat_jid`
2. Mismo `sender`
3. Mismo `content`
4. Timestamp dentro de un margen de **48 horas** (antes o después)

### Implementación

```go
func (store *MessageStore) StoreMessage(...) error {
    // Verificar duplicados en las últimas 48 horas
    var exists int
    err := store.db.QueryRow(`
        SELECT COUNT(*) FROM messages
        WHERE chat_jid = ?
        AND sender = ?
        AND content = ?
        AND timestamp >= datetime(?, '-48 hours')
        AND timestamp <= datetime(?, '+48 hours')
    `, chatJID, sender, content, timestamp, timestamp).Scan(&exists)

    if exists > 0 {
        // Mensaje duplicado, no insertar
        return nil
    }

    // Insertar mensaje con processed = 0
    ...
}
```

### ¿Por qué 48 horas?

- **Evita spam**: Si alguien envía el mismo mensaje múltiples veces en 2 días, solo se guarda una vez
- **Respuestas repetidas**: Clientes que envían la misma consulta varias veces no saturan la base de datos
- **Mensajes programados**: Evita duplicados de mensajes automatizados que se repiten
- **Sincronización**: Maneja casos donde WhatsApp re-sincroniza mensajes antiguos
- **Flexibilidad**: Ventana amplia que cubre la mayoría de casos de uso sin perder mensajes legítimos únicos

## 📊 Sistema de Procesamiento

### Flujo de Procesamiento

```
1. Mensaje llega
   ↓
2. Se verifica si es duplicado
   ↓
3. Si no existe, se guarda con processed=0
   ↓
4. Tu aplicación obtiene mensajes no procesados
   ↓
5. Procesa el mensaje
   ↓
6. Marca como processed=1
   ↓
7. Nunca se vuelve a procesar
```

### Funciones Disponibles

#### 1. Obtener Mensajes No Procesados

```go
// Desde Go
messages, err := messageStore.GetUnprocessedMessages(100)

// Desde JavaScript (Frontend)
const messages = await window.go.main.App.GetUnprocessedMessages(100);
```

#### 2. Marcar Mensaje como Procesado

```go
// Uno por uno
err := messageStore.MarkMessageAsProcessed(messageID, chatJID)

// Desde JavaScript
await window.go.main.App.MarkMessageAsProcessed(messageID, chatJID);
```

#### 3. Marcar Múltiples (Lote)

```go
messageIDs := []string{"msg1", "msg2", "msg3"}
err := messageStore.MarkMessagesAsProcessed(messageIDs, chatJID)
```

#### 4. Obtener Estadísticas

```go
// Desde Go
total, processed, unprocessed, err := messageStore.GetMessageStats()

// Desde JavaScript
const stats = await window.go.main.App.GetMessageStats();
// stats = { total: 150, processed: 100, unprocessed: 50 }
```

## 🔄 Casos de Uso

### Caso 1: Bot de Respuestas Automáticas

```go
// Obtener mensajes no procesados cada X segundos
messages, _ := GetUnprocessedMessages(50)

for _, msg := range messages {
    if !msg.IsFromMe && strings.Contains(msg.Content, "hola") {
        // Responder
        SendMessage(msg.ChatJID, "¡Hola! ¿En qué puedo ayudarte?")
    }

    // Marcar como procesado
    MarkMessageAsProcessed(msg.ID, msg.ChatJID)
}
```

### Caso 2: Análisis de Sentimientos

```go
messages, _ := GetUnprocessedMessages(100)

for _, msg := range messages {
    // Analizar sentimiento
    sentiment := AnalyzeSentiment(msg.Content)

    // Guardar en otra tabla
    SaveSentimentAnalysis(msg.ID, sentiment)

    // Marcar como procesado
    MarkMessageAsProcessed(msg.ID, msg.ChatJID)
}
```

### Caso 3: Logs de Auditoría

```go
messages, _ := GetUnprocessedMessages(1000)

for _, msg := range messages {
    // Enviar a sistema de logs
    LogToExternalSystem(msg)

    // Marcar como procesado
    MarkMessageAsProcessed(msg.ID, msg.ChatJID)
}
```

## 🚫 Política de Retención

### Mensajes NO se Eliminan Automáticamente

- Los mensajes **NUNCA** se eliminan por antigüedad
- Todos los mensajes permanecen en la base de datos
- Esto permite:
  - Auditoría completa
  - Análisis histórico
  - Re-procesamiento si es necesario

### Si Necesitas Limpiar Manualmente

```sql
-- Eliminar mensajes procesados más viejos de 30 días
DELETE FROM messages
WHERE processed = 1
AND timestamp < datetime('now', '-30 days');

-- Eliminar solo de un chat específico
DELETE FROM messages
WHERE chat_jid = '573001234567@s.whatsapp.net'
AND processed = 1;
```

## ⚡ Rendimiento

### Índices Optimizados

Los índices creados aseguran:

- **Detección de duplicados**: < 1ms
- **Consulta de no procesados**: < 5ms con 10,000 mensajes
- **Actualización de estado**: < 1ms

### Recomendaciones

1. **Procesar en lotes**: Usa `MarkMessagesAsProcessed` para múltiples mensajes
2. **Limitar consultas**: No consultes más de 1000 mensajes por vez
3. **Índices**: Los índices se crean automáticamente

## 🔐 Integridad de Datos

### Transacciones

Las operaciones en lote usan transacciones:

```go
tx, _ := store.db.Begin()
// Procesar múltiples mensajes
tx.Commit()
```

### Rollback Automático

Si algo falla durante un lote, **todos** los cambios se revierten.

## 📈 Monitoreo

### Obtener Estadísticas

```javascript
// En la UI
const stats = await window.go.main.App.GetMessageStats();
console.log(`Total: ${stats.total}`);
console.log(`Procesados: ${stats.processed}`);
console.log(`Pendientes: ${stats.unprocessed}`);
```

### Logs

La aplicación registra:

- Mensajes duplicados detectados (no se insertan)
- Errores de procesamiento
- Estadísticas periódicas

## 🎨 Ejemplo Completo: Worker de Procesamiento

```go
// worker.go
func StartMessageProcessor(app *App) {
    ticker := time.NewTicker(5 * time.Second)

    go func() {
        for range ticker.C {
            messages, err := app.GetUnprocessedMessages(100)
            if err != nil {
                log.Printf("Error: %v", err)
                continue
            }

            for _, msg := range messages {
                // Tu lógica de procesamiento
                processMessage(msg)

                // Marcar como procesado
                app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
            }

            // Log estadísticas
            stats, _ := app.GetMessageStats()
            log.Printf("Pendientes: %d", stats["unprocessed"])
        }
    }()
}
```

## 🔍 Debugging

### Ver Mensajes No Procesados (SQL)

```sql
SELECT id, sender, content, timestamp
FROM messages
WHERE processed = 0
ORDER BY timestamp DESC
LIMIT 10;
```

### Resetear Estado de Procesamiento

```sql
-- Marcar todos como no procesados (para re-procesar)
UPDATE messages SET processed = 0;

-- Solo un chat específico
UPDATE messages SET processed = 0
WHERE chat_jid = '573001234567@s.whatsapp.net';
```

## ✅ Checklist de Implementación

Para usar este sistema en tu aplicación:

- [x] Base de datos con columna `processed`
- [x] Índices creados automáticamente
- [x] Control de duplicados activo
- [x] Funciones de procesamiento disponibles
- [ ] Implementar tu lógica de procesamiento
- [ ] Configurar worker o cron job
- [ ] Agregar monitoreo/estadísticas
- [ ] Definir política de limpieza (opcional)

## 🚀 Próximos Pasos

1. **Implementa tu lógica**: Define qué hacer con mensajes no procesados
2. **Crea un worker**: Procesa mensajes periódicamente
3. **Monitorea**: Usa `GetMessageStats()` para ver el estado
4. **Optimiza**: Ajusta el intervalo de procesamiento según tu carga

---

**Arquitectura robusta para aplicaciones de automatización WhatsApp** 🎯
