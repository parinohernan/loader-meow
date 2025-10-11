# 🎛️ Acciones de Gestión de Mensajes

## 📋 Descripción

Sistema completo de gestión de mensajes procesados con acciones para ver, editar, reprocesar y eliminar.

## 🔧 Acciones Disponibles

### 1. 👁️ Ver Detalles

**Función**: `viewProcessingDetails(resultId)`

Muestra un modal con toda la información del procesamiento:

- ✅ Estado del procesamiento
- 📱 Remitente (sender_phone y real_phone)
- 🔢 Intentos de procesamiento (X/3)
- 📝 Contenido completo del mensaje
- ❌ Mensaje de error (si hubo error)
- 🤖 Respuesta completa de IA (JSON formateado)
- 📦 IDs de cargas creadas en Supabase

**Características**:

- Scroll automático si el contenido es largo
- Cierra con ESC o click fuera del modal
- Respuesta de IA con scroll independiente

### 2. ✏️ Editar Mensaje

**Función**: `editMessage(messageID, chatJID)`

Permite editar el contenido del mensaje antes de reprocesar:

- 📝 Editor de texto grande con scroll
- 💾 Guarda el nuevo contenido en la base de datos
- 🔄 Resetea automáticamente el contador de intentos
- ✅ Marca el mensaje como no procesado para reprocesar

**Flujo**:

1. Click en "✏️"
2. Se abre modal con el contenido actual en un textarea
3. Editas el mensaje
4. Click en "💾 Guardar y Reprocesar"
5. Se actualiza en BD y resetea `processing_attempts = 0`, `processed = 0`
6. El mensaje aparecerá en la próxima cola de procesamiento

**Casos de uso**:

- Corregir errores de ortografía
- Agregar información faltante
- Reformular mensajes confusos
- Mejorar el formato para que IA lo entienda mejor

### 3. 🔄 Reprocesar Mensaje

**Función**: `reprocessMessage(messageID, chatJID)`

Marca un mensaje para reprocesar sin editar el contenido:

- 🔁 Resetea `processing_attempts` a 0
- ✅ Marca `processed = 0`
- 🗑️ Limpia errores anteriores
- 📅 Limpia `last_processing_attempt`

**Cuándo usar**:

- El mensaje falló por error temporal (timeout, rate limit)
- Ya agregaste una nueva API key
- El error fue de Supabase/geocoding y ya se solucionó
- Quieres darle otra oportunidad sin editar

**Confirmación**: Pide confirmación antes de ejecutar

### 4. 🗑️ Eliminar Mensaje

**Función**: `deleteMessage(messageID, chatJID)`

Elimina completamente el mensaje de la base de datos:

- ❌ Elimina de la tabla `messages`
- 🗑️ También elimina de `ai_processing_results` (CASCADE)
- ⚠️ **Acción irreversible**

**Cuándo usar**:

- Mensaje de prueba que no debe procesarse
- Spam o mensajes basura
- Mensajes duplicados que pasaron el filtro
- Información incorrecta que no se puede corregir

**Confirmación**: Pide confirmación doble antes de ejecutar

## 🎨 Interfaz Visual

### Tabla de Resultados

```
┌────────┬─────────┬──────────┬────────┬──────────┬────────┬──────────────────┐
│ Fecha  │ Mensaje │ Remitente│ Estado │ Intentos │ Cargas │ Acciones         │
├────────┼─────────┼──────────┼────────┼──────────┼────────┼──────────────────┤
│ 10/11  │ Tengo..│ +549...  │ ✅ Exit│   0/3    │   2    │ 👁️ ✏️ 🔄 🗑️    │
│ 10/11  │ Neces..│ +549...  │ ❌ Error│  2/3    │   0    │ 👁️ ✏️ 🔄 🗑️    │
│ 10/11  │ Busco..│ +549...  │ ❌ Error│  3/3    │   0    │ 👁️ ✏️ 🔄 🗑️    │
└────────┴─────────┴──────────┴────────┴──────────┴────────┴──────────────────┘
```

