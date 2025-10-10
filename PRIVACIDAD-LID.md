# 🔒 Manejo de LIDs (Lidded IDs) - Usuarios con Privacidad

## 🎯 ¿Qué es un LID?

**LID** (Lidded ID) = Identificador Anónimo de WhatsApp

Cuando un usuario configura en WhatsApp:

- **Configuración** → **Privacidad** → **Quién puede ver mi número de teléfono** → **Nadie** o **Mis contactos**

WhatsApp:

- ✅ Oculta su número de teléfono
- ✅ Usa un LID anónimo (ej: `21496412029002@lid`)
- ❌ **NO revela** el número real en ningún lado

## 📊 Ejemplo Real

```
Usuario: Hernan Parino
Configuración: Privacidad activada (número oculto)

En GetGroupInfo():
  JID: 21496412029002@lid  ← Mismo que LID
  LID: 21496412029002@lid  ← No hay número real

PushName: "Hernan Parino"  ← Único dato confiable
```

## ⚠️ Limitación Importante

**ES IMPOSIBLE** obtener el número de teléfono real cuando:

- JID = LID (ambos `@lid`)
- Usuario tiene privacidad activada
- WhatsApp protege intencionalmente esta información

## ✅ Soluciones Prácticas

### Opción 1: Usar PushName como Identificador (RECOMENDADO)

```go
// En tu base de datos, el campo "sender" será:
// - Número de teléfono si está disponible: "573001234567"
// - PushName si tiene LID: "Hernan Parino"

sender = "Hernan Parino"
```

**Ventajas:**

- ✅ Identificador único y legible
- ✅ Respeta la privacidad del usuario
- ✅ Es lo que WhatsApp usa oficialmente

**Desventajas:**

- ⚠️ El PushName puede cambiar si el usuario lo modifica
- ⚠️ Dos personas con el mismo nombre serían el mismo "sender"

### Opción 2: Usar LID como Identificador Único

```go
sender = "21496412029002"  // El LID
```

**Ventajas:**

- ✅ Identificador único garantizado
- ✅ No cambia nunca

**Desventajas:**

- ❌ No es legible para humanos
- ❌ No sabes quién es sin ver el PushName

### Opción 3: Combinar PushName + LID (MEJOR)

```go
// Guardar ambos en la BD
sender_id = "21496412029002"         // Campo: sender_lid
sender_name = "Hernan Parino"        // Campo: sender_name o usar sender
```

**Ventajas:**

- ✅ Identificador único (LID)
- ✅ Legible (PushName)
- ✅ Lo mejor de ambos mundos

## 🔧 Implementación Recomendada

### Modificar la Base de Datos

```sql
ALTER TABLE messages ADD COLUMN sender_name TEXT;
ALTER TABLE messages ADD COLUMN sender_lid TEXT;

-- Índice para búsqueda
CREATE INDEX idx_sender_lid ON messages(sender_lid);
```

Luego:

- `sender` = PushName (para lectura humana)
- `sender_lid` = LID real (para identificación única)

### En el Código

```go
// Al guardar mensaje
if senderJID.Server == "lid" {
    senderPhone = msg.Info.PushName  // Nombre legible
    senderLID = senderJID.User       // ID único
} else {
    senderPhone = senderJID.User     // Número real
    senderLID = ""                   // No tiene LID
}
```

## 🎯 Mi Recomendación

Dado que **NO puedes obtener el número real** por limitaciones de WhatsApp:

### Para tu caso:

```go
// Usar PushName directamente como sender
sender = "Hernan Parino"
```

Esto es:

- ✅ Simple
- ✅ Legible
- ✅ Funcional
- ✅ Respeta la privacidad

### Para Detección de Duplicados:

```sql
-- Los duplicados se detectarán correctamente:
WHERE sender = "Hernan Parino"
AND content = "mensaje de prueba"
AND timestamp >= datetime(?, '-48 hours')
```

Si "Hernan Parino" envía el mismo mensaje 3 veces en 2 días, solo se guardará 1 vez.

## 📱 Casos en tu Base de Datos

```sql
-- Usuario SIN privacidad
sender: "573001234567"
content: "Hola"

-- Usuario CON privacidad (LID)
sender: "Hernan Parino"
content: "Muño"

-- Usuario CON privacidad SIN PushName (raro)
sender: "21496412029002"
content: "Mensaje"
```

## 🤔 ¿Y si Dos Personas Tienen el Mismo Nombre?

Es muy raro, pero podría pasar. Opciones:

1. **Ignorarlo**: En la práctica es extremadamente raro
2. **Agregar LID al final**: "Hernan Parino (LID:2149...)"
3. **Campo separado**: Usar `sender_lid` adicional

## ✅ Conclusión

**No puedes obtener el número real** cuando el usuario tiene privacidad activada. Esto es **intencional** de WhatsApp para proteger la privacidad.

La mejor solución es usar **PushName como identificador**, que es lo que ya implementé.

---

**Resumen:** WhatsApp protege intencionalmente los números de usuarios con privacidad activada. Usa PushName como identificador único. 🔒
