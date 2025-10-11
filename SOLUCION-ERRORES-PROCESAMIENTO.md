# 🔧 Solución a Errores de Procesamiento

## ❌ Problemas Identificados

### 1. Error de JSON con Backticks

**Error**: `invalid character '`' looking for beginning of value`

**Causa**: Gemini AI responde con bloques de código markdown:

````
```json
[{"material": "Ganado", ...}]
````

```

En lugar de JSON puro:
```

[{"material": "Ganado", ...}]

````

### 2. Error 429 - Quota Excedida
**Error**: `You exceeded your current quota, please check your plan and billing details`

**Causa**:
- El tier gratuito de Gemini tiene límite de **50 requests por día**
- Ya alcanzaste ese límite con tu API key actual

## ✅ Soluciones Implementadas

### Solución 1: Limpieza Automática de Markdown

**Función `cleanAIResponse()`** en `ai_service.go`:
```go
func cleanAIResponse(response string) string {
    // Remover ```json al inicio
    if strings.HasPrefix(strings.TrimSpace(cleaned), "```json") {
        cleaned = strings.TrimPrefix(strings.TrimSpace(cleaned), "```json")
    } else if strings.HasPrefix(strings.TrimSpace(cleaned), "```") {
        cleaned = strings.TrimPrefix(strings.TrimSpace(cleaned), "```")
    }

    // Remover ``` al final
    if strings.HasSuffix(strings.TrimSpace(cleaned), "```") {
        cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), "```")
    }

    return strings.TrimSpace(cleaned)
}
````

**Resultado**: Ahora acepta respuestas con o sin markdown ✅

### Solución 2: Rotación Automática de API Keys

**Cuando detecta error 429**:

1. Llama a `KeysManager.TryNextKey()`
2. Cambia automáticamente a la siguiente API key del pool
3. Reintenta la llamada con la nueva key
4. Máximo 1 cambio de key por mensaje (evita loops)

**Código**:

```go
if resp.StatusCode == 429 && s.config.KeysManager != nil && retryCount == 0 {
    newKey, err := s.config.KeysManager.TryNextKey()
    if err == nil {
        s.config.APIKey = newKey
        return s.processMessageWithRetry(content, realPhone, retryCount+1)
    }
}
```

### Solución 3: Prompt Mejorado

**Instrucción explícita** agregada al prompt:

````
**IMPORTANTE: Responde ÚNICAMENTE con el array JSON, SIN usar bloques de código markdown (```), SIN backticks, SIN explicaciones. Solo el JSON puro.**
````

Esto reduce la probabilidad de que IA use markdown.

## 🔑 Cómo Agregar Más API Keys

### Opción 1: Desde la Interfaz (Recomendado)

1. Ve a la pestaña "🤖 Procesamiento IA"
2. En la sección "🔑 Gestión de API Keys"
3. Ingresa tu nueva API key
4. Dale un nombre (ej: "Key Secundaria")
5. Click en "➕ Agregar Key"

### Opción 2: Obtener Nuevas API Keys de Gemini

1. Ve a: https://makersuite.google.com/app/apikey
2. Crea una nueva API key (puedes crear varias con diferentes cuentas de Google)
3. Agrégala desde la interfaz

### Opción 3: Esperar Reset Diario

El límite de 50 requests se resetea cada 24 horas. Puedes:

- Esperar hasta mañana
- O agregar una nueva API key ahora

## 🎯 Pool de API Keys

### Ventajas del Sistema de Pool:

1. **Rotación automática**: Cambia de key cuando una alcanza el límite
2. **Múltiples keys**: Puedes tener 5-10 keys y usar 500 requests/día
3. **Sin downtime**: Si una key falla, usa otra automáticamente
4. **Tracking de errores**: Cada key registra cuántos errores tuvo

### Estrategia Recomendada:

**Para desarrollo**:

- 2-3 API keys (diferentes cuentas de Google)
- ~100-150 requests diarios

**Para producción**:

- 5-10 API keys
- ~250-500 requests diarios
- O considerar un plan pago de Gemini

## 📊 Límites de Gemini API

### Tier Gratuito:

- **50 requests por día** por API key
- **15 requests por minuto**
- Resetea cada 24 horas

### Tier Pago (si necesitas más):

- **1,500 requests por minuto**
- **Millones de requests por día**
- Costo: ~$0.001 por request

## 🔍 Monitoreo de Uso

### Ver qué key está activa:

En la UI de "Procesamiento IA", verás:

- ✓ Activa - Key que se está usando actualmente
- Contador de errores por key

### Rotación Manual:

Si una key tiene muchos errores:

1. Click en "Activar" en otra key
2. El sistema usará la nueva key inmediatamente

## 🛠️ Troubleshooting

### Si sigues viendo error 429:

1. **Todas tus keys alcanzaron el límite**
2. **Solución inmediata**: Agregar más API keys
3. **Solución temporal**: Esperar 24 horas para reset

### Si ves error de JSON con backticks:

1. **Ya está solucionado** con la función `cleanAIResponse()`
2. Si persiste, reporta el mensaje exacto para mejorar la limpieza

### Si la rotación automática no funciona:

1. Verifica que tengas más de 1 API key configurada
2. Verifica que las otras keys sean válidas
3. Revisa los logs para ver si intentó cambiar de key

## 📈 Recomendaciones

1. **Agregar 2-3 API keys adicionales ahora**
2. **Usar cuentas de Google diferentes** para cada key
3. **Configurar el procesamiento automático a cada 10 minutos** (en lugar de 5) para distribuir el uso
4. **Monitorear el contador de errores** de cada key
5. **Considerar plan pago** si procesas más de 200 mensajes diarios

## 🎯 Próximos Pasos

1. **Agrega 2-3 API keys más** desde la interfaz
2. **Reinicia el procesamiento** - ahora rotará automáticamente
3. **Monitorea** que esté usando diferentes keys
4. **Si alcanzas límite en todas**: Espera 24 horas o agrega más keys
