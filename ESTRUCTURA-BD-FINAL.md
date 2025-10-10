# 📊 Estructura Final de Base de Datos

## 🗄️ Tabla `messages`

```sql
CREATE TABLE messages (
    id TEXT,                    -- ID único del mensaje de WhatsApp
    chat_jid TEXT,              -- JID del chat (grupo o individual)
    sender_phone TEXT,          -- Número de teléfono O LID si tiene privacidad
    sender_name TEXT,           -- PushName del remitente
    content TEXT,               -- Contenido del mensaje
    timestamp TIMESTAMP,        -- Fecha y hora del mensaje
    is_from_me BOOLEAN,         -- Si lo enviaste tú
    media_type TEXT,            -- Tipo de media (image, video, audio, document)
    filename TEXT,              -- Nombre del archivo de media
    url TEXT,                   -- URL del media en WhatsApp
    media_key BLOB,             -- Clave de encriptación del media
    file_sha256 BLOB,           -- Hash del archivo
    file_enc_sha256 BLOB,       -- Hash del archivo encriptado
    file_length INTEGER,        -- Tamaño del archivo
    processed BOOLEAN DEFAULT 0, -- Flag de procesamiento
    PRIMARY KEY (id, chat_jid)
);
```

## 📋 Campos Clave

### `sender_phone`

**Propósito:** Identificador único del remitente

**Valores posibles:**

- `"573001234567"` - Número de teléfono real (usuario sin privacidad)
- `"21496412029002"` - LID (usuario con privacidad activada)

**Nota:** Si es LID (> 15 dígitos), NO es el número real.

### `sender_name`

**Propósito:** Nombre legible del remitente

**Valores posibles:**

- `"Hernan Parino"` - PushName del contacto
- `"Juan Pérez"` - Nombre configurado en WhatsApp
- `"Yo"` - Para mensajes propios

## 🔍 Ejemplos de Datos

### Mensaje de Usuario Normal (Sin Privacidad)

```sql
id:           '3EB0ABCD1234567890'
chat_jid:     '120363039914586861@g.us'
sender_phone: '573001234567'           ← Número real
sender_name:  'Juan Pérez'
content:      'Hola, ¿cómo estás?'
timestamp:    '2025-01-08 10:00:00'
is_from_me:   0
processed:    0
```

### Mensaje de Usuario con Privacidad (LID)

```sql
id:           'AC1BB447C7A1A7C3E59A'
chat_jid:     '120363039914586861@g.us'
sender_phone: '21496412029002'         ← LID (NO es número real)
sender_name:  'Hernan Parino'
content:      'Muño'
timestamp:    '2025-01-08 01:01:32'
is_from_me:   0
processed:    0
```

### Mensaje Propio

```sql
id:           'XYZ789ABC123DEF456'
chat_jid:     '120363039914586861@g.us'
sender_phone: '573009876543'           ← Tu número
sender_name:  'Yo'
content:      'Perfecto!'
timestamp:    '2025-01-08 10:05:00'
is_from_me:   1
processed:    1
```

## 🛡️ Detección de Duplicados

### Query de Verificación

```sql
SELECT COUNT(*) FROM messages
WHERE chat_jid = ?
AND sender_phone = ?              ← Compara por teléfono/LID
AND content = ?
AND timestamp >= datetime(?, '-48 hours')
AND timestamp <= datetime(?, '+48 hours')
```

### Ejemplos

#### Caso 1: Usuario Normal Repite Mensaje

```sql
-- Mensaje 1
sender_phone: '573001234567'
content: 'Hola'
timestamp: '2025-01-08 10:00'

-- Mensaje 2 (DUPLICADO - SE DESCARTA)
sender_phone: '573001234567'  ← Mismo
content: 'Hola'                ← Mismo
timestamp: '2025-01-08 12:00'  ← Dentro de 48h
```

#### Caso 2: Usuario LID Repite Mensaje

```sql
-- Mensaje 1
sender_phone: '21496412029002'  ← LID
sender_name: 'Hernan Parino'
content: 'Muño'
timestamp: '2025-01-08 01:01'

-- Mensaje 2 (DUPLICADO - SE DESCARTA)
sender_phone: '21496412029002'  ← Mismo LID
content: 'Muño'                  ← Mismo
timestamp: '2025-01-08 15:00'    ← Dentro de 48h
```

## 📊 Queries Útiles

### Ver Todos los Usuarios LID

```sql
SELECT DISTINCT sender_phone, sender_name
FROM messages
WHERE LENGTH(sender_phone) > 15
ORDER BY sender_name;
```

### Ver Mensajes No Procesados con Detalles

```sql
SELECT
    sender_name,
    sender_phone,
    CASE
        WHEN LENGTH(sender_phone) > 15 THEN 'LID (Privado)'
        ELSE 'Número Real'
    END as tipo,
    content,
    timestamp
FROM messages
WHERE processed = 0
ORDER BY timestamp DESC
LIMIT 20;
```

### Estadísticas de Privacidad

```sql
SELECT
    COUNT(*) as total_mensajes,
    SUM(CASE WHEN LENGTH(sender_phone) > 15 THEN 1 ELSE 0 END) as con_lid,
    SUM(CASE WHEN LENGTH(sender_phone) <= 15 THEN 1 ELSE 0 END) as con_numero
FROM messages
WHERE is_from_me = 0;
```

## 🔍 Índices Creados

```sql
-- Búsqueda de duplicados por teléfono/LID
idx_messages_duplicate_phone
ON messages(chat_jid, sender_phone, content, timestamp)

-- Búsqueda por nombre
idx_messages_sender_name
ON messages(sender_name, timestamp)

-- Mensajes no procesados
idx_messages_processed
ON messages(processed, timestamp)
```

## 🎯 Acceso desde la Aplicación

### Desde Go

```go
messages, _ := messageStore.GetUnprocessedMessages(100)

for _, msg := range messages {
    fmt.Printf("Teléfono/LID: %s\n", msg.SenderPhone)
    fmt.Printf("Nombre: %s\n", msg.SenderName)

    // Verificar si es LID
    if len(msg.SenderPhone) > 15 {
        fmt.Printf("⚠️ Usuario con privacidad (LID)\n")
    } else {
        fmt.Printf("✅ Número real: +%s\n", msg.SenderPhone)
    }
}
```

### Desde JavaScript

```javascript
const messages = await window.go.main.App.GetUnprocessedMessages(100);

messages.forEach((msg) => {
  console.log("Teléfono/LID:", msg.sender_phone);
  console.log("Nombre:", msg.sender_name);

  if (msg.sender_phone.length > 15) {
    console.log("⚠️ Usuario con privacidad");
  } else {
    console.log("✅ Número real: +" + msg.sender_phone);
  }
});
```

## ✅ Ventajas de Esta Estructura

1. **Captura TODO** lo disponible:

   - ✅ Número real cuando está disponible
   - ✅ LID cuando el número está oculto
   - ✅ Nombre del remitente siempre

2. **Identificación única**:

   - `sender_phone` es único (número o LID)
   - Perfecto para detectar duplicados

3. **Legibilidad**:

   - `sender_name` para mostrar en UI
   - Fácil de leer para humanos

4. **Flexibilidad**:
   - Puedes filtrar/procesar por nombre o teléfono
   - Sabes cuándo es LID (privacidad) vs número real

---

**Ahora capturas TODA la información disponible: número/LID + nombre** 🎯✅
