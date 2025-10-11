# 🔄 Migración: Columnas de Procesamiento

## 📋 Descripción

Esta migración agrega columnas a la tabla `messages` para soportar el sistema de reintentos de procesamiento con IA.

## ✅ Solución Automática

La aplicación ahora **ejecuta automáticamente** las migraciones al iniciar. No necesitas hacer nada manualmente.

### Cómo Funciona

Al iniciar `NewMessageStore()`:

1. Se crean las tablas si no existen
2. Se crean los índices si no existen
3. **Se ejecuta `runMigrations()`** que agrega columnas faltantes
4. Si las columnas ya existen, se ignora el error

## 📊 Columnas Agregadas

### 1. `processing_attempts`

- **Tipo**: `INT DEFAULT 0`
- **Propósito**: Contador de intentos de procesamiento
- **Rango**: 0 a 3 (máximo)

### 2. `last_processing_error`

- **Tipo**: `TEXT`
- **Propósito**: Almacenar el mensaje de error del último intento fallido
- **Uso**: Debugging y análisis

### 3. `last_processing_attempt`

- **Tipo**: `TIMESTAMP NULL`
- **Propósito**: Fecha y hora del último intento de procesamiento
- **Uso**: Identificar mensajes "atascados"

## 🔧 Implementación

### Código en `whatsapp_service.go`

```go
func runMigrations(db *sql.DB) error {
    columns := []struct {
        name         string
        definition   string
    }{
        {"processing_attempts", "INT DEFAULT 0"},
        {"last_processing_error", "TEXT"},
        {"last_processing_attempt", "TIMESTAMP NULL"},
    }

    for _, col := range columns {
        alterQuery := fmt.Sprintf("ALTER TABLE messages ADD COLUMN %s %s", col.name, col.definition)
        _, err := db.Exec(alterQuery)

        // Ignorar si ya existe
        if err != nil && !isColumnExistsError(err) {
            return fmt.Errorf("failed to add column %s: %v", col.name, err)
        }
    }

    return nil
}
```

### Detección de Errores

La función `isColumnExistsError()` detecta:

- "Duplicate column name" (MariaDB/MySQL)
- "column already exists"
- Error code 1060

## 🚀 Ejecución

### Primera vez (columnas no existen):

```
✅ ALTER TABLE messages ADD COLUMN processing_attempts INT DEFAULT 0
✅ ALTER TABLE messages ADD COLUMN last_processing_error TEXT
✅ ALTER TABLE messages ADD COLUMN last_processing_attempt TIMESTAMP NULL
```

### Ejecuciones siguientes (columnas ya existen):

```
ℹ️ ALTER TABLE messages ADD COLUMN processing_attempts... (ignorado - ya existe)
ℹ️ ALTER TABLE messages ADD COLUMN last_processing_error... (ignorado - ya existe)
ℹ️ ALTER TABLE messages ADD COLUMN last_processing_attempt... (ignorado - ya existe)
```

## 🔍 Verificación Manual

Si quieres verificar que las columnas se agregaron correctamente:

```sql
DESCRIBE messages;
```

Deberías ver:

```
+---------------------------+---------------+------+-----+-------------------+
| Field                     | Type          | Null | Key | Default           |
+---------------------------+---------------+------+-----+-------------------+
| id                        | varchar(255)  | NO   | PRI | NULL              |
| chat_jid                  | varchar(255)  | NO   | PRI | NULL              |
| sender_phone              | varchar(100)  | YES  |     | NULL              |
| sender_name               | varchar(500)  | YES  |     | NULL              |
| content                   | text          | YES  |     | NULL              |
| timestamp                 | timestamp     | YES  |     | NULL              |
| is_from_me                | tinyint(1)    | YES  |     | 0                 |
| media_type                | varchar(100)  | YES  |     | NULL              |
| filename                  | varchar(500)  | YES  |     | NULL              |
| url                       | varchar(1000) | YES  |     | NULL              |
| media_key                 | longblob      | YES  |     | NULL              |
| file_sha256               | longblob      | YES  |     | NULL              |
| file_enc_sha256           | longblob      | YES  |     | NULL              |
| file_length               | bigint(20)    | YES  |     | NULL              |
| processed                 | tinyint(1)    | YES  |     | 0                 |
| processing_attempts       | int(11)       | YES  |     | 0                 | ← Nueva
| last_processing_error     | text          | YES  |     | NULL              | ← Nueva
| last_processing_attempt   | timestamp     | YES  |     | NULL              | ← Nueva
+---------------------------+---------------+------+-----+-------------------+
```

## 🛠️ Solución Manual (Solo si falla la automática)

Si por alguna razón la migración automática falla, puedes ejecutar manualmente:

```sql
ALTER TABLE messages ADD COLUMN processing_attempts INT DEFAULT 0;
ALTER TABLE messages ADD COLUMN last_processing_error TEXT;
ALTER TABLE messages ADD COLUMN last_processing_attempt TIMESTAMP NULL;
```

## ⚠️ Notas Importantes

1. **No afecta datos existentes**: Solo agrega columnas
2. **Valores por defecto**: Los mensajes existentes tendrán `processing_attempts = 0`
3. **Idempotente**: Se puede ejecutar múltiples veces sin problemas
4. **Sin downtime**: La migración es instantánea
5. **Retrocompatible**: Los mensajes antiguos funcionan normalmente

## 📈 Impacto

### Antes de la migración:

```sql
SELECT id, content, processed FROM messages LIMIT 1;
```

### Después de la migración:

```sql
SELECT id, content, processed, processing_attempts, last_processing_error
FROM messages LIMIT 1;
```

## 🎯 Próximos Pasos

1. Reinicia la aplicación
2. Las columnas se agregarán automáticamente
3. El sistema de reintentos funcionará correctamente
4. Podrás ver intentos en la tabla de resultados