### Columna de Intentos

- **0/3**: Color normal (#e9edef)
- **1/3 o 2/3**: Color amarillo (#ffc107) - Advertencia
- **3/3**: Color rojo (#f15c6d) - Máximo alcanzado

### Botones de Acción

- **👁️**: Fondo gris oscuro, hover agrandado
- **✏️**: Fondo gris oscuro, hover agrandado
- **🔄**: Fondo gris oscuro, hover agrandado
- **🗑️**: Fondo rojo, hover más oscuro

## 💾 Funciones Backend

### `ResetProcessingAttempts(messageID, chatJID)`

```sql
UPDATE messages SET
  processing_attempts = 0,
  processed = 0,
  last_processing_error = NULL,
  last_processing_attempt = NULL
WHERE id = ? AND chat_jid = ?
```

### `DeleteMessage(messageID, chatJID)`

```sql
DELETE FROM messages
WHERE id = ? AND chat_jid = ?
```

- Activa CASCADE en `ai_processing_results`

### `UpdateMessageContent(messageID, chatJID, newContent)`

```sql
UPDATE messages SET
  content = ?,
  processing_attempts = 0,
  processed = 0,
  last_processing_error = NULL
WHERE id = ? AND chat_jid = ?
```

### `GetMessageDetails(messageID, chatJID)`

- Busca el mensaje en la BD
- Retorna objeto `ChatMessage` completo

## 🔄 Flujo de Reprocesamiento

### Opción 1: Reprocesar sin editar

```
1. Click en 🔄
2. Confirmar
3. Se resetea processing_attempts a 0
4. Mensaje aparece en cola de procesamiento
5. Se procesa en el próximo ciclo (5 min o manual)
```

### Opción 2: Editar y reprocesar

```
1. Click en ✏️
2. Editar contenido en modal
3. Click en "💾 Guardar y Reprocesar"
4. Se actualiza content y resetea processing_attempts
5. Mensaje aparece en cola de procesamiento
6. Se procesa en el próximo ciclo
```

## 🎯 Casos de Uso Comunes

### Mensaje con error de IA

1. Ver detalles (👁️) → Leer error
2. Si es error de formato → Editar (✏️) → Guardar
3. Si es error temporal → Reprocesar (🔄)

### Mensaje con error de Supabase

1. Ver detalles (👁️) → Verificar respuesta de IA
2. Si IA está OK → Reprocesar (🔄)
3. Si IA está mal → Editar (✏️) o Eliminar (🗑️)

### Mensaje de prueba

1. Eliminar directamente (🗑️)

### Alcanzó máximo de intentos (3/3)

1. Ver detalles (👁️) → Analizar error
2. Editar (✏️) → Corregir → Guardar
3. O Eliminar (🗑️) si no tiene solución

## 🔒 Seguridad

### Confirmaciones

- ✅ **Reprocesar**: Confirmación simple
- ⚠️ **Eliminar**: Confirmación con advertencia de irreversible
- ✅ **Editar**: Sin confirmación (se puede cancelar)

### Validaciones

- Campo de contenido no puede estar vacío al editar
- Validación de messageID y chatJID en backend
- Manejo de errores con notificaciones visuales

## 📊 Notificaciones

Todas las acciones muestran notificaciones:

- ✅ Verde: Acción exitosa
- ❌ Rojo: Error
- ⚠️ Amarillo: Advertencia

## 🚀 Mejoras Futuras

1. **Edición en lote**: Editar múltiples mensajes a la vez
2. **Reprocesar en lote**: Reprocesar todos los mensajes con error
3. **Filtros avanzados**: Filtrar por estado, intentos, fecha
4. **Exportar resultados**: Descargar tabla como CSV
5. **Historial de ediciones**: Ver cambios anteriores del mensaje
6. **Comparación**: Ver diferencias antes/después de editar
