# 🤖 Sistema de Procesamiento con IA

Este documento describe el sistema de procesamiento automático de mensajes de WhatsApp usando Gemini AI para generar cargas de transporte.

## 📋 Descripción General

El sistema procesa automáticamente mensajes de WhatsApp que contienen información sobre cargas de transporte, los envía a Gemini AI para extraer datos estructurados, y luego sube las cargas generadas a Supabase.

## 🔧 Componentes del Sistema

### 1. Filtrado de Mensajes (`GetProcessableMessages`)

- **Filtros aplicados**:
  - `processed = 0` (no procesados)
  - `content IS NOT NULL AND content != ''` (con texto)
  - `real_phone IS NOT NULL AND real_phone != ''` (con teléfono asociado)
- **Orden**: Por timestamp ascendente (más antiguos primero)
- **Límite**: Configurable (por defecto 10 por lote)

### 2. Procesamiento con IA (`AIService`)

- **Modelo**: Gemini 2.0 Flash Experimental
- **Temperatura**: 0.1 (respuestas consistentes)
- **Max Tokens**: 8192
- **Prompt**: Carga desde `contecto_funcionalidad_ia.md`
- **Validación**: Verifica que la respuesta sea JSON válido

### 3. Integración con Supabase (`SupabaseService`)

- **Geocoding**: Google Maps API para convertir direcciones en coordenadas
- **Mapeo de Datos**: Convierte materiales, presentaciones, equipos a IDs de Supabase
- **Creación de Ubicaciones**: Busca o crea ubicaciones en la base de datos
- **Creación de Cargas**: Inserta cargas en la tabla `cargas`

### 4. Procesamiento Automático

- **Frecuencia**: Cada 5 minutos
- **Límite por Lote**: 10 mensajes
- **Background**: Ejecuta en goroutine separada
- **Logging**: Registra éxitos y errores

## 📊 Base de Datos

### Tabla `ai_processing_results`

```sql
CREATE TABLE ai_processing_results (
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
);
```

**Estados posibles**:

- `processing`: En proceso
- `success`: Exitoso
- `error`: Error en el procesamiento

## 🔑 Configuración

### Variables de Entorno Requeridas

```bash
# API Key de Gemini (REQUERIDO)
GEMINI_API_KEY=tu_api_key_aqui

# Modelo de Gemini (opcional)
GEMINI_MODEL=gemini-2.0-flash-exp

# Configuración de generación (opcional)
GEMINI_TEMPERATURE=0.1
GEMINI_MAX_TOKENS=8192

# API Key de Supabase (opcional, ya incluida por defecto)
SUPABASE_KEY=tu_supabase_key_aqui

# API Key de Google Maps (opcional, ya incluida por defecto)
GOOGLE_MAPS_API_KEY=tu_google_maps_key_aqui
```

### Archivo de Configuración

Crea `ai-config.env` basado en `ai-config.env.example`:

```bash
cp ai-config.env.example ai-config.env
# Edita ai-config.env con tus credenciales
```

## 🚀 Uso

### Procesamiento Manual

1. Ve a la pestaña "🤖 Procesamiento IA"
2. Haz clic en "▶️ Procesar Mensajes"
3. El sistema procesará hasta 10 mensajes pendientes
4. Verás los resultados en la tabla de abajo

### Procesamiento Automático

- Se inicia automáticamente cuando WhatsApp se conecta
- Procesa mensajes cada 5 minutos
- Los logs aparecen en la consola de la aplicación

### Estadísticas

La interfaz muestra:

- **Mensajes pendientes**: Cantidad de mensajes esperando procesamiento
- **Procesados hoy**: Mensajes procesados exitosamente en el día
- **Errores**: Cantidad de errores en el procesamiento

## 🔍 Flujo de Procesamiento

1. **Filtrado**: Se obtienen mensajes que cumplen los criterios
2. **Preparación**: Se agrega "ALT: +número_real" al contenido
3. **IA**: Se envía a Gemini con el prompt completo
4. **Validación**: Se verifica que la respuesta sea JSON válido
5. **Geocoding**: Se convierten direcciones en coordenadas
6. **Supabase**: Se crean ubicaciones y cargas
7. **Registro**: Se guarda el resultado en `ai_processing_results`
8. **Marcado**: Se marca el mensaje como procesado

## 📝 Formato de Respuesta de IA

La IA debe responder con un array JSON de cargas:

```json
[
  {
    "material": "Ganado",
    "presentacion": "Granel",
    "peso": "15000",
    "tipoEquipo": "Semi",
    "localidadCarga": "Villa del Rosario, Córdoba, Argentina",
    "localidadDescarga": "Emilia, Santa Fe, Argentina",
    "fechaCarga": "15/01/2024",
    "fechaDescarga": "16/01/2024",
    "telefono": "+5493512345678",
    "correo": "contacto@empresa.com",
    "puntoReferencia": "Frente al supermercado",
    "precio": "150000",
    "formaDePago": "Efectivo",
    "observaciones": "Carga de ganado bovino"
  }
]
```

## 🛠️ Solución de Problemas

### Error: "AI configuration is incomplete"

- Verifica que `GEMINI_API_KEY` esté configurado
- Asegúrate de que el archivo `ai-config.env` existe

### Error: "Invalid JSON response from AI"

- La IA devolvió una respuesta que no es JSON válido
- Revisa los logs para ver la respuesta exacta
- Puede ser que el mensaje no contenga información de carga

### Error: "Geocoding failed"

- Verifica que `GOOGLE_MAPS_API_KEY` sea válida
- Asegúrate de que la API de Google Maps esté habilitada

### Error: "Failed to insert carga"

- Verifica la conexión a Supabase
- Revisa que los datos de la carga sean válidos
- Comprueba que las ubicaciones se hayan creado correctamente

## 📈 Monitoreo

### Logs en Consola

- `🤖 Iniciando procesamiento automático cada 5 minutos`
- `🤖 Procesamiento automático completado: X exitosos, Y errores`

### Base de Datos

Consulta `ai_processing_results` para ver el historial:

```sql
SELECT
    status,
    COUNT(*) as count,
    DATE(processed_at) as fecha
FROM ai_processing_results
WHERE processed_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)
GROUP BY status, DATE(processed_at)
ORDER BY fecha DESC;
```

## 🔒 Seguridad

- Las API keys se almacenan en variables de entorno
- El archivo `ai-config.env` está en `.gitignore`
- Las credenciales nunca se exponen en el frontend
- Las comunicaciones con APIs externas usan HTTPS

## 📚 Referencias

- [Gemini API Documentation](https://ai.google.dev/docs)
- [Supabase Documentation](https://supabase.com/docs)
- [Google Maps Geocoding API](https://developers.google.com/maps/documentation/geocoding)
- [WhatsApp Business API](https://developers.facebook.com/docs/whatsapp)
