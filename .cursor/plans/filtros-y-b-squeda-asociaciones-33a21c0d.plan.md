<!-- 33a21c0d-a189-47be-96e9-1d59c806e8f5 535367f1-611c-4ba3-bd4d-ecd477ae06ab -->
# Plan: Sistema de Procesamiento de Mensajes con IA para Cargas de Transporte

## 1. Backend: Estructura de datos y base de datos

### 1.1 Crear tabla de resultados de procesamiento
**Archivo**: `whatsapp_service.go` - función `NewMessageStore()`

Agregar nueva tabla en el schema:
```sql
CREATE TABLE IF NOT EXISTS ai_processing_results (
    id INT AUTO_INCREMENT PRIMARY KEY,
    message_id VARCHAR(255),
    chat_jid VARCHAR(255),
    content TEXT,
    sender_phone VARCHAR(100),
    real_phone VARCHAR(50),
    ai_response TEXT,
    status VARCHAR(50),
    error_message TEXT,
    supabase_ids TEXT,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (message_id, chat_jid) REFERENCES messages(id, chat_jid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

Status posibles: "pending", "processing", "success", "error", "uploaded"

### 1.2 Agregar función para obtener mensajes procesables
**Archivo**: `whatsapp_service.go`

```go
func (store *MessageStore) GetProcessableMessages(limit int) ([]ChatMessage, error) {
    query := `
        SELECT m.id, m.chat_jid, m.sender_phone, m.sender_name, m.content, 
               m.timestamp, m.is_from_me, m.media_type, m.filename, m.processed,
               pa.real_phone
        FROM messages m
        INNER JOIN phone_associations pa ON m.sender_phone = pa.sender_phone
        WHERE m.processed = 0 
          AND m.content IS NOT NULL 
          AND m.content != ''
          AND pa.real_phone IS NOT NULL
          AND pa.real_phone != ''
        ORDER BY m.timestamp ASC
        LIMIT ?
    `
    // Implementar scan y retornar con campo ALT agregado
}
```

## 2. Backend: Integración con Gemini API

### 2.1 Crear archivo de configuración
**Nuevo archivo**: `ai_config.go`

```go
package main

import "os"

type AIConfig struct {
    APIKey      string
    Model       string
    Temperature float32
    MaxTokens   int
}

func GetAIConfig() *AIConfig {
    return &AIConfig{
        APIKey:      getEnv("GEMINI_API_KEY", ""),
        Model:       getEnv("GEMINI_MODEL", "gemini-2.0-flash-exp"),
        Temperature: 0.1,
        MaxTokens:   8192,
    }
}
```

### 2.2 Crear servicio de IA
**Nuevo archivo**: `ai_service.go`

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type AIService struct {
    config     *AIConfig
    systemPrompt string
}

type GeminiRequest struct {
    Contents []GeminiContent `json:"contents"`
    GenerationConfig GeminiGenerationConfig `json:"generationConfig"`
}

type GeminiContent struct {
    Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
    Text string `json:"text"`
}

type GeminiGenerationConfig struct {
    Temperature float32 `json:"temperature"`
    MaxOutputTokens int `json:"maxOutputTokens"`
}

func NewAIService() *AIService {
    // Cargar prompt desde contecto_funcionalidad_ia.md
    return &AIService{
        config: GetAIConfig(),
        systemPrompt: loadSystemPrompt(),
    }
}

func (s *AIService) ProcessMessage(content string, realPhone string) ([]byte, error) {
    // Agregar "ALT: +5491234..." al final del mensaje
    messageWithAlt := fmt.Sprintf("%s\n\nALT: %s", content, realPhone)
    
    // Construir prompt completo
    fullPrompt := fmt.Sprintf("%s\n\nMENSAJE:\n%s", s.systemPrompt, messageWithAlt)
    
    // Llamar a Gemini API
    url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", 
        s.config.Model, s.config.APIKey)
    
    // Implementar llamada HTTP POST
}
```

## 3. Backend: Integración con Supabase

### 3.1 Crear servicio de Supabase
**Nuevo archivo**: `supabase_service.go`

```go
package main

type SupabaseService struct {
    url    string
    apiKey string
    client *http.Client
}

type CargaData struct {
    Material          string `json:"material"`
    Presentacion      string `json:"presentacion"`
    Peso              string `json:"peso"`
    TipoEquipo        string `json:"tipoEquipo"`
    LocalidadCarga    string `json:"localidadCarga"`
    LocalidadDescarga string `json:"localidadDescarga"`
    FechaCarga        string `json:"fechaCarga"`
    FechaDescarga     string `json:"fechaDescarga"`
    Telefono          string `json:"telefono"`
    // ... resto de campos
}

func NewSupabaseService() *SupabaseService {
    return &SupabaseService{
        url:    "https://ikiusmdtltakhmmlljsp.supabase.co",
        apiKey: getEnv("SUPABASE_KEY", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."),
        client: &http.Client{Timeout: 30 * time.Second},
    }
}

func (s *SupabaseService) CrearCargasDesdeJSON(jsonData []byte) ([]string, error) {
    // 1. Parsear JSON de IA
    var cargas []CargaData
    json.Unmarshal(jsonData, &cargas)
    
    // 2. Para cada carga:
    //    - Obtener/crear ubicaciones (con geocoding)
    //    - Mapear materiales/presentaciones/equipos a IDs
    //    - Insertar en tabla 'cargas'
    
    // 3. Retornar array de IDs creados
}

func (s *SupabaseService) ObtenerOCrearUbicacion(direccion string) (string, error) {
    // Geocoding con Google Maps API
    // Buscar o crear en tabla 'ubicaciones'
}
```

