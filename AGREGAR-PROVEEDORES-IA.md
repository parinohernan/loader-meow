# 🤖 Guía: Agregar Nuevos Proveedores de IA

Esta guía explica cómo agregar nuevos proveedores de IA al sistema **Loader Meow**.

---

## 📊 Proveedores Actuales

| Proveedor           | Formato API       | Dificultad | Estado          |
| ------------------- | ----------------- | ---------- | --------------- |
| **Gemini** (Google) | Propietario       | Media      | ✅ Implementado |
| **Groq**            | OpenAI-compatible | Fácil      | ✅ Implementado |
| **Grok** (xAI)      | OpenAI-compatible | Fácil      | ✅ Implementado |
| **DeepSeek**        | OpenAI-compatible | Fácil      | ✅ Implementado |
| **Qwen** (Alibaba)  | Propietario       | Difícil    | ✅ Implementado |

---

## 🎯 Tipos de API y Complejidad

### **Formato OpenAI-compatible** (⭐ Fácil)

Proveedores que usan el mismo formato de request/response que OpenAI.

**Proveedores compatibles:**

- Groq ✅
- Grok (xAI) ✅
- DeepSeek ✅
- **Together AI** 🆕
- **Perplexity** 🆕
- **Mistral AI** 🆕
- **OpenRouter** 🆕
- **Fireworks AI** 🆕

**Ventaja:** Solo necesitas cambiar:

1. URL base (`base_url`)
2. Nombre del modelo
3. API key

### **Formato Propietario** (⭐⭐ Media/Difícil)

Proveedores con formato único de request/response.

**Ejemplos:**

- Gemini (Google) ✅
- Claude (Anthropic) 🆕
- Cohere 🆕

**Requiere:** Crear una función `callXXX()` específica con su propio formato.

---

## 🛠️ Pasos para Agregar un Proveedor

### **Opción 1: Proveedor OpenAI-compatible** (Recomendado)

#### **Paso 1: Agregar a la Base de Datos**

Ejecuta este SQL (reemplaza los valores):

```sql
-- Agregar proveedor
INSERT INTO ai_providers (name, display_name, base_url, priority)
VALUES ('together', 'Together AI', 'https://api.together.xyz/v1', 82);

-- Obtener ID del proveedor
SET @provider_id = (SELECT id FROM ai_providers WHERE name = 'together');

-- Agregar modelos
INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default)
VALUES
    (@provider_id, 'meta-llama/Llama-3.3-70B-Instruct-Turbo', 'Llama 3.3 70B Turbo', 8192, 131072, 1),
    (@provider_id, 'mistralai/Mixtral-8x22B-Instruct-v0.1', 'Mixtral 8x22B', 8192, 65536, 0);
```

**Parámetros importantes:**

- `name`: Nombre interno (minúsculas, sin espacios)
- `display_name`: Nombre visible en el UI
- `base_url`: URL base de la API (sin el `/chat/completions`)
- `priority`: Mayor = más prioritario (100 = máxima prioridad)
- `max_tokens`: Tokens máximos de salida
- `context_window`: Ventana de contexto del modelo

#### **Paso 2: Agregar Case en el Código**

Edita `ai_provider_service.go`:

```go
switch config.ProviderName {
case "gemini":
    response, err = s.callGemini(config, systemPrompt, userMessage, realPhone)
case "groq":
    response, err = s.callGroq(config, systemPrompt, userMessage, realPhone)
case "together":  // ← NUEVO
    response, err = s.callOpenAICompatible(config, systemPrompt, userMessage, realPhone, "Together AI")
// ... resto de cases
}
```

#### **Paso 3: Crear Función Genérica** (opcional, recomendado)

Si agregas varios proveedores OpenAI-compatible, crea una función genérica:

```go
// callOpenAICompatible llama a cualquier API compatible con OpenAI
func (s *AIProviderService) callOpenAICompatible(config *AIConfigDB, systemPrompt, userMessage, realPhone, providerName string) ([]byte, error) {
	request := map[string]interface{}{
		"model": config.ModelName,
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": fmt.Sprintf("%s\n\nIMPORTANTE: Debes responder con un array JSON.", systemPrompt),
			},
			{
				"role": "user",
				"content": fmt.Sprintf("Teléfono: %s\n\n%s", realPhone, userMessage),
			},
		},
		"temperature": 0.7,
		"max_tokens": config.MaxTokens,
	}

	jsonData, _ := json.Marshal(request)
	url := fmt.Sprintf("%s/chat/completions", config.BaseURL)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 429 {
			fmt.Printf("🚨 ERROR 429 EN %s\n", providerName)
		}
		return nil, fmt.Errorf("%s API error %d: %s", providerName, resp.StatusCode, string(body))
	}

	// Parsear respuesta
	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	json.Unmarshal(body, &apiResp)
	return []byte(apiResp.Choices[0].Message.Content), nil
}
```

#### **Paso 4: Actualizar el Switch**

```go
case "together":
    response, err = s.callOpenAICompatible(config, systemPrompt, userMessage, realPhone, "Together AI")
case "perplexity":
    response, err = s.callOpenAICompatible(config, systemPrompt, userMessage, realPhone, "Perplexity")
case "mistral":
    response, err = s.callOpenAICompatible(config, systemPrompt, userMessage, realPhone, "Mistral AI")
```

---

### **Opción 2: Proveedor con Formato Propietario**

Si el proveedor NO es compatible con OpenAI (ej: Claude):

#### **Paso 1: Investigar la API**

1. Lee la documentación oficial del proveedor
2. Identifica:
   - URL del endpoint
   - Formato del request (JSON)
   - Formato de la response
   - Headers necesarios
   - Manejo de errores (429, 400, etc.)

