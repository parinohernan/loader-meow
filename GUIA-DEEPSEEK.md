# 🤖 Guía DeepSeek - Configuración y Uso

Guía completa para usar **DeepSeek** en Loader Meow.

---

## 📋 ¿Qué es DeepSeek?

[DeepSeek](https://www.deepseek.com/) es un proveedor de IA chino que ofrece modelos de lenguaje de alta calidad a precios **muy económicos**. Es compatible con el formato OpenAI, lo que facilita su integración.

### **Ventajas:**

- ✅ **Muy económico** (~$0.14 USD por 1M tokens de entrada)
- ✅ **API compatible con OpenAI** (fácil integración)
- ✅ **Modelos potentes** (deepseek-chat, deepseek-reasoner)
- ✅ **Gran contexto** (32,768 tokens)
- ✅ **Buena velocidad de respuesta**

### **Desventajas:**

- ⚠️ **Requiere recarga de créditos** (no hay plan gratuito perpetuo)
- ⚠️ **Servicio chino** (posibles restricciones regionales)

---

## 🚀 Configuración Inicial

### **Paso 1: Crear Cuenta en DeepSeek**

1. Ve a [https://platform.deepseek.com](https://platform.deepseek.com)
2. Haz clic en **"Sign Up"** o **"Register"**
3. Completa el registro con tu email
4. Verifica tu email

### **Paso 2: Obtener API Key**

1. Inicia sesión en [https://platform.deepseek.com](https://platform.deepseek.com)
2. Ve a la sección **"API Keys"**: [https://platform.deepseek.com/api_keys](https://platform.deepseek.com/api_keys)
3. Haz clic en **"Create API Key"**
4. Copia la key (guárdala en un lugar seguro, no se volverá a mostrar)

### **Paso 3: Agregar Créditos**

⚠️ **IMPORTANTE:** DeepSeek requiere que agregues créditos antes de usar la API.

1. Ve a **"Billing"** o **"Credits"** en el panel
2. Haz clic en **"Add Credits"** o **"Recharge"**
3. Selecciona el monto (mínimo suele ser $5-10 USD)
4. Completa el pago (tarjeta de crédito/débito)
5. Espera la confirmación (suele ser instantáneo)

**Recomendación:** Empieza con $5-10 USD. Con eso puedes procesar miles de mensajes.

---

## 🔧 Configuración en Loader Meow

### **Paso 1: Aplicar Migración SQL**

Si aún no has agregado DeepSeek a la base de datos:

**Opción A: PHPMyAdmin**

```sql
-- Copiar y ejecutar en PHPMyAdmin
INSERT IGNORE INTO ai_providers (name, display_name, base_url, priority)
VALUES ('deepseek', 'DeepSeek', 'https://api.deepseek.com/v1', 85);

SET @deepseek_provider_id = (SELECT id FROM ai_providers WHERE name = 'deepseek');

INSERT IGNORE INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_default)
VALUES
    (@deepseek_provider_id, 'deepseek-chat', 'DeepSeek Chat', 4096, 32768, 1),
    (@deepseek_provider_id, 'deepseek-coder', 'DeepSeek Coder', 4096, 32768, 0);
```

**Opción B: Script automático**

```bash
.\apply-deepseek-migration.bat
```

### **Paso 2: Reconstruir la App**

```bash
.\rebuild-dev.bat
```

### **Paso 3: Agregar API Key en la App**

1. Abre **Loader Meow**
2. Ve a **"⚙️ Configuración IA"**
3. Haz clic en **"➕ Agregar Configuración"**
4. Configura:
   - **Proveedor:** DeepSeek
   - **Modelo:** DeepSeek Chat
   - **API Key:** Pega tu key de DeepSeek
   - **Nombre descriptivo:** "DeepSeek Principal" (o lo que prefieras)
5. Haz clic en **"Guardar"**
6. **Activa** la configuración (toggle verde)

---

## 💰 Precios y Consumo

### **Tabla de Precios (Octubre 2025)**

| Modelo                | Entrada (1M tokens) | Salida (1M tokens) | Contexto      |
| --------------------- | ------------------- | ------------------ | ------------- |
| **deepseek-chat**     | ~$0.14 USD          | ~$0.28 USD         | 32,768 tokens |
| **deepseek-coder**    | ~$0.14 USD          | ~$0.28 USD         | 32,768 tokens |
| **deepseek-reasoner** | ~$0.55 USD          | ~$2.19 USD         | 32,768 tokens |

_Nota: Precios aproximados según [documentación oficial](https://api-docs.deepseek.com/quick_start/pricing)_

### **Estimación de Costos para Loader Meow**

Asumiendo un mensaje típico de WhatsApp procesado con IA:

- **Prompt del sistema:** ~1,500 tokens
- **Mensaje del usuario:** ~500 tokens
- **Respuesta de la IA:** ~300 tokens
- **Total por mensaje:** ~2,300 tokens

**Costo por mensaje:**

```
Entrada: 2,000 tokens = $0.00028 USD
Salida:    300 tokens = $0.00008 USD
─────────────────────────────────────
TOTAL:                = $0.00036 USD (~0.04 centavos)
```

**Con $10 USD puedes procesar:**

- ~27,777 mensajes
- ~925 mensajes por día durante 30 días

---

## ❌ Errores Comunes y Soluciones

### **Error 402: Insufficient Balance**

```json
{
  "error": {
    "message": "Insufficient Balance",
    "type": "unknown_error",
    "code": "invalid_request_error"
  }
}
```

**Causa:** No tienes créditos en tu cuenta de DeepSeek.

**Solución:**

1. Ve a [https://platform.deepseek.com](https://platform.deepseek.com)
2. Sección **"Billing"**
3. Agrega créditos (mínimo $5-10 USD)

---

### **Error 401: Unauthorized / Invalid API Key**

```json
{
  "error": {
    "message": "Invalid API Key",
    "type": "invalid_request_error"
  }
}
```

**Causa:** La API key es incorrecta o fue revocada.

**Solución:**

1. Verifica que copiaste la key correctamente (sin espacios extras)
2. Ve a [https://platform.deepseek.com/api_keys](https://platform.deepseek.com/api_keys)
3. Si es necesario, genera una nueva key
4. Actualiza la key en **"⚙️ Configuración IA"** de la app

---

### **Error 429: Rate Limit Exceeded**

```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error"
  }
}
```

**Causa:** Excediste el límite de requests por minuto/día.

**Solución:**

- ✅ El sistema **rotará automáticamente** a otra API key si tienes más configuradas
- O espera unos minutos y reintenta
- O agrega más API keys en **"⚙️ Configuración IA"**

---

### **Error: "unsupported provider: deepseek"**

**Causa:** El código no reconoce el proveedor DeepSeek.

**Solución:**

1. Verifica que ejecutaste la migración SQL
2. Verifica que reconstruiste la app: `.\rebuild-dev.bat`
3. Verifica que el código en `ai_provider_service.go` incluye el case `"deepseek"`

---

## 🔄 Rotación Automática de API Keys

Si tienes **múltiples API keys de DeepSeek**, el sistema rotará automáticamente cuando:

1. ❌ Una key alcance el **límite de rate** (429)
2. ❌ Una key tenga **saldo insuficiente** (402)
3. ❌ Ocurra cualquier **error recuperable**

**Ejemplo:**

```
Key 1 (Principal)    → 402 (sin saldo) → Rota
Key 2 (Secundaria)   → ✅ Procesa exitosamente
```

**Para agregar más keys:**

1. Ve a **"⚙️ Configuración IA"**
2. Click **"➕ Agregar Configuración"**
3. Selecciona **DeepSeek** + **deepseek-chat**
4. Agrega tu nueva API key
5. Asigna un nombre descriptivo: "DeepSeek Key 2"

---

## 📊 Monitoreo de Uso

### **En DeepSeek Platform:**

1. Ve a [https://platform.deepseek.com](https://platform.deepseek.com)
2. Sección **"Usage"** o **"Billing"**
3. Verás:
   - Créditos restantes
   - Uso diario/mensual
   - Historial de requests
   - Costo por modelo

### **En Loader Meow:**

Los logs de PowerShell te mostrarán:

```
🤖 [DeepSeek] Enviando request...
⏱️ Respuesta recibida en 1.23 segundos
📥 Recibiendo respuesta de DeepSeek... (Status: 200)
📦 Respuesta recibida: 450 bytes
✅ Procesamiento exitoso con DeepSeek - DeepSeek Chat (DeepSeek Principal)
```

---

## 🆚 Comparación con Otros Proveedores

| Característica             | DeepSeek              | Groq        | Gemini    | Grok      |
| -------------------------- | --------------------- | ----------- | --------- | --------- |
| **Precio/1M tokens**       | $0.14-0.28            | Gratis\*    | Gratis\*  | $5.00     |
| **Velocidad**              | Rápido                | Muy rápido  | Medio     | Rápido    |
| **Contexto**               | 32K                   | 131K        | 1M+       | 131K      |
| **Gratis**                 | ❌ (requiere recarga) | ✅          | ✅        | ❌        |
| **Calidad**                | Excelente             | Muy buena   | Excelente | Muy buena |
| **Límite diario (gratis)** | -                     | 100K tokens | Variable  | -         |

\*Gratis con límites de uso diario/mensual

---

## 💡 Recomendaciones

### **Cuándo usar DeepSeek:**

✅ **SÍ usar si:**

- Necesitas procesar **muchos mensajes** (miles por día)
- Quieres **precios predecibles** y muy bajos
- Agotaste los límites gratuitos de Groq/Gemini
- Buscas **buena calidad a bajo costo**

❌ **NO usar si:**

- Estás probando/desarrollando (usa Groq o Gemini gratis primero)
- No quieres agregar método de pago
- Procesas muy pocos mensajes (<100/día)

### **Estrategia Recomendada:**

1. **Desarrollo:** Usa **Groq** (gratis, rápido)
2. **Producción baja:** Usa **Gemini** (gratis con límites generosos)
3. **Producción alta:** Usa **DeepSeek** (económico, sin límites si pagas)
4. **Backup:** Ten keys de **múltiples proveedores** para rotación automática

---

## 🔗 Enlaces Útiles

- **DeepSeek Platform:** [https://platform.deepseek.com](https://platform.deepseek.com)
- **Documentación API:** [https://api-docs.deepseek.com](https://api-docs.deepseek.com)
- **API Keys:** [https://platform.deepseek.com/api_keys](https://platform.deepseek.com/api_keys)
- **Precios:** [https://api-docs.deepseek.com/quick_start/pricing](https://api-docs.deepseek.com/quick_start/pricing)
- **Modelos Disponibles:** [https://api-docs.deepseek.com/quick_start/models_pricing](https://api-docs.deepseek.com/quick_start/models_pricing)

---

## 📞 Soporte

Si tienes problemas con DeepSeek:

1. **Verifica logs en PowerShell** (busca mensajes con `[DeepSeek]`)
2. **Revisa tu saldo** en [https://platform.deepseek.com](https://platform.deepseek.com)
3. **Consulta la documentación oficial:** [https://api-docs.deepseek.com](https://api-docs.deepseek.com)
4. **Revisa esta guía:** Errores comunes arriba

---

**¿DeepSeek agregado correctamente?** ✅ Ahora puedes procesar miles de mensajes a bajo costo! 🚀
