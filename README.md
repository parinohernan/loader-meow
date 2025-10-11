# 🐱 Loader Meow - WhatsApp Desktop Client

Una aplicación de escritorio moderna para WhatsApp construida con **Go** y **Wails**, integrada con **whatsmeow** para conectarse a WhatsApp Web y gestionar tus mensajes.

![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)
![Wails](https://img.shields.io/badge/Wails-v2.10.2-DF0000?style=flat)
![License](https://img.shields.io/badge/license-MIT-green?style=flat)

## ✨ Características

- 🖥️ **Aplicación de Escritorio Nativa**: Construida con Wails v2 para Windows, macOS y Linux
- 📱 **Conexión con WhatsApp Web**: Usa la librería whatsmeow para conectarse a tu cuenta
- 💬 **Vista de Chats y Mensajes**: Interfaz similar a WhatsApp Web
- 🔄 **Sincronización en Tiempo Real**: Recibe mensajes instantáneamente
- 💾 **Base de Datos MySQL**: Almacena mensajes en MySQL para mejor rendimiento y escalabilidad
- 📨 **Enviar Mensajes**: Responde desde la aplicación de escritorio
- 👥 **Soporte para Grupos**: Muestra números de teléfono y nombres de participantes
- 🎨 **Interfaz Moderna**: UI oscura inspirada en WhatsApp Web

## 🚀 Requisitos Previos

### Herramientas Necesarias

1. **Go 1.22 o superior**

   ```bash
   go version
   ```

2. **Wails CLI**

   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```

3. **MySQL** (REQUERIDO para la base de datos):

   - **Windows**: [MySQL Community Server](https://dev.mysql.com/downloads/mysql/) o [XAMPP](https://www.apachefriends.org/)
   - **macOS**: `brew install mysql` o [MySQL Community Server](https://dev.mysql.com/downloads/mysql/)
   - **Linux**: `sudo apt install mysql-server` o `sudo yum install mysql-server`

   ⚠️ **IMPORTANTE**: La aplicación requiere MySQL para funcionar correctamente

4. **Dependencias del Sistema** (según tu SO):

   **Windows:**

   - WebView2 (generalmente ya está instalado en Windows 10/11)
   - Puedes descargar el instalador desde: https://developer.microsoft.com/microsoft-edge/webview2/

   **macOS:**

   - Xcode Command Line Tools

   ```bash
   xcode-select --install
   ```

   **Linux (Ubuntu/Debian):**

   ```bash
   sudo apt update
   sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.0-dev
   ```

## 📦 Instalación

1. **Clonar el repositorio**

   ```bash
   git clone https://github.com/TU-USUARIO/loader-meow.git
   cd loader-meow
   ```

2. **Configurar MySQL**

   **Windows:**

   ```bash
   ./setup-mysql.bat
   ```

   Este script:

   - Verifica que MySQL esté instalado
   - Crea la base de datos `whatsapp_loader`
   - Configura las credenciales de conexión
   - Genera el archivo de configuración

   **macOS/Linux:**

   ```bash
   mysql -u root -p
   CREATE DATABASE whatsapp_loader CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   ```

3. **Instalar dependencias**

   ```bash
   go mod download
   go mod tidy
   ```

## 🎮 Uso

### Modo Desarrollo

**Windows:**

1. Copia el archivo de configuración:

   ```bash
   copy run-dev.bat.example run-dev.bat
   ```

2. Edita `run-dev.bat` con tus credenciales de MySQL

3. Ejecuta:
   ```bash
   ./run-dev.bat
   ```

**macOS/Linux:**

1. Copia el archivo de configuración:

   ```bash
   cp mysql-config.env.example mysql-config.env
   ```

2. Edita `mysql-config.env` con tus credenciales

3. Ejecuta:
   ```bash
   export $(cat mysql-config.env | xargs) && wails dev
   ```

⚠️ **IMPORTANTE**:

- Asegúrate de que MySQL esté ejecutándose antes de iniciar la aplicación
- NUNCA subas archivos con credenciales reales al repositorio

### Primera Conexión

1. **Inicia la aplicación** en modo desarrollo o producción
2. **Haz clic en "Conectar WhatsApp"**
3. **Escanea el código QR** con tu WhatsApp móvil:
   - Abre WhatsApp en tu teléfono
   - Ve a **Configuración > Dispositivos vinculados**
   - Toca **Vincular un dispositivo**
   - Escanea el código QR que aparece en la aplicación
4. **¡Listo!** Tu WhatsApp se conectará automáticamente

### Modo Producción

**Windows:**

```bash
wails build
```

**macOS/Linux:**

```bash
wails build
```

El ejecutable se generará en la carpeta `build/bin/`:

- **Windows:** `build/bin/loader-meow.exe`
- **macOS:** `build/bin/loader-meow.app`
- **Linux:** `build/bin/loader-meow`

## 📖 Características de la Aplicación

### 1. Conexión con QR

- Escanea el código QR para vincular tu cuenta de WhatsApp
- La sesión se guarda localmente para futuras conexiones
- No necesitas escanear el QR cada vez

### 2. Lista de Chats

- Ver todos tus chats activos
- Ordenados por fecha del último mensaje
- Actualización automática cuando llegan nuevos mensajes

### 3. Vista de Mensajes

- Lee todos los mensajes de un chat
- Ver quién envió cada mensaje (en grupos muestra el número de teléfono)
- Indica si el mensaje tiene archivos adjuntos
- Ordenados cronológicamente

### 4. Enviar Mensajes

- Escribe y envía mensajes de texto
- Presiona Enter para enviar rápidamente
- Los mensajes se sincronizan con tu WhatsApp móvil

## 🏗️ Arquitectura del Proyecto

```
loader-meow/
├── main.go                 # Punto de entrada de Wails
├── app.go                  # Lógica de la aplicación Wails
├── whatsapp_service.go     # Servicio de WhatsApp con whatsmeow
├── go.mod                  # Dependencias
├── wails.json              # Configuración de Wails
├── frontend/
│   └── dist/
│       └── index.html      # Interfaz de usuario
├── store/                  # Carpeta de datos (generada, en .gitignore)
│   ├── whatsapp.db        # Sesión de WhatsApp
│   └── messages.db        # Mensajes almacenados
├── build/                  # Ejecutables (generados, en .gitignore)
├── setup-cgo.bat          # Script de configuración (Windows)
├── run-with-cgo.bat       # Script de ejecución dev (Windows)
└── build-with-cgo.bat     # Script de compilación (Windows)
```

## 🔧 Cómo Funciona

### Flujo de Conexión

1. **Inicio**: La app verifica si ya hay una sesión guardada
2. **Sin Sesión**: Muestra el botón de conexión
3. **Conexión**: Genera un código QR
4. **Escaneo**: El usuario escanea el QR con su móvil
5. **Autenticación**: whatsmeow establece la conexión
6. **Sincronización**: La app recibe todos los chats y mensajes

### Almacenamiento

- **MySQL Database**: Almacena todos los mensajes, chats y asociaciones de teléfonos
- **whatsapp.db**: Guarda la sesión y configuración de WhatsApp (SQLite para whatsmeow)
- Los datos de WhatsApp se guardan en la carpeta `store/` (no incluida en Git)
- La base de datos MySQL se configura externamente

### Eventos en Tiempo Real

La aplicación usa el sistema de eventos de Wails para:

- Recibir códigos QR
- Notificar conexión exitosa
- Actualizar mensajes en tiempo real
- Sincronizar chats automáticamente

## 🛠️ Tecnologías Utilizadas

### Backend (Go)

- **Wails v2**: Framework para aplicaciones de escritorio
- **whatsmeow**: Librería para conectarse a WhatsApp Web
- **MySQL**: Base de datos para mensajes y asociaciones
- **go-sql-driver/mysql**: Driver de MySQL para Go

### Frontend

- **HTML5/CSS3**: Interfaz moderna
- **Vanilla JavaScript**: Sin frameworks adicionales
- **Wails Runtime**: Bridge entre JS y Go

## 🐛 Solución de Problemas

### Error: "WebView2 no encontrado" (Windows)

- Descarga e instala WebView2 Runtime desde Microsoft

### Error: "gcc: command not found" o "CGO_ENABLED=0"

**Causa**: GCC no está instalado o CGO no está habilitado

**Solución Windows:**

1. Instala TDM-GCC: https://jmeubank.github.io/tdm-gcc/download/
2. Marca "Add to PATH" durante instalación
3. Reinicia la terminal
4. Ejecuta: `./setup-cgo.bat`
5. Usa: `./run-with-cgo.bat` (NO `wails dev` directamente)

**Solución macOS/Linux:**

- `sudo apt install build-essential` o Xcode Command Line Tools

📖 Ver archivo **SOLUCION-CGO.md** para una guía detallada paso a paso

### Error de conexión a WhatsApp

- Verifica tu conexión a internet
- Asegúrate de que WhatsApp Web funcione en tu navegador
- Elimina la carpeta `store/` y vuelve a escanear el QR

### Los mensajes no se actualizan

- Haz clic en el botón de refrescar (🔄) en la lista de chats
- Verifica que la conexión esté activa

### La base de datos está bloqueada

- Cierra todas las instancias de la aplicación
- Si persiste, elimina `store/messages.db` (perderás el historial local)

## 📚 Recursos Adicionales

- [Documentación de Wails](https://wails.io/docs/introduction)
- [whatsmeow GitHub](https://github.com/tulir/whatsmeow)
- [WhatsApp Web Protocol](https://github.com/sigalor/whatsapp-web-reveng)
- [Go Documentation](https://go.dev/doc/)

## ⚠️ Importante

- Esta aplicación es un cliente no oficial de WhatsApp
- Usa la librería whatsmeow que sigue el protocolo oficial de WhatsApp Web
- Los mensajes se almacenan localmente en tu computadora
- Tu sesión de WhatsApp permanece vinculada hasta que la cierres manualmente

## 🔐 Privacidad y Seguridad

- **Datos Locales**: Todo se almacena en tu computadora
- **Sin Servidores Externos**: Conexión directa a WhatsApp
- **Código Abierto**: Puedes revisar todo el código fuente
- **Sesión Encriptada**: La sesión se almacena de forma segura

## 🌟 Características Futuras

- [ ] Soporte para enviar imágenes y archivos
- [ ] Búsqueda de mensajes
- [ ] Notificaciones de escritorio
- [ ] Tema claro/oscuro
- [ ] Respuestas rápidas
- [ ] Estados de WhatsApp
- [ ] Grupos: crear y administrar
- [ ] Descarga automática de medios

## 🤝 Contribuciones

¡Las contribuciones son bienvenidas! Siéntete libre de:

- Reportar bugs
- Sugerir nuevas características
- Mejorar la documentación
- Enviar pull requests

## 📝 Licencia

Este proyecto es de código abierto bajo la Licencia MIT.

## 🙏 Agradecimientos

- [Wails](https://wails.io/) - Framework increíble para aplicaciones de escritorio con Go
- [whatsmeow](https://github.com/tulir/whatsmeow) - Librería de WhatsApp Web
- La comunidad de Go y desarrolladores de código abierto

---

**Hecho con ❤️ usando Go, Wails y whatsmeow**

🐱 Loader Meow - Tu cliente de WhatsApp de escritorio