## 4. Backend: Orquestador de procesamiento

### 4.1 Crear procesador principal
**Nuevo archivo**: `message_processor.go`

```go
package main

type MessageProcessor struct {
    messageStore    *MessageStore
    aiService       *AIService
    supabaseService *SupabaseService
    logger          waLog.Logger
}

type ProcessingResult struct {
    MessageID    string
    ChatJID      string
    Status       string
    AIResponse   string
    SupabaseIDs  []string
    Error        string
}

func (p *MessageProcessor) ProcessPendingMessages(limit int) ([]ProcessingResult, error) {
    // 1. Obtener mensajes procesables
    messages := p.messageStore.GetProcessableMessages(limit)
    
    // 2. Para cada mensaje:
    results := []ProcessingResult{}
    for _, msg := range messages {
        result := p.processMessage(msg)
        results = append(results, result)
        
        // 3. Guardar resultado en ai_processing_results
        p.saveProcessingResult(result)
        
        // 4. Si success, marcar mensaje como procesado
        if result.Status == "success" {
            p.messageStore.MarkMessageAsProcessed(msg.ID, msg.ChatJID)
        }
    }
    
    return results, nil
}

func (p *MessageProcessor) processMessage(msg ChatMessage) ProcessingResult {
    // 1. Llamar a IA
    aiResponse, err := p.aiService.ProcessMessage(msg.Content, msg.RealPhone)
    if err != nil {
        return ProcessingResult{Status: "error", Error: err.Error()}
    }
    
    // 2. Subir a Supabase
    ids, err := p.supabaseService.CrearCargasDesdeJSON(aiResponse)
    if err != nil {
        return ProcessingResult{Status: "error", Error: err.Error()}
    }
    
    // 3. Retornar resultado exitoso
    return ProcessingResult{
        MessageID: msg.ID,
        ChatJID: msg.ChatJID,
        Status: "success",
        AIResponse: string(aiResponse),
        SupabaseIDs: ids,
    }
}
```

## 5. Backend: Exponer funciones a frontend

### 5.1 Actualizar app.go
**Archivo**: `app.go`

```go
// Agregar campos
type App struct {
    // ... existentes
    messageProcessor *MessageProcessor
}

// Agregar función para procesar mensajes
func (a *App) ProcessMessages(limit int) ([]ProcessingResult, error) {
    if a.messageProcessor == nil {
        return nil, fmt.Errorf("processor not initialized")
    }
    return a.messageProcessor.ProcessPendingMessages(limit)
}

// Agregar función para obtener resultados
func (a *App) GetProcessingResults(limit int) ([]ProcessingResult, error) {
    // Query desde ai_processing_results
}

// Agregar función para obtener conteo de mensajes procesables
func (a *App) GetProcessableMessagesCount() (int, error) {
    // COUNT(*) de mensajes procesables
}
```

## 6. Backend: Configuración automática en background

### 6.1 Agregar goroutine de procesamiento automático
**Archivo**: `whatsapp_service.go`

```go
func (s *WhatsAppService) startAutoProcessor() {
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
            // Procesar hasta 10 mensajes cada 5 minutos
            s.messageProcessor.ProcessPendingMessages(10)
        }
    }()
}
```

## 7. Frontend: Nueva pestaña de Procesamiento IA

### 7.1 Actualizar HTML con nueva pestaña
**Archivo**: `frontend/dist/index.html`

Agregar en sección de tabs:
```html
<button class="tab" onclick="showTab('processing')">
  🤖 Procesamiento IA
</button>
```

Agregar nuevo panel:
```html
<div id="processingPanel" class="processing-panel">
  <div class="processing-header">
    <h2>🤖 Procesamiento con IA</h2>
    <p>Procesa mensajes de WhatsApp para generar cargas automáticamente</p>
  </div>
  
  <div class="processing-stats">
    <div class="stat-card">
      <span class="stat-value" id="pendingCount">0</span>
      <span class="stat-label">Mensajes pendientes</span>
    </div>
    <div class="stat-card">
      <span class="stat-value" id="processedCount">0</span>
      <span class="stat-label">Procesados hoy</span>
    </div>
    <div class="stat-card">
      <span class="stat-value" id="errorCount">0</span>
      <span class="stat-label">Errores</span>
    </div>
  </div>
  
  <div class="processing-actions">
    <button class="process-btn" onclick="processMessages()">
      ▶️ Procesar Mensajes
    </button>
    <button class="refresh-btn" onclick="refreshProcessingStats()">
      🔄 Actualizar
    </button>
  </div>
  
  <div class="processing-results">
    <h3>Resultados de Procesamiento</h3>
    <table id="processingResultsTable">
      <thead>
        <tr>
          <th>Fecha</th>
          <th>Mensaje</th>
          <th>Remitente</th>
          <th>Estado</th>
          <th>Cargas Creadas</th>
          <th>Acciones</th>
        </tr>
      </thead>
      <tbody id="processingResultsBody"></tbody>
    </table>
  </div>
</div>
```

