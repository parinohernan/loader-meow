# 📱 Loader Meow - Cliente WhatsApp Desktop

## 🎯 ¿Qué es?

Loader Meow es una aplicación de escritorio que te permite usar WhatsApp en tu computadora de forma nativa. Funciona como WhatsApp Web pero en una aplicación independiente construida con Go.

## ✨ Características Principales

### ✅ Implementado

- **Conexión con QR**: Escanea un código QR para vincular tu WhatsApp
- **Lista de Chats**: Ve todos tus chats ordenados por fecha
- **Ver Mensajes**: Lee mensajes de cualquier chat
- **Enviar Mensajes**: Envía mensajes de texto
- **Tiempo Real**: Recibe mensajes instantáneamente
- **Almacenamiento Local**: Guarda mensajes en SQLite
- **Interfaz Moderna**: UI oscura inspirada en WhatsApp Web

### 🚀 Próximas Mejoras Sugeridas

- Envío de imágenes y archivos
- Notificaciones de escritorio
- Búsqueda de mensajes
- Crear y administrar grupos
- Ver estados de WhatsApp
- Respuestas rápidas
- Tema claro/oscuro

## 🏗️ Tecnologías

- **Go**: Lenguaje de programación
- **Wails v2**: Framework para apps de escritorio
- **whatsmeow**: Librería de WhatsApp Web
- **SQLite**: Base de datos local
- **HTML/CSS/JS**: Interfaz de usuario

## 📂 Estructura del Proyecto

```
loader-meow/
├── main.go                 # Punto de entrada Wails
├── app.go                  # Lógica de la app
├── whatsapp_service.go     # Servicio de WhatsApp
├── go.mod                  # Dependencias
├── wails.json              # Config de Wails
├── frontend/dist/
│   └── index.html         # Interfaz de usuario
└── store/                 # Datos (generado al ejecutar)
    ├── whatsapp.db        # Sesión de WhatsApp
    └── messages.db        # Mensajes
```

## 🚀 Inicio Rápido

### 1. Instalar Dependencias

```bash
# Instalar Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Instalar dependencias del proyecto
go mod download
```

### 2. Ejecutar en Desarrollo

```bash
wails dev
```

### 3. Compilar para Producción

```bash
wails build
```

El ejecutable estará en: `build/bin/loader-meow.exe`

### 4. Conectar WhatsApp

1. Abre la aplicación
2. Haz clic en "Conectar WhatsApp"
3. Escanea el QR con tu WhatsApp móvil
4. ¡Listo!

## 🔍 Cómo Funciona

### Flujo de Datos

```
1. Usuario escanea QR
   ↓
2. whatsmeow autentica con WhatsApp
   ↓
3. Conexión establecida
   ↓
4. Mensajes se sincronizan
   ↓
5. UI muestra chats y mensajes
   ↓
6. Usuario envía mensaje
   ↓
7. whatsmeow envía a WhatsApp
   ↓
8. Mensaje aparece en todos los dispositivos
```

### Componentes Principales

**whatsapp_service.go**

- Maneja la conexión con WhatsApp
- Procesa eventos (mensajes, conexión, etc.)
- Almacena mensajes en SQLite
- Gestiona la sesión

**app.go**

- Expone métodos al frontend (JavaScript)
- Maneja eventos de Wails
- Coordina entre UI y servicio de WhatsApp

**frontend/dist/index.html**

- Interfaz de usuario completa
- Lista de chats
- Vista de mensajes
- Input para enviar mensajes

## 📊 Base de Datos

### whatsapp.db

Almacena la sesión de WhatsApp (credenciales, keys, etc.)

### messages.db

```sql
-- Tabla de chats
CREATE TABLE chats (
    jid TEXT PRIMARY KEY,
    name TEXT,
    last_message_time TIMESTAMP
);

-- Tabla de mensajes
CREATE TABLE messages (
    id TEXT,
    chat_jid TEXT,
    sender TEXT,
    content TEXT,
    timestamp TIMESTAMP,
    is_from_me BOOLEAN,
    media_type TEXT,
    filename TEXT,
    PRIMARY KEY (id, chat_jid)
);
```

## 🎨 Interfaz de Usuario

### Vista de Conexión

- Botón para conectar
- Código QR para escanear
- Estado de conexión

### Vista Principal

- **Sidebar Izquierdo**: Lista de chats
  - Nombre del chat
  - Hora del último mensaje
  - Botón de actualizar
- **Panel Derecho**: Mensajes
  - Burbujas de mensajes (entrantes/salientes)
  - Nombre del remitente
  - Hora del mensaje
  - Input para enviar

## 🔐 Seguridad y Privacidad

- ✅ **Todo es local**: Los datos se guardan en tu computadora
- ✅ **Sin servidores externos**: Conexión directa a WhatsApp
- ✅ **Código abierto**: Puedes revisar el código fuente
- ✅ **Sesión segura**: whatsmeow usa el protocolo oficial

## 🛠️ Desarrollo

### Agregar una Nueva Funcionalidad

1. **Backend (Go)**

   - Agregar método en `whatsapp_service.go` si es necesario
   - Exponer método en `app.go`

2. **Frontend (JavaScript)**
   - Llamar método con `window.go.main.App.MetodoNuevo()`
   - Actualizar UI según resultado

### Ejemplo: Agregar Búsqueda

```go
// En whatsapp_service.go
func (store *MessageStore) SearchMessages(query string) ([]ChatMessage, error) {
    // SQL para buscar en messages
}

// En app.go
func (a *App) SearchMessages(query string) ([]ChatMessage, error) {
    return a.waService.messageStore.SearchMessages(query)
}
```

```javascript
// En index.html
async function search() {
  const query = document.getElementById("searchInput").value;
  const results = await window.go.main.App.SearchMessages(query);
  displayResults(results);
}
```

## 📚 Recursos

- **Wails**: https://wails.io/
- **whatsmeow**: https://github.com/tulir/whatsmeow
- **WhatsApp Protocol**: https://github.com/sigalor/whatsapp-web-reveng

## ⚠️ Notas Importantes

1. **Cliente No Oficial**: Esta app no está afiliada con WhatsApp
2. **Protocolo Oficial**: Usa el mismo protocolo que WhatsApp Web
3. **Límites**: WhatsApp tiene límites de dispositivos vinculados
4. **Cierre de Sesión**: Puedes cerrar sesión desde tu móvil

## 🤝 Contribuir

Ideas para contribuir:

- Implementar envío de archivos
- Agregar notificaciones
- Mejorar la UI
- Agregar tests
- Optimizar rendimiento

---

**¡Gracias por usar Loader Meow! 🐱**

_Una forma moderna de usar WhatsApp en tu escritorio_

