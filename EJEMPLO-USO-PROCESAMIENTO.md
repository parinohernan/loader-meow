# 💡 Ejemplos de Uso del Sistema de Procesamiento

## 🎯 Casos de Uso Prácticos

### 1. Bot de Respuestas Automáticas

```go
// bot.go
package main

import (
    "strings"
    "time"
)

type AutoReplyBot struct {
    app *App
}

func NewAutoReplyBot(app *App) *AutoReplyBot {
    return &AutoReplyBot{app: app}
}

func (bot *AutoReplyBot) Start() {
    ticker := time.NewTicker(3 * time.Second)

    go func() {
        for range ticker.C {
            bot.processMessages()
        }
    }()
}

func (bot *AutoReplyBot) processMessages() {
    // Obtener mensajes no procesados
    messages, err := bot.app.GetUnprocessedMessages(50)
    if err != nil {
        return
    }

    for _, msg := range messages {
        // Solo procesar mensajes entrantes (no los nuestros)
        if msg.IsFromMe {
            bot.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
            continue
        }

        // Lógica de respuestas
        response := bot.generateResponse(msg.Content)
        if response != "" {
            bot.app.SendMessage(msg.ChatJID, response)
        }

        // Marcar como procesado
        bot.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
    }
}

func (bot *AutoReplyBot) generateResponse(content string) string {
    content = strings.ToLower(content)

    if strings.Contains(content, "hola") {
        return "¡Hola! ¿En qué puedo ayudarte?"
    }

    if strings.Contains(content, "precio") || strings.Contains(content, "costo") {
        return "Para información de precios, visita nuestro catálogo."
    }

    if strings.Contains(content, "horario") {
        return "Nuestro horario de atención es de Lunes a Viernes, 9am-6pm."
    }

    return "" // No responder si no coincide
}

// En tu main.go, después de conectar WhatsApp:
// bot := NewAutoReplyBot(app)
// bot.Start()
```

### 2. Sistema de Notificaciones

```go
// notifier.go
package main

import (
    "fmt"
    "strings"
    "time"
)

type Notifier struct {
    app *App
    keywords []string
    notifyChannel string // JID donde enviar notificaciones
}

func NewNotifier(app *App, keywords []string, notifyChannel string) *Notifier {
    return &Notifier{
        app: app,
        keywords: keywords,
        notifyChannel: notifyChannel,
    }
}

func (n *Notifier) Start() {
    ticker := time.NewTicker(5 * time.Second)

    go func() {
        for range ticker.C {
            n.checkKeywords()
        }
    }()
}

func (n *Notifier) checkKeywords() {
    messages, err := n.app.GetUnprocessedMessages(100)
    if err != nil {
        return
    }

    for _, msg := range messages {
        if msg.IsFromMe {
            n.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
            continue
        }

        // Verificar si contiene palabras clave
        for _, keyword := range n.keywords {
            if strings.Contains(strings.ToLower(msg.Content), keyword) {
                n.sendNotification(msg)
                break
            }
        }

        n.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
    }
}

func (n *Notifier) sendNotification(msg ChatMessage) {
    notification := fmt.Sprintf(
        "🔔 Palabra clave detectada!\n\nDe: +%s\nMensaje: %s",
        msg.Sender,
        msg.Content,
    )

    n.app.SendMessage(n.notifyChannel, notification)
}

// Uso:
// keywords := []string{"urgente", "problema", "ayuda"}
// notifier := NewNotifier(app, keywords, "573001234567@s.whatsapp.net")
// notifier.Start()
```

### 3. Logger de Mensajes

```go
// logger.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "time"
)

type MessageLogger struct {
    app *App
    logFile *os.File
}

func NewMessageLogger(app *App, filename string) (*MessageLogger, error) {
    file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }

    return &MessageLogger{
        app: app,
        logFile: file,
    }, nil
}

func (ml *MessageLogger) Start() {
    ticker := time.NewTicker(10 * time.Second)

    go func() {
        for range ticker.C {
            ml.logMessages()
        }
    }()
}

func (ml *MessageLogger) logMessages() {
    messages, err := ml.app.GetUnprocessedMessages(500)
    if err != nil {
        return
    }

    for _, msg := range messages {
        logEntry := map[string]interface{}{
            "timestamp": msg.Timestamp,
            "chat": msg.ChatJID,
            "sender": msg.Sender,
            "content": msg.Content,
            "is_from_me": msg.IsFromMe,
        }

        jsonData, _ := json.Marshal(logEntry)
        ml.logFile.WriteString(string(jsonData) + "\n")

        ml.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
    }
}

func (ml *MessageLogger) Close() {
    ml.logFile.Close()
}

// Uso:
// logger, _ := NewMessageLogger(app, "messages.log")
// logger.Start()
// defer logger.Close()
```

### 4. Análisis de Actividad

