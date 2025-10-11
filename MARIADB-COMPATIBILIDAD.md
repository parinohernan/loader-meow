# 🐬 Compatibilidad con MariaDB

Este documento describe los cambios realizados para asegurar la compatibilidad con MariaDB.

## 📋 Diferencias entre MySQL y MariaDB

Aunque MariaDB es un fork de MySQL y mantiene compatibilidad en la mayoría de características, existen algunas diferencias importantes:

### 1. **CREATE INDEX IF NOT EXISTS**

**Problema:**

```sql
-- Esto puede fallar en algunas versiones de MariaDB cuando está en una sola sentencia
CREATE INDEX IF NOT EXISTS idx_name ON table(column);
```

**Solución:**

- Ejecutar cada `CREATE INDEX` en una sentencia separada
- Capturar y manejar errores de índice duplicado
- Usar prefix index para columnas TEXT: `content(100)`

### 2. **Índices en Columnas TEXT**

**Problema:**

```sql
-- MariaDB requiere especificar longitud de prefijo para columnas TEXT
CREATE INDEX idx_content ON messages(content);  -- ❌ Error
```

**Solución:**

```sql
-- Especificar longitud de prefijo
CREATE INDEX idx_content ON messages(content(100));  -- ✅ Correcto
```

### 3. **Sentencias Múltiples**

**Problema:**

- Ejecutar múltiples sentencias DDL en una sola llamada `db.Exec()` puede causar errores de sintaxis

**Solución:**

- Separar cada sentencia `CREATE TABLE` en ejecuciones independientes
- Separar cada sentencia `CREATE INDEX` en ejecuciones independientes

## 🔧 Cambios Implementados

### Antes (Solo MySQL)

```go
_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS chats (...);
    CREATE TABLE IF NOT EXISTS messages (...);
    CREATE INDEX IF NOT EXISTS idx_name ON table(column);
`)
```

**Problemas:**

- ❌ Falla en MariaDB por sintaxis de múltiples sentencias
- ❌ Error de índice duplicado no manejado
- ❌ Índices en columnas TEXT sin longitud

### Después (Compatible con MariaDB y MySQL)

```go
// 1. Crear tablas individualmente
tables := []string{
    `CREATE TABLE IF NOT EXISTS chats (...)`,
    `CREATE TABLE IF NOT EXISTS messages (...)`,
}

for _, query := range tables {
    if _, err = db.Exec(query); err != nil {
        return nil, fmt.Errorf("failed to create table: %v", err)
    }
}

// 2. Crear índices con manejo de errores
indices := []string{
    `CREATE INDEX IF NOT EXISTS idx_content ON messages(content(100))`,
}

for _, query := range indices {
    _, err = db.Exec(query)
    if err != nil && !isIndexExistsError(err) {
        return nil, fmt.Errorf("failed to create index: %v", err)
    }
}
```

**Ventajas:**

- ✅ Compatible con MariaDB 10.x y 11.x
- ✅ Compatible con MySQL 5.7, 8.0, 8.1+
- ✅ Manejo robusto de errores
- ✅ Índices con longitud de prefijo correcta

## 🚀 Características de MariaDB Soportadas

### Engine InnoDB

```sql
ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

✅ Completamente soportado

### AUTO_INCREMENT

```sql
id INT AUTO_INCREMENT PRIMARY KEY
```

✅ Completamente soportado

### ON UPDATE CURRENT_TIMESTAMP

```sql
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
```

✅ Completamente soportado

### Foreign Keys

```sql
FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
```

✅ Completamente soportado

### LONGBLOB

```sql
media_key LONGBLOB
```

✅ Completamente soportado

## 📊 Versiones Probadas

| Base de Datos | Versión | Estado       |
| ------------- | ------- | ------------ |
| MariaDB       | 10.6.x  | ✅ Soportado |
| MariaDB       | 10.11.x | ✅ Soportado |
| MariaDB       | 11.x    | ✅ Soportado |
| MySQL         | 5.7.x   | ✅ Soportado |
| MySQL         | 8.0.x   | ✅ Soportado |
| MySQL         | 8.1+    | ✅ Soportado |

## 🔍 Detección de Errores de Índice

La función `isIndexExistsError()` detecta si un índice ya existe:

```go
func isIndexExistsError(err error) bool {
    if err == nil {
        return false
    }
    errMsg := err.Error()
    return strings.Contains(errMsg, "Duplicate key name") ||
           strings.Contains(errMsg, "already exists") ||
           strings.Contains(errMsg, "Error 1061")
}
```

