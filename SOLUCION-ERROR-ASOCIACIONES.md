# 🔧 Solución: Error al cargar asociaciones

## 📋 Posibles Causas y Soluciones

### **Causa 1: Base de datos antigua sin las nuevas columnas**

**Síntoma:** Error SQL mencionando que la columna `sender_phone` o `sender_name` no existe.

**Solución:**

1. Cierra la aplicación completamente
2. Ejecuta `reset-database.bat` para resetear la base de datos
3. Vuelve a ejecutar `run-with-cgo.bat`
4. Escanea el QR de WhatsApp nuevamente
5. Los nuevos mensajes se guardarán con el esquema correcto

**Nota:** Esto eliminará todos los mensajes guardados anteriormente.

---

### **Causa 2: No hay mensajes en la base de datos aún**

**Síntoma:** La pestaña de asociaciones aparece vacía con el mensaje "No hay datos de asociaciones disponibles".

**Solución:**

- Esto es normal si acabas de instalar la aplicación
- Espera a recibir algunos mensajes en grupos
- Una vez que lleguen mensajes, la pestaña mostrará los remitentes

---

### **Causa 3: La aplicación no se reinició después de los cambios**

**Síntoma:** Error `window.go.main.App.GetSendersForAssociation is not a function`

**Solución:**

1. Cierra completamente la aplicación (incluyendo la ventana de consola)
2. Ejecuta nuevamente `run-with-cgo.bat` o `rebuild-dev.bat`
3. Espera a que compile completamente
4. Abre la aplicación y prueba la pestaña de asociaciones

---

### **Causa 4: Error de JavaScript en el frontend**

**Síntoma:** Error en la consola del navegador (F12) al hacer clic en la pestaña.

**Solución:**

1. Presiona F12 para abrir las herramientas de desarrollo
2. Ve a la pestaña "Console"
3. Copia el error completo
4. Revisa si hay algún problema con las llamadas a las funciones

---

## 🔍 Verificación Rápida

Para verificar que todo está funcionando:

1. **Abre la aplicación**
2. **Conecta WhatsApp** escaneando el QR
3. **Recibe algunos mensajes** en grupos
4. **Haz clic en la pestaña "🔗 Asociaciones"**
5. **Deberías ver** una lista de remitentes con sus datos

---

## 📊 Verificar la Base de Datos Manualmente

Si quieres verificar que las tablas existen correctamente:

1. Descarga **DB Browser for SQLite** (https://sqlitebrowser.org/)
2. Abre el archivo `store/messages.db`
3. Verifica que existan las siguientes tablas:
   - `messages` (con columnas `sender_phone` y `sender_name`)
   - `phone_associations` (con columnas `sender_phone`, `real_phone`, `display_name`)

---

## 🆘 Si nada funciona

Si después de intentar todas las soluciones anteriores el problema persiste:

1. Cierra la aplicación completamente
2. Elimina la carpeta `store/` manualmente
3. Elimina la carpeta `%USERPROFILE%\.wails\cache`
4. Ejecuta `rebuild-dev.bat`
5. Escanea el QR nuevamente

Esto hará una **limpieza completa** y recreará todo desde cero.

---

## ✅ Cambios Recientes

Los siguientes cambios se implementaron para mejorar el manejo de errores:

- ✅ La función ahora retorna un array vacío en lugar de error si la tabla no existe
- ✅ Se agregó `COALESCE` para manejar valores NULL
- ✅ Se agregó logging detallado para debug
- ✅ Se mejoró el manejo de errores en el escaneo de resultados
- ✅ **SOLUCIONADO:** Conversión de timestamp de string a time.Time (error "unsupported Scan")

---

## 📝 Logs Útiles

Al abrir la pestaña de asociaciones, deberías ver en la consola:

```
[WhatsApp INFO] 📋 Obtenidos X remitentes para asociaciones
```

Si ves un error SQL, copia el mensaje completo para identificar el problema exacto.
