# 🔄 Migración de SQLite a MySQL

Este documento describe la migración completa de SQLite a MySQL realizada en Loader Meow.

## 📋 Resumen de Cambios

### ✅ Completado

1. **Dependencias actualizadas**

   - ❌ `github.com/mattn/go-sqlite3`
   - ✅ `github.com/go-sql-driver/mysql`

2. **Configuración de conexión**

   - ✅ Nuevo archivo `config.go` con configuración MySQL
   - ✅ Variables de entorno para conexión
   - ✅ Pool de conexiones optimizado para MySQL

3. **Esquemas de base de datos**

   - ✅ Conversión de tipos SQLite a MySQL
   - ✅ Engine InnoDB con charset utf8mb4
   - ✅ Índices optimizados para MySQL

4. **Consultas SQL**

   - ✅ `INSERT OR REPLACE` → `INSERT ... ON DUPLICATE KEY UPDATE`
   - ✅ `datetime()` → `DATE_SUB()/DATE_ADD()`
   - ✅ `CURRENT_TIMESTAMP` → `NOW()`

5. **Scripts y documentación**
   - ✅ `setup-mysql.bat` - Configuración automática
   - ✅ `load-env.bat` - Carga de variables de entorno
   - ✅ README.md actualizado
   - ✅ Documentación de migración

## 🗂️ Archivos Modificados

### Nuevos Archivos

- `config.go` - Configuración de base de datos
- `mysql-config.env` - Variables de entorno
- `setup-mysql.bat` - Script de configuración
- `load-env.bat` - Script de ejecución
- `MIGRACION-MYSQL.md` - Esta documentación

### Archivos Modificados

- `go.mod` - Dependencias actualizadas
- `whatsapp_service.go` - Lógica de conexión y esquemas
- `README.md` - Documentación actualizada

## 🔧 Cambios Técnicos Detallados

### 1. Configuración de Conexión

**Antes (SQLite):**

```go
db, err := sql.Open("sqlite3", "file:store/messages.db?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
```

**Después (MySQL):**

```go
config := GetDatabaseConfig()
db, err := sql.Open("mysql", config.GetConnectionString())
```

### 2. Pool de Conexiones

**Antes (SQLite):**

```go
db.SetMaxOpenConns(1)  // SQLite funciona mejor con una conexión
db.SetMaxIdleConns(1)
```

**Después (MySQL):**

```go
db.SetMaxOpenConns(25)  // MySQL puede manejar múltiples conexiones
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(time.Hour)
```

### 3. Esquemas de Tablas

**Antes (SQLite):**

```sql
CREATE TABLE IF NOT EXISTS messages (
    id TEXT,
    content TEXT,
    timestamp TIMESTAMP,
    processed BOOLEAN DEFAULT 0
);
```

**Después (MySQL):**

```sql
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(255),
    content TEXT,
    timestamp TIMESTAMP,
    processed BOOLEAN DEFAULT FALSE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 4. Consultas SQL

**Antes (SQLite):**

```sql
INSERT OR REPLACE INTO chats (jid, name) VALUES (?, ?)
SELECT COUNT(*) FROM messages WHERE timestamp >= datetime(?, '-24 hours')
```

**Después (MySQL):**

```sql
INSERT INTO chats (jid, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = VALUES(name)
SELECT COUNT(*) FROM messages WHERE timestamp >= DATE_SUB(?, INTERVAL 24 HOUR)
```

## 🚀 Instrucciones de Instalación

### 1. Instalar MySQL

**Windows:**

- Descargar MySQL Community Server
- O instalar XAMPP (incluye MySQL)

**macOS:**

```bash
brew install mysql
brew services start mysql
```

**Linux:**

```bash
sudo apt install mysql-server
sudo systemctl start mysql
```

### 2. Configurar la Base de Datos

**Windows:**

```bash
./setup-mysql.bat
```

**Manual:**

```sql
CREATE DATABASE whatsapp_loader CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. Configurar Variables de Entorno

Crear archivo `mysql-config.env`:

```
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=tu_password
DB_NAME=whatsapp_loader
DB_CHARSET=utf8mb4
```

### 4. Ejecutar la Aplicación

**Windows:**

```bash
./load-env.bat
```

**macOS/Linux:**

```bash
export $(cat mysql-config.env | xargs) && wails dev
```

## 🔍 Verificación de la Migración

### 1. Verificar Conexión

```bash
mysql -u root -p whatsapp_loader
```

### 2. Verificar Tablas

```sql
SHOW TABLES;
DESCRIBE messages;
DESCRIBE chats;
DESCRIBE phone_associations;
```

### 3. Verificar Datos

```sql
SELECT COUNT(*) FROM messages;
SELECT COUNT(*) FROM chats;
SELECT COUNT(*) FROM phone_associations;
```

## ⚠️ Consideraciones Importantes

### Ventajas de MySQL sobre SQLite

1. **Rendimiento**: Mejor para múltiples conexiones concurrentes
2. **Escalabilidad**: Soporte para bases de datos más grandes
3. **Características**: Triggers, stored procedures, etc.
4. **Backup**: Herramientas nativas de backup y restauración
5. **Monitoreo**: Mejor visibilidad del rendimiento

### Desventajas

1. **Complejidad**: Requiere instalación y configuración adicional
2. **Recursos**: Mayor uso de memoria y CPU
3. **Dependencias**: Requiere que MySQL esté ejecutándose

### Migración de Datos Existentes

Si tienes datos en SQLite que necesitas migrar:

1. **Exportar de SQLite:**

```bash
sqlite3 store/messages.db ".dump" > backup.sql
```

2. **Convertir sintaxis:**

   - Reemplazar `INSERT OR REPLACE` por `INSERT ... ON DUPLICATE KEY UPDATE`
   - Reemplazar `datetime()` por funciones MySQL
   - Ajustar tipos de datos

3. **Importar a MySQL:**

```bash
mysql -u root -p whatsapp_loader < converted_backup.sql
```

## 🐛 Solución de Problemas

### Error: "Access denied for user"

```bash
mysql -u root -p
GRANT ALL PRIVILEGES ON whatsapp_loader.* TO 'root'@'localhost';
FLUSH PRIVILEGES;
```

### Error: "Can't connect to MySQL server"

- Verificar que MySQL esté ejecutándose
- Verificar host y puerto en la configuración
- Verificar firewall/antivirus

### Error: "Unknown database"

```sql
CREATE DATABASE whatsapp_loader CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### Error: "Table doesn't exist"

- La aplicación crea las tablas automáticamente al iniciar
- Verificar que el usuario tenga permisos CREATE

## 📊 Comparación de Rendimiento

| Aspecto                 | SQLite   | MySQL      |
| ----------------------- | -------- | ---------- |
| Conexiones concurrentes | 1 óptimo | 25+        |
| Tamaño de base de datos | Limitado | Escalable  |
| Velocidad de lectura    | Rápida   | Rápida     |
| Velocidad de escritura  | Rápida   | Muy rápida |
| Configuración           | Simple   | Compleja   |
| Recursos                | Mínimos  | Moderados  |

## 🎯 Próximos Pasos

1. **Monitoreo**: Implementar logging de consultas lentas
2. **Optimización**: Ajustar índices según uso real
3. **Backup**: Implementar backup automático
4. **Pool**: Ajustar configuración de pool según carga
5. **Replicación**: Considerar replicación para alta disponibilidad

---

**Migración completada exitosamente** ✅

La aplicación ahora utiliza MySQL como base de datos principal, proporcionando mejor rendimiento y escalabilidad para el manejo de mensajes de WhatsApp.
