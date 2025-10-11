# 🔍 Cómo Ver Logs en Wails

## 📋 Dos Tipos de Logs

### 1. **Logs del Backend (PowerShell/CMD)**

Se ven automáticamente en la terminal donde ejecutaste `run-dev.bat`

**Logs que verás**:

```
[WhatsApp INFO] Procesando mensaje individual: A5B78653...
[WhatsApp INFO] Llamando a IA para mensaje A5B78653...
🤖 [15:47:14] Enviando request a Gemini...
📏 Tamaño del prompt: 3500 caracteres
⏱️ Esperando respuesta (timeout: 120s)...
```

### 2. **Logs del Frontend (Consola de Wails)**

Necesitas abrir la consola de desarrollo de Wails

## 🛠️ Cómo Abrir la Consola de Desarrollo en Wails

### Opción 1: Atajo de Teclado (En Desarrollo)

**Durante `wails dev`**:

- Presiona `F12` mientras la aplicación está en foco
- O presiona `Ctrl + Shift + I` (Windows/Linux)
- O presiona `Cmd + Option + I` (macOS)

### Opción 2: Click Derecho

1. Click derecho en cualquier parte de la aplicación
2. Selecciona "Inspect" o "Inspeccionar"
3. Se abrirá DevTools similar a un navegador

### Opción 3: Configuración en Código

Ya está configurado en `main.go` para modo desarrollo:

```go
Debug: options.Debug{
    OpenInspectorOnStartup: false,
},
```

Puedes cambiarlo a `true` para que se abra automáticamente.

## 📊 Qué Verás en Cada Consola

### Backend (PowerShell):

```
✅ Logs de Go (procesamiento, IA, Supabase)
✅ Logs de WhatsApp (conexión, mensajes)
✅ Logs del sistema (errores de compilación)
❌ NO verás logs de JavaScript del frontend
```

### Frontend (F12 en Wails):

```
✅ Logs de JavaScript (console.log, console.error)
✅ Errores del frontend
✅ Llamadas a funciones Go desde JS
❌ NO verás logs del backend de Go
```

## 🎯 Para Tu Caso Actual

### Para ver por qué "Procesar" no funciona:

1. **Presiona F12** en la aplicación Wails
2. **Ve a la pestaña "Console"**
3. **Click en ▶️** de un mensaje
4. **Deberías ver**:

   ```
   🔵 processSingleMessage llamado con: A5B78653..., 120363039914586861@g.us
   🚀 Iniciando procesamiento de mensaje: A5B78653...
   📞 Llamando a window.go.main.App.ProcessSingleMessage...
   ```

5. **Si ves un error**, compártelo
6. **Si no ves nada**, hay un problema con los event listeners

## 🔧 Troubleshooting

### Si F12 no abre nada:

1. Asegúrate de estar en modo desarrollo (`wails dev`)
2. Verifica que la ventana de Wails esté en foco
3. Intenta con `Ctrl + Shift + I`

### Si ves "ProcessSingleMessage is not a function":

- El backend no está exponiendo correctamente la función
- Reinicia la aplicación

### Si ves timeout:

- El mensaje está tardando más de 120 segundos
- Verifica los logs del backend (PowerShell)

## 📝 Logs Útiles

### Backend (PowerShell):

```bash
# Ver todo el flujo
[WhatsApp INFO] ...
🤖 Enviando request a Gemini...
⏱️ Respuesta recibida en 8.5 segundos
🔍 Buscando ubicación: ...
✅ Ubicación creada: ID xxx
```

### Frontend (F12):

```javascript
// Ver errores de JavaScript
🔵 processSingleMessage llamado...
📞 Llamando a window.go.main.App...
❌ Error completo: ...
```

## 🎨 Alternativa: Usar Logs del Backend

Si no puedes abrir F12, puedes confiar en los logs del backend:

- Si ves `[WhatsApp INFO] Procesando mensaje individual: ...` → La función se llamó
- Si NO ves ese log → El evento del botón no se está ejecutando

## 🚀 Próximos Pasos

1. Presiona **F12** en Wails
2. Ve a **Console**
3. Click en **▶️**
4. Comparte lo que ves (o screenshot)
