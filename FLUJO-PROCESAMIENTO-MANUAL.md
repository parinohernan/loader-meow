# 🎯 Flujo de Procesamiento Manual

## 📋 Cambios Implementados

El sistema ha cambiado de **procesamiento automático en lote** a **procesamiento manual mensaje por mensaje**.

## 🔄 Nuevo Flujo

### 1. Vista Principal

Al abrir la pestaña "🤖 Procesamiento IA" verás:

```
┌─────────────────────────────────────────────────────────┐
│  🔑 Gestión de API Keys                                 │
│  [Lista de keys + Agregar nueva]                        │
├─────────────────────────────────────────────────────────┤
│  📊 Estadísticas                                        │
│  ┌──────────────┬──────────────┬──────────────┐        │
│  │ Pendientes:5 │ Procesados:12│ Errores: 2   │        │
│  └──────────────┴──────────────┴──────────────┘        │
├─────────────────────────────────────────────────────────┤
│  📋 Mensajes Sin Procesar                               │
│  ┌──────┬────────────────────┬────────┬─────────────┐  │
│  │Fecha │Mensaje (con ALT)   │Remitente│Acciones    │  │
│  ├──────┼────────────────────┼────────┼─────────────┤  │
│  │10/11 │Tengo 15tn maiz...  │+549... │▶️ ✏️ 🗑️   │  │
│  │      │ALT: +5492924...    │        │            │  │
│  ├──────┼────────────────────┼────────┼─────────────┤  │
│  │10/11 │Necesito Semi...    │+549... │▶️ ✏️ 🗑️   │  │
│  │      │ALT: +5493512...    │        │            │  │
│  └──────┴────────────────────┴────────┴─────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### 2. Contenido de Mensajes

**Cada mensaje muestra**:

- ✅ **Contenido original** del mensaje de WhatsApp
- ✅ **ALT: +teléfono_real** ya agregado automáticamente
- ✅ **Fecha y hora** del mensaje
- ✅ **Nombre del remitente**
- ✅ **Teléfono real** asociado
- ✅ **Contador de intentos** (0/3, 1/3, 2/3, 3/3)

**Ejemplo de contenido mostrado**:

```
Tengo 15 toneladas de maíz en Córdoba para llevar a Rosario.
Necesito semi para el 15/10. Pago $150000

