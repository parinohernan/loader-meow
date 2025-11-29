# 📘 Integración de Facebook

## 🎯 Funcionalidad

Este sistema permite obtener publicaciones de grupos de Facebook y procesarlas con IA, igual que los mensajes de WhatsApp.

## 🔧 Configuración Inicial

### 1. Obtener Access Token de Facebook

1. Ve a [Facebook Developers](https://developers.facebook.com/)
2. Crea una nueva App o usa una existente
3. Obtén un Access Token con los siguientes permisos:
   - `groups_read` - Para leer publicaciones de grupos
   - `public_profile` - Perfil básico

**Opciones para obtener el token:**
- Usa [Graph API Explorer](https://developers.facebook.com/tools/explorer/)
- O genera un token de larga duración desde tu app

### 2. Configurar Token

**Opción A: Variable de entorno**
```bash
export FACEBOOK_ACCESS_TOKEN=tu_token_aqui
```

**Opción B: Archivo de configuración**
1. Copia `facebook-config.env.example` a `facebook-config.env`
2. Edita y agrega tu token:
```
FACEBOOK_ACCESS_TOKEN=tu_token_aqui
```

## 📋 Uso desde la Aplicación

### Inicializar Servicio de Facebook

Primero debes inicializar el servicio (después de inicializar WhatsApp):

```javascript
// Desde el frontend
await window.go.main.App.InitFacebook();
```

### Agregar Grupos

Puedes agregar múltiples grupos de Facebook:

```javascript
// Agregar un grupo
await window.go.main.App.AddFacebookGroup(
    "1234567890123456",           // Group ID
    "Grupo de Cargas",            // Nombre del grupo
    ""                            // Token personalizado (opcional, vacío usa el token por defecto)
);
```

**Cómo obtener el Group ID:**
1. Ve al grupo en Facebook
2. El ID está en la URL: `https://www.facebook.com/groups/1234567890123456/`
3. O usa la Graph API para listar tus grupos

### Listar Grupos

```javascript
const groups = await window.go.main.App.GetFacebookGroups();
console.log(groups);
```

### Obtener Publicaciones

**De un grupo específico:**
```javascript
await window.go.main.App.FetchFacebookGroupPosts(
    "1234567890123456",  // Group ID
    50                   // Límite de publicaciones
);
```

**De todos los grupos habilitados:**
```javascript
const results = await window.go.main.App.FetchAllFacebookGroupsPosts(50);
// results es un mapa: { groupID: "success" o "error message" }
```

### Gestionar Grupos

**Habilitar/Deshabilitar:**
```javascript
await window.go.main.App.ToggleFacebookGroup(
    "1234567890123456",  // Group ID
    true                 // true = habilitado, false = deshabilitado
);
```

**Eliminar grupo:**
```javascript
await window.go.main.App.RemoveFacebookGroup("1234567890123456");
```

**Actualizar token:**
```javascript
await window.go.main.App.UpdateFacebookAccessToken("nuevo_token");
```

## 🔄 Flujo de Procesamiento

1. **Obtener publicaciones**: Las publicaciones se almacenan en la misma base de datos que los mensajes de WhatsApp
2. **Procesamiento automático**: Las publicaciones aparecen como mensajes no procesados
3. **Procesar con IA**: Usa las mismas funciones de procesamiento que WhatsApp:
   ```javascript
   await window.go.main.App.ProcessMessages(100);
   ```

## 📊 Estructura de Datos

Las publicaciones de Facebook se almacenan como mensajes con:
- `chat_jid`: `facebook_group_{groupID}`
- `sender_phone`: ID del usuario de Facebook
- `sender_name`: Nombre del usuario
- `content`: Mensaje de la publicación
- `processed`: `false` por defecto (listo para procesar)

## 🔐 Seguridad

- Los tokens se almacenan en la base de datos
- Cada grupo puede tener su propio token (opcional)
- Los tokens no se exponen en el frontend

## ⚠️ Limitaciones de Facebook API

- **Rate Limits**: Facebook limita las solicitudes. No hagas demasiadas llamadas seguidas
- **Permisos**: Necesitas ser miembro del grupo o tener permisos de administrador
- **Tokens**: Los tokens de usuario expiran. Considera usar tokens de larga duración o de página

## 🐛 Solución de Problemas

### Error: "Facebook service not initialized"
- Asegúrate de llamar `InitFacebook()` después de `InitWhatsApp()`

### Error: "Facebook API error"
- Verifica que el token sea válido
- Verifica que tengas permisos para leer el grupo
- Verifica que el Group ID sea correcto

### No se obtienen publicaciones
- Verifica que el grupo esté habilitado (`enabled: true`)
- Verifica los permisos del token
- Revisa los logs para ver errores específicos

## 📝 Notas

- Las publicaciones se almacenan con el mismo formato que los mensajes de WhatsApp
- El sistema de detección de duplicados también aplica a las publicaciones de Facebook
- Puedes procesar publicaciones de Facebook junto con mensajes de WhatsApp