```go
// analytics.go
package main

import (
    "fmt"
    "time"
)

type Analytics struct {
    app *App
    stats map[string]int // sender -> count
}

func NewAnalytics(app *App) *Analytics {
    return &Analytics{
        app: app,
        stats: make(map[string]int),
    }
}

func (a *Analytics) Start() {
    // Procesar cada minuto
    ticker := time.NewTicker(1 * time.Minute)

    // Reportar cada hora
    reportTicker := time.NewTicker(1 * time.Hour)

    go func() {
        for {
            select {
            case <-ticker.C:
                a.collectStats()
            case <-reportTicker.C:
                a.generateReport()
            }
        }
    }()
}

func (a *Analytics) collectStats() {
    messages, err := a.app.GetUnprocessedMessages(1000)
    if err != nil {
        return
    }

    for _, msg := range messages {
        if !msg.IsFromMe {
            a.stats[msg.Sender]++
        }
        a.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
    }
}

func (a *Analytics) generateReport() {
    fmt.Println("\n=== Reporte de Actividad ===")
    fmt.Printf("Total de remitentes únicos: %d\n", len(a.stats))

    // Top 10 más activos
    fmt.Println("\nTop 10 más activos:")
    // Aquí irías ordenando el map...

    // Resetear stats
    a.stats = make(map[string]int)
}

// Uso:
// analytics := NewAnalytics(app)
// analytics.Start()
```

### 5. Sistema de Comandos

```go
// commands.go
package main

import (
    "strings"
    "time"
)

type CommandHandler struct {
    app *App
}

func NewCommandHandler(app *App) *CommandHandler {
    return &CommandHandler{app: app}
}

func (ch *CommandHandler) Start() {
    ticker := time.NewTicker(2 * time.Second)

    go func() {
        for range ticker.C {
            ch.processCommands()
        }
    }()
}

func (ch *CommandHandler) processCommands() {
    messages, err := ch.app.GetUnprocessedMessages(50)
    if err != nil {
        return
    }

    for _, msg := range messages {
        if msg.IsFromMe {
            ch.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
            continue
        }

        // Verificar si es un comando (empieza con /)
        if strings.HasPrefix(msg.Content, "/") {
            ch.handleCommand(msg)
        }

        ch.app.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
    }
}

func (ch *CommandHandler) handleCommand(msg ChatMessage) {
    parts := strings.Split(msg.Content, " ")
    command := parts[0]

    switch command {
    case "/help":
        response := "Comandos disponibles:\n/help - Muestra esta ayuda\n/stats - Estadísticas\n/ping - Verifica conexión"
        ch.app.SendMessage(msg.ChatJID, response)

    case "/stats":
        stats, _ := ch.app.GetMessageStats()
        response := fmt.Sprintf(
            "Estadísticas:\nTotal: %d\nProcesados: %d\nPendientes: %d",
            stats["total"],
            stats["processed"],
            stats["unprocessed"],
        )
        ch.app.SendMessage(msg.ChatJID, response)

    case "/ping":
        ch.app.SendMessage(msg.ChatJID, "Pong! 🏓")

    default:
        ch.app.SendMessage(msg.ChatJID, "Comando no reconocido. Usa /help")
    }
}

// Uso:
// commands := NewCommandHandler(app)
// commands.Start()
```

## 🔄 Ejemplo Completo con Múltiples Workers

```go
// main.go
package main

import (
    "time"
)

func main() {
    // ... [código de inicialización de Wails] ...

    // Después de conectar WhatsApp
    go func() {
        // Esperar a que conecte
        time.Sleep(5 * time.Second)

        if app.IsConnected() {
            // Iniciar bot de respuestas automáticas
            bot := NewAutoReplyBot(app)
            bot.Start()

            // Iniciar sistema de notificaciones
            keywords := []string{"urgente", "problema"}
            notifier := NewNotifier(app, keywords, "TU_NUMERO@s.whatsapp.net")
            notifier.Start()

            // Iniciar logger
            logger, _ := NewMessageLogger(app, "messages.log")
            logger.Start()

            // Iniciar comandos
            commands := NewCommandHandler(app)
            commands.Start()

            fmt.Println("✅ Todos los workers iniciados")
        }
    }()

    // ... [resto del código de Wails] ...
}
```

## 📊 Monitoreo del Sistema

```go
// monitor.go
package main

import (
    "fmt"
    "time"
)

func StartMonitor(app *App) {
    ticker := time.NewTicker(30 * time.Second)

    go func() {
        for range ticker.C {
            stats, err := app.GetMessageStats()
            if err != nil {
                continue
            }

            fmt.Printf("\n[STATS] Total: %d | Procesados: %d | Pendientes: %d\n",
                stats["total"],
                stats["processed"],
                stats["unprocessed"],
            )

            // Alerta si hay muchos pendientes
            if stats["unprocessed"] > 100 {
                fmt.Println("⚠️ ALERTA: Más de 100 mensajes pendientes!")
            }
        }
    }()
}

// En main.go:
// StartMonitor(app)
```

## 🎯 Tips y Mejores Prácticas

1. **Intervalo de Procesamiento**: Ajusta según tu volumen

   - Bajo volumen: 5-10 segundos
   - Alto volumen: 1-2 segundos
   - Muy alto: Implementa colas

2. **Límite de Mensajes**: No consultes más de 500-1000 por vez

3. **Error Handling**: Siempre marca como procesado, incluso si hay error

4. **Logging**: Registra estadísticas periódicamente

5. **Testing**: Prueba con cuentas de test primero

---

¡Ahora tienes ejemplos completos para empezar a procesar mensajes! 🚀