ALT: +5492924406159
```

## 🎮 Acciones por Mensaje

### ▶️ Procesar (Botón Verde)

**Qué hace**:

1. Toma el mensaje con el ALT ya incluido
2. Lo envía a Gemini AI
3. Espera la respuesta (JSON de cargas)
4. Sube las cargas a Supabase
5. Marca el mensaje como procesado
6. Remueve el mensaje de la lista

**Cuándo usar**:

- Mensaje se ve correcto y completo
- Listo para procesar sin modificaciones

**Feedback**:

- Botón cambia a "⏳" mientras procesa
- Notificación: "✅ Mensaje procesado exitosamente. X cargas creadas"
- O: "❌ Error procesando: [mensaje de error]"

### ✏️ Editar

**Qué hace**:

1. Abre modal con textarea del contenido
2. Puedes modificar el texto
3. Al guardar:
   - Actualiza el contenido en la BD
   - Resetea contador de intentos a 0
   - Marca como no procesado
   - El mensaje permanece en la lista (actualizado)

**Cuándo usar**:

- Mensaje tiene errores de ortografía
- Falta información importante
- Quieres reformular para que IA lo entienda mejor
- Necesitas agregar detalles adicionales

**Características**:

- Textarea grande con scroll
- Font monospace para mejor legibilidad
- Botón "💾 Guardar y Reprocesar"
- Luego puedes usar ▶️ para procesarlo

### 🗑️ Eliminar (Botón Rojo)

**Qué hace**:

1. Pide confirmación
2. Elimina el mensaje de la BD permanentemente
3. Remueve de la lista

**Cuándo usar**:

- Mensaje de prueba
- Spam o basura
- Información incorrecta irreparable
- Duplicados que pasaron el filtro

**Advertencia**: ⚠️ Acción irreversible

## 📊 Estadísticas

### Mensajes Pendientes

- Cuenta de mensajes sin procesar
- Actualiza en tiempo real
- Incluye mensajes con intentos fallidos (< 3)

### Procesados Hoy

- Mensajes procesados exitosamente en el día actual
- Se resetea diariamente

### Errores

- Mensajes que fallaron en el día actual
- Incluye todos los intentos fallidos

## 🔄 Flujo de Trabajo Típico

### Caso 1: Mensaje Simple

```
1. Revisar mensaje en la lista
2. Verificar que tiene ALT correcto
3. Click en ▶️ Procesar
4. Esperar notificación de éxito
5. Mensaje desaparece de la lista
```

### Caso 2: Mensaje Necesita Edición

```
1. Click en ✏️ Editar
2. Modificar el texto en el modal
3. Click en "💾 Guardar"
4. Verificar que el mensaje actualizado se ve bien
5. Click en ▶️ Procesar
6. Mensaje procesado exitosamente
```

### Caso 3: Mensaje con Error

```
1. Click en ▶️ Procesar
2. Ver notificación de error
3. Analizar el error
4. Click en ✏️ Editar para corregir
5. O Click en 🗑️ Eliminar si no tiene solución
```

### Caso 4: Error de Quota (429)

```
1. Click en ▶️ Procesar
2. Error: "Quota exceeded"
3. Sistema intenta automáticamente con otra key
4. Si tiene otra key disponible, procesa exitosamente
5. Si no: Agregar nueva key desde la sección superior
```

## ⚙️ Ventajas del Flujo Manual

### ✅ Control Total

- Decides cuándo procesar cada mensaje
- Puedes revisar antes de enviar a IA
- No desperdicias requests en mensajes malos

### ✅ Mejor Uso de Quota

- No procesas automáticamente mensajes con errores
- Puedes editar antes de consumir quota
- Distribuyes el uso a lo largo del día

### ✅ Debugging Más Fácil

- Ves inmediatamente si un mensaje falla
- Puedes editarlo y reprocesar en el momento
- No pierdes tiempo esperando ciclos automáticos

### ✅ Flexibilidad

- Procesas en el orden que quieras
- Puedes saltarte mensajes problemáticos
- Eliminas basura antes de procesarla

## 🎯 Recomendaciones de Uso

### Inicio del Día

1. Abre la pestaña "Procesamiento IA"
2. Revisa la lista de mensajes pendientes
3. Elimina spam/basura primero
4. Edita mensajes que necesitan corrección
5. Procesa mensajes buenos de a uno

### Durante el Día

1. Cuando llegue un mensaje nuevo, aparecerá automáticamente en la BD
2. Click en "🔄 Actualizar Lista" para verlo
3. Revísalo y procésalo cuando estés listo

### Gestión de Errores

1. Si un mensaje falla, aparece en rojo el contador de intentos
2. Revisa por qué falló (✏️ para ver contenido completo)
3. Edita si es necesario o elimina si no sirve
4. Después de 3 intentos, el mensaje sigue en la lista pero marcado

## 📱 Interfaz Mejorada

### Columnas de la Tabla:

1. **Fecha**: Cuándo llegó el mensaje
2. **Mensaje (con ALT)**: Contenido completo incluyendo teléfono real
3. **Remitente**: Nombre de quien envió
4. **Teléfono Real**: El número asociado (ya incluido en ALT)
5. **Intentos**: Contador visual 0/3, 1/3, 2/3, 3/3
6. **Acciones**: Botones ▶️ ✏️ 🗑️

### Colores de Estado:

- **Verde** (#25d366): Botón de procesar, teléfonos reales
- **Amarillo** (#ffc107): Intentos 1-2
- **Rojo** (#f15c6d): Intentos 3, botón eliminar
- **Gris**: Botones normales

## 🚫 Cambios Respecto al Sistema Anterior

### ❌ Removido:

- Procesamiento automático cada 5 minutos
- Botón "Procesar Mensajes" (lote)
- Vista de "Resultados de Procesamiento"
- Función `ProcessMessages(10)` del lote

### ✅ Agregado:

- Vista de mensajes sin procesar
- Botón "▶️" por cada mensaje
- Contenido con ALT ya visible
- Procesamiento uno por uno

## 🎓 Tips de Uso

1. **Revisa antes de procesar**: Verifica que el mensaje tenga sentido
2. **Edita si es necesario**: Mejor corregir antes que desperdiciar quota
3. **Elimina basura**: Limpia mensajes malos antes de procesarlos
4. **Gestiona tus keys**: Agrega 2-3 keys para tener 150 requests/día
5. **Monitorea intentos**: Si ves 2/3 o 3/3, investiga por qué falla

## 📈 Beneficios

- 🎯 **Precisión**: Solo procesas mensajes válidos
- 💰 **Eficiencia**: No desperdicias quota de API
- 🔍 **Visibilidad**: Ves exactamente qué va a procesar
- ⚡ **Velocidad**: Procesamiento inmediato al click
- 🛠️ **Control**: Total control sobre cada mensaje
