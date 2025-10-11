# 🔄 Estrategia de Reintentos para Procesamiento de Mensajes

## 📋 Descripción

El sistema implementa una estrategia inteligente de reintentos para manejar errores en el procesamiento de mensajes con IA, evitando loops infinitos y permitiendo recuperación automática de errores transitorios.

## 🎯 Objetivo

- ✅ Reintentar mensajes que fallaron por errores temporales (API rate limit, timeout, etc.)
- ✅ NO reintentar indefinidamente para evitar desperdicio de recursos
- ✅ Registrar intentos y errores para debugging
- ✅ Marcar como "procesado" después de 3 intentos fallidos

## 🔢 Campos Agregados a la Tabla `messages`

```sql
processing_attempts INT DEFAULT 0
last_processing_error TEXT
last_processing_attempt TIMESTAMP NULL
```

### `processing_attempts`

- Contador de intentos de procesamiento
- Se incrementa con cada error
- Máximo: 3 intentos

### `last_processing_error`

- Mensaje de error del último intento fallido
- Útil para debugging y análisis

### `last_processing_attempt`

- Timestamp del último intento de procesamiento
- Permite identificar mensajes "atascados"

## 📊 Flujo de Procesamiento

### 1. Selección de Mensajes

```
GetProcessableMessages() filtra:
- processed = 0 (no procesados)
- content IS NOT NULL (con texto)
- real_phone asociado
- processing_attempts < 3 (menos de 3 intentos)
```

### 2. Procesamiento de Mensaje

#### Si es exitoso (status = "success"):

```
1. Guardar resultado en ai_processing_results
2. Marcar mensaje como processed = 1
3. ✅ No se volverá a procesar
```

#### Si falla (status = "error"):

```
1. Guardar resultado en ai_processing_results
2. Incrementar processing_attempts
3. Guardar last_processing_error
4. Actualizar last_processing_attempt
5. Mensaje queda con processed = 0
```

#### Después del 3er intento fallido:

```
1. processing_attempts = 3
2. Mensaje queda fuera del filtro de GetProcessableMessages()
3. ❌ No se volverá a procesar automáticamente
4. Puede ser reprocesado manualmente si se resetea processing_attempts
```

## 🔍 Tipos de Errores y Estrategia

### Errores Transitorios (Se Reintenta)

- ⏱️ **Timeout de API**: Intenta 3 veces
- 🚦 **Rate Limit (429)**: Intenta 3 veces (espera 5 min entre lotes)
- 🌐 **Error de Red**: Intenta 3 veces
- 💾 **Error de Base de Datos**: Intenta 3 veces

### Errores Permanentes (Deberían marcarse como procesados después de 3 intentos)

- 🔑 **API Key inválida**: Se detiene después de 3 intentos
- 📝 **JSON inválido de IA**: Se detiene después de 3 intentos
- 🚫 **Mensaje no tiene información de carga**: Se detiene después de 3 intentos
- ❌ **Error de validación de datos**: Se detiene después de 3 intentos

## 📈 Métricas y Monitoreo

### Consulta SQL para ver mensajes con errores:

```sql
SELECT
    id,
    sender_phone,
    processing_attempts,
    last_processing_error,
    last_processing_attempt,
    LEFT(content, 50) as content_preview
FROM messages
WHERE processing_attempts > 0
  AND processed = 0
ORDER BY last_processing_attempt DESC;
```

### Consulta para ver mensajes que alcanzaron el máximo:

```sql
SELECT
    COUNT(*) as total_failed,
    sender_phone,
    last_processing_error
FROM messages
WHERE processing_attempts >= 3
  AND processed = 0
GROUP BY sender_phone, last_processing_error;
```

## 🔧 Configuración

### Límite de Reintentos

Definido en la query SQL:

```sql
AND (m.processing_attempts < 3 OR m.processing_attempts IS NULL)
```

Para cambiar el límite de 3 a otro valor:

1. Modificar la query en `GetProcessableMessages()` en `whatsapp_service.go`
2. No requiere cambios en la base de datos

### Tiempo entre Reintentos

- **Automático**: 5 minutos (intervalo del background processor)
- **Manual**: Inmediato (botón en UI)

## 🛠️ Funciones Implementadas

### En `MessageStore`:

#### `IncrementProcessingAttempt(messageID, chatJID, errorMsg)`

- Incrementa el contador de intentos
- Guarda el mensaje de error
- Actualiza el timestamp del último intento

#### `MarkMessageAsProcessed(messageID, chatJID)`

- Marca como procesado exitosamente
- No se volverá a procesar

### En `MessageProcessor`:

#### Lógica de reintentos automática:

```go
if result.Status == "success" {
    // Marcar como procesado
    messageStore.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
} else if result.Status == "error" {
    // Incrementar intentos y registrar error
    messageStore.IncrementProcessingAttempt(msg.ID, msg.ChatJID, result.ErrorMessage)
}
```

## 📱 Interfaz de Usuario

### Visualización en Frontend:

- ✅ Columna "Intentos" en la tabla de resultados
- ⚠️ Indicador visual para mensajes con múltiples intentos
- 🔴 Color diferente para mensajes que alcanzaron el máximo

### Acciones Manuales (futuro):

- 🔄 Botón para resetear contador de intentos
- 🔍 Ver detalles de errores de cada intento
- 📊 Dashboard con estadísticas de reintentos

## 🎛️ Ajustes Recomendados

### Para Desarrollo:

- Límite de 2 intentos
- Ver logs detallados de cada intento

### Para Producción:

- Límite de 3 intentos
- Alertas cuando múltiples mensajes alcanzan el máximo
- Dashboard de monitoreo de tasas de error

## 🚨 Alertas y Notificaciones

### Casos que requieren atención:

1. **Múltiples mensajes con 3 intentos**: Problema con API o configuración
2. **Mismo error en todos los mensajes**: Revisar API key o prompt
3. **Mensajes antiguos sin procesar**: Posible problema de asociación de teléfono

## 🔮 Mejoras Futuras

1. **Backoff Exponencial**: Esperar más tiempo entre cada intento (1min, 5min, 15min)
2. **Reintentos Inteligentes**: Detectar tipo de error y ajustar estrategia
3. **Rotación Automática de Keys**: Cambiar de API key después de X errores
4. **Notificaciones**: Alertar al usuario cuando un mensaje falla 3 veces
5. **Reprocesamiento Manual**: UI para resetear y reprocesar mensajes fallidos

## 📝 Notas Importantes

- ⚠️ Los mensajes NO se marcan como procesados automáticamente después de 3 intentos
- ⚠️ Simplemente dejan de aparecer en la cola de procesamiento
- ✅ Esto permite reprocesarlos manualmente si se corrige el problema
- 💾 El historial de intentos y errores se mantiene en la base de datos