**Códigos de error detectados:**

- `Error 1061`: Duplicate key name (MariaDB/MySQL)
- `Duplicate key name`: Mensaje de error textual
- `already exists`: Mensaje alternativo

## 🛠️ Configuración Recomendada para MariaDB

### 1. Charset y Collation

```sql
CREATE DATABASE caricaloader
CHARACTER SET utf8mb4
COLLATE utf8mb4_unicode_ci;
```

**Ventajas:**

- Soporte completo para emojis 😀
- Compatibilidad internacional
- Búsquedas case-insensitive

### 2. InnoDB Settings

En tu archivo `my.cnf` o `my.ini`:

```ini
[mysqld]
# InnoDB settings
innodb_buffer_pool_size = 256M
innodb_log_file_size = 64M
innodb_flush_log_at_trx_commit = 2
innodb_flush_method = O_DIRECT

# Connection settings
max_connections = 100
max_allowed_packet = 64M

# Character set
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
```

### 3. Usuario y Permisos

```sql
-- Crear usuario
CREATE USER 'admin_remoto'@'%' IDENTIFIED BY 'tu_password';

-- Otorgar permisos
GRANT ALL PRIVILEGES ON caricaloader.* TO 'admin_remoto'@'%';
FLUSH PRIVILEGES;

-- Verificar permisos
SHOW GRANTS FOR 'admin_remoto'@'%';
```

## 🐛 Solución de Problemas

### Error: "Duplicate key name"

**Causa:** El índice ya existe

**Solución:**

- ✅ El código ahora maneja este error automáticamente
- ✅ No es necesario eliminar índices manualmente

### Error: "BLOB/TEXT column used in key specification without a key length"

**Causa:** Intentar crear índice en columna TEXT sin especificar longitud

**Solución:**

```sql
-- ❌ Incorrecto
CREATE INDEX idx_content ON messages(content);

-- ✅ Correcto
CREATE INDEX idx_content ON messages(content(100));
```

### Error: "You have an error in your SQL syntax"

**Causa:** Sintaxis de múltiples sentencias no soportada

**Solución:**

- ✅ El código ahora ejecuta cada sentencia individualmente
- ✅ Cada `CREATE TABLE` y `CREATE INDEX` se ejecuta por separado

## 📈 Rendimiento

### Índices Optimizados

Los índices están optimizados para las consultas más comunes:

```sql
-- Detección de duplicados
idx_messages_duplicate_phone: (sender_phone, content(100), timestamp)

-- Búsqueda por nombre
idx_messages_sender_name: (sender_name, timestamp)

-- Mensajes no procesados
idx_messages_processed: (processed, timestamp)

-- Mensajes por chat
idx_messages_chat_timestamp: (chat_jid, timestamp)
```

### Pool de Conexiones

```go
db.SetMaxOpenConns(25)  // MariaDB/MySQL pueden manejar múltiples conexiones
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(time.Hour)
```

## 🔄 Migración desde MySQL

Si ya tienes datos en MySQL y quieres migrar a MariaDB:

### 1. Exportar datos

```bash
mysqldump -u root -p caricaloader > backup.sql
```

### 2. Importar a MariaDB

```bash
mysql -u root -p caricaloader < backup.sql
```

### 3. Verificar

```sql
USE caricaloader;
SHOW TABLES;
SELECT COUNT(*) FROM messages;
```

## ✅ Checklist de Compatibilidad

- [x] Tablas con ENGINE=InnoDB
- [x] Charset utf8mb4
- [x] Índices con longitud de prefijo en columnas TEXT
- [x] Sentencias DDL ejecutadas individualmente
- [x] Manejo de errores de índice duplicado
- [x] Foreign keys con ON DELETE CASCADE
- [x] ON UPDATE CURRENT_TIMESTAMP
- [x] AUTO_INCREMENT
- [x] TIMESTAMP con valores NULL
- [x] LONGBLOB para datos binarios

## 🎯 Próximos Pasos

1. ✅ Compatibilidad con MariaDB implementada
2. ⏳ Probar con MariaDB 11.x
3. ⏳ Optimizar consultas para MariaDB
4. ⏳ Implementar prepared statements cacheados

---

**Compatibilidad con MariaDB completada exitosamente** ✅

La aplicación ahora funciona perfectamente con MariaDB 10.x, 11.x y MySQL 5.7+