#### **Paso 2: Crear Función Específica**

Copia y modifica la función `callGemini` o `callQwen` según el formato:

```go
func (s *AIProviderService) callClaude(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	// Request específico de Claude
	request := map[string]interface{}{
		"model": config.ModelName,
		"max_tokens": config.MaxTokens,
		"system": systemPrompt,  // ← Claude usa "system" separado
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": userMessage,
			},
		},
	}

	// ... resto del código adaptado
}
```

#### **Paso 3: Agregar Case**

```go
case "claude":
    response, err = s.callClaude(config, systemPrompt, userMessage, realPhone)
```

---

## 🧪 Cómo Probar un Nuevo Proveedor

1. **Agrega el proveedor y modelo a la BD**
2. **Reconstruye la app:** `.\rebuild-dev.bat`
3. **Abre la app** y ve a **"⚙️ Configuración IA"**
4. **Agrega una API key** del nuevo proveedor
5. **Activa la configuración**
6. **Procesa un mensaje de prueba**
7. **Revisa los logs** en la consola de PowerShell

---

## 📋 Lista de Proveedores Recomendados

### **OpenAI-compatible** (Agregar fácil)

| Proveedor        | URL Base                                | Gratis        | Notas                        |
| ---------------- | --------------------------------------- | ------------- | ---------------------------- |
| **Together AI**  | `https://api.together.xyz/v1`           | Sí (trial)    | Muchos modelos open-source   |
| **Perplexity**   | `https://api.perplexity.ai`             | Sí (limitado) | Excelente para búsqueda      |
| **Mistral AI**   | `https://api.mistral.ai/v1`             | Sí (trial)    | Modelos europeos             |
| **OpenRouter**   | `https://openrouter.ai/api/v1`          | Sí            | Agrega 100+ modelos a la vez |
| **Fireworks AI** | `https://api.fireworks.ai/inference/v1` | Sí (trial)    | Muy rápido                   |

### **Formato Propietario** (Más trabajo)

| Proveedor              | URL Base                       | Gratis        | Dificultad |
| ---------------------- | ------------------------------ | ------------- | ---------- |
| **Claude** (Anthropic) | `https://api.anthropic.com/v1` | Sí (trial)    | Media      |
| **Cohere**             | `https://api.cohere.ai/v1`     | Sí (limitado) | Media      |

---

## 🎯 Ejemplo Completo: Agregar Together AI

### **1. SQL (migrations/add_together_provider.sql)**

```sql
INSERT INTO ai_providers (name, display_name, base_url, priority)
VALUES ('together', 'Together AI', 'https://api.together.xyz/v1', 82);

SET @provider_id = (SELECT id FROM ai_providers WHERE name = 'together');

INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default)
VALUES
    (@provider_id, 'meta-llama/Llama-3.3-70B-Instruct-Turbo', 'Llama 3.3 70B Turbo', 8192, 131072, 1);
```

### **2. Código (ai_provider_service.go)**

```go
// En el switch
case "together":
    response, err = s.callTogether(config, systemPrompt, userMessage, realPhone)

// Nueva función (copia de callGroq)
func (s *AIProviderService) callTogether(config *AIConfigDB, systemPrompt, userMessage, realPhone string) ([]byte, error) {
	request := map[string]interface{}{
		"model": config.ModelName,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf("Teléfono: %s\n\n%s", realPhone, userMessage)},
		},
		"temperature": 0.7,
		"max_tokens": config.MaxTokens,
	}

	jsonData, _ := json.Marshal(request)
	url := "https://api.together.xyz/v1/chat/completions"

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	resp, _ := s.client.Do(req)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("together API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	json.Unmarshal(body, &apiResp)
	return []byte(apiResp.Choices[0].Message.Content), nil
}
```

### **3. Ejecutar migración**

```bash
mysql -u root -p whatsapp_cargas < migrations/add_together_provider.sql
```

### **4. Reconstruir**

```bash
.\rebuild-dev.bat
```

---

## ✅ Checklist Final

Antes de usar un nuevo proveedor, verifica:

- [ ] Provider agregado en `ai_providers` tabla
- [ ] Al menos 1 modelo agregado en `ai_models` tabla
- [ ] Case agregado en el switch de `ai_provider_service.go`
- [ ] Función `callXXX()` implementada
- [ ] Código compilado sin errores (`.\rebuild-dev.bat`)
- [ ] API key agregada en "⚙️ Configuración IA"
- [ ] Mensaje de prueba procesado exitosamente
- [ ] Logs verificados en PowerShell

---

## 🆘 Solución de Problemas

### **Error: "unsupported provider: xxx"**

→ Falta agregar el case en el switch de `ai_provider_service.go`

### **Error: "no active AI configuration"**

→ Debes activar una configuración en "⚙️ Configuración IA"

### **Error 401: "Invalid API key"**

→ Verifica que la API key sea correcta

### **Error 429: "Rate limit"**

→ Sistema rotará automáticamente a otra key si hay disponibles

### **Error: "failed to parse response"**

→ El formato de respuesta del proveedor es diferente, necesitas adaptar el parsing

---

## 📚 Recursos Útiles

- **OpenAI API Reference:** https://platform.openai.com/docs/api-reference
- **Groq Docs:** https://console.groq.com/docs
- **DeepSeek Docs:** https://platform.deepseek.com/docs
- **Together AI Docs:** https://docs.together.ai/
- **OpenRouter Docs:** https://openrouter.ai/docs

---

**¿Necesitas ayuda?** Revisa los logs en PowerShell o contacta al desarrollador.