### 7.2 Agregar estilos CSS
**Archivo**: `frontend/dist/index.html` (dentro de `<style>`)

```css
.processing-panel {
  display: none;
  flex-direction: column;
  padding: 20px;
  overflow-y: auto;
}

.processing-stats {
  display: flex;
  gap: 20px;
  margin-bottom: 30px;
}

.stat-card {
  flex: 1;
  background: #2a3942;
  padding: 20px;
  border-radius: 8px;
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 36px;
  font-weight: bold;
  color: #25d366;
}

.process-btn {
  background: #25d366;
  color: white;
  padding: 15px 30px;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  cursor: pointer;
}
```

### 7.3 Agregar JavaScript
**Archivo**: `frontend/dist/index.html` (dentro de `<script>`)

```javascript
// Función para procesar mensajes
async function processMessages() {
  const btn = document.querySelector('.process-btn');
  btn.disabled = true;
  btn.textContent = '⏳ Procesando...';
  
  try {
    const results = await window.go.main.App.ProcessMessages(10);
    
    // Actualizar tabla de resultados
    renderProcessingResults(results);
    
    // Actualizar stats
    refreshProcessingStats();
    
    alert(`✅ Procesados ${results.length} mensajes`);
  } catch (error) {
    alert(`❌ Error: ${error}`);
  } finally {
    btn.disabled = false;
    btn.textContent = '▶️ Procesar Mensajes';
  }
}

// Función para actualizar estadísticas
async function refreshProcessingStats() {
  try {
    const count = await window.go.main.App.GetProcessableMessagesCount();
    document.getElementById('pendingCount').textContent = count;
    
    // Obtener resultados del día
    const results = await window.go.main.App.GetProcessingResults(100);
    // ... actualizar stats
  } catch (error) {
    console.error('Error actualizando stats:', error);
  }
}

// Renderizar tabla de resultados
function renderProcessingResults(results) {
  const tbody = document.getElementById('processingResultsBody');
  tbody.innerHTML = '';
  
  results.forEach(result => {
    const row = document.createElement('tr');
    row.innerHTML = `
      <td>${formatDateTime(result.processed_at)}</td>
      <td>${truncate(result.content, 50)}</td>
      <td>${result.sender_name}</td>
      <td><span class="status-${result.status}">${result.status}</span></td>
      <td>${result.supabase_ids?.length || 0}</td>
      <td>
        <button onclick="viewDetails('${result.id}')">Ver</button>
      </td>
    `;
    tbody.appendChild(row);
  });
}
```

## 8. Archivos de configuración

### 8.1 Actualizar .gitignore
**Archivo**: `.gitignore`

Agregar:
```
# AI API Keys
ai-config.env
.env
```

### 8.2 Crear archivo de ejemplo
**Nuevo archivo**: `ai-config.env.example`

```
GEMINI_API_KEY=tu_api_key_aqui
GEMINI_MODEL=gemini-2.0-flash-exp
SUPABASE_KEY=tu_supabase_key_aqui
GOOGLE_MAPS_API_KEY=AIzaSyASe9Id-6Dr6lxr5mCb7O3l2HlmNrY-mRU
```

## 9. Documentación

### 9.1 Actualizar README
**Archivo**: `README.md`

Agregar sección:
```markdown
## Procesamiento con IA

La aplicación puede procesar automáticamente mensajes de WhatsApp para generar cargas de transporte usando Gemini AI.

### Configuración:
1. Crear archivo `ai-config.env` con tu API key de Gemini
2. Configurar variables de entorno
3. Procesar mensajes desde la pestaña "Procesamiento IA"

### Funcionamiento:
- Manual: Botón en UI
- Automático: Cada 5 minutos procesa hasta 10 mensajes
```

## Orden de implementación:

1. Base de datos (1.1, 1.2)
2. Configuración IA (2.1, 8.2)
3. Servicio IA (2.2)
4. Servicio Supabase (3.1)
5. Procesador (4.1)
6. Exposición a frontend (5.1)
7. Background processor (6.1)
8. Frontend (7.1, 7.2, 7.3)
9. Documentación (9.1)


### To-dos

- [ ] Crear tabla ai_processing_results y función GetProcessableMessages
- [ ] Crear ai_config.go y cargar variables de entorno
- [ ] Implementar ai_service.go con integración Gemini API
- [ ] Implementar supabase_service.go con geocoding y creación de cargas
- [ ] Crear message_processor.go orquestador principal
- [ ] Exponer funciones en app.go para frontend
- [ ] Implementar procesamiento automático cada 5 minutos
- [ ] Crear pestaña Procesamiento IA con estadísticas y tabla
- [ ] Implementar JavaScript para procesar y mostrar resultados
- [ ] Actualizar README y crear archivos de configuración