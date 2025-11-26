# Sistema de Perfilado de Usuarios

## 📋 Resumen

Este documento describe el sistema de perfilado de usuarios implementado para distinguir entre **loaders** (empresas/personas que ofrecen cargas) y **camioneros** (personas que buscan cargas para transportar).

## 🎯 Objetivos

1. **Filtrar mensajes de camioneros** que buscan carga (no son ofertas de carga)
2. **Perfilar automáticamente a los usuarios** según su comportamiento
3. **Mantener un score de confianza** para cada contacto

## 🔄 Cómo Funciona

### 1. Filtrado de Mensajes de Camioneros

El prompt de IA fue mejorado para identificar y **rechazar** mensajes de camioneros que buscan carga:

**Ejemplos de mensajes que NO se procesan:**

- ❌ "Busco carga para camión semi, zona Buenos Aires"
- ❌ "Camión disponible, tolva 30tn, busco flete"
- ❌ "Ofrezco servicio de transporte, semi cerealero"
- ❌ "Camionero disponible, tengo chasis y acoplado"

**Ejemplos de mensajes que SÍ se procesan:**

- ✅ "Necesito transportar 25 toneladas de soja de Rosario a Buenos Aires"
- ✅ "Tengo carga de 15tn de trigo, busco camión"
- ✅ "Hay carga disponible: maíz de Córdoba a Santa Fe"

Cuando la IA detecta un mensaje de camionero, devuelve un **array vacío []**.

### 2. Sistema de Confianza

Cada contacto en la tabla `phone_associations` tiene un score de confianza:

- **+1** cada vez que envía un mensaje con carga válida (es un loader)
- **-1** cada vez que envía un mensaje de camionero buscando carga

```
Score positivo → Loader (ofrece cargas)
Score negativo → Camionero (busca cargas)
Score 0 → Desconocido (aún no clasificado)
```

### 3. Perfilado Automático

Basado en el score de confianza, el sistema actualiza automáticamente el perfil:

| Score | Perfil        | Descripción                   |
| ----- | ------------- | ----------------------------- |
| > 0   | `loader`      | Envía ofertas de carga        |
| < 0   | `camionero`   | Busca cargas para transportar |
| = 0   | `desconocido` | No hay suficiente información |

## 🗄️ Estructura de Base de Datos

### Nuevas Columnas en `phone_associations`

```sql
nombre VARCHAR(255) DEFAULT ''
  -- Nombre del contacto (puede editarse manualmente)

perfil ENUM('desconocido', 'loader', 'camionero') DEFAULT 'desconocido'
  -- Perfil automático basado en comportamiento

confianza INT DEFAULT 0
  -- Score de confiabilidad: +1 por carga válida, -1 por mensaje de camionero
```

### Índices Creados

- `idx_perfil` - Para búsquedas por tipo de perfil
- `idx_confianza` - Para ordenar por confianza

## 📝 Archivos Modificados

### 1. `contecto_funcionalidad_ia.md`

- ✅ Agregado filtrado de mensajes de camioneros
- ✅ Ejemplos claros de qué procesar y qué no
- ✅ Instrucciones para identificar camioneros
- ✅ Excepciones de ciudades argentinas: Chilecito (La Rioja), Concepción del Uruguay (Entre Ríos)

### 2. `migrations/add_phone_profiling_columns.sql`

- ✅ Script SQL para agregar las 3 nuevas columnas
- ✅ Creación de índices para optimizar búsquedas

### 3. `whatsapp_service.go`

- ✅ Actualizada tabla `phone_associations` con nuevas columnas
- ✅ Método `UpdatePhoneProfiling()` para actualizar confianza y perfil
- ✅ Método `UpdatePhoneName()` para actualizar nombres
- ✅ Método `UpdatePhoneProfiling()` en MessageStore

### 4. `message_processor.go`

- ✅ Actualiza confianza cuando se procesa una carga exitosamente (+1)
- ✅ Actualiza confianza cuando el mensaje es vacío/camionero (-1)
- ✅ Logs informativos del perfilado

## 🚀 Cómo Aplicar los Cambios

### Paso 1: Aplicar la migración SQL

```bash
apply-phone-profiling-migration.bat
```

Este script:

1. Carga la configuración de MySQL
2. Ejecuta la migración
3. Agrega las 3 columnas nuevas
4. Crea los índices

### Paso 2: Recompilar la aplicación

```bash
rebuild-dev.bat
```

o

```bash
build.bat
```

### Paso 3: Ejecutar la aplicación

```bash
run-dev.bat
```

## 📊 Beneficios

### 1. Filtrado Automático

- No se crearán cargas falsas de camioneros buscando trabajo
- Solo se procesan ofertas reales de carga

### 2. Conocimiento del Usuario

- Sabes quién es loader (confiable para cargas)
- Sabes quién es camionero (buscando trabajo)
- Puedes priorizar mensajes de loaders confiables

### 3. Score de Confianza

- Identifica usuarios más activos y confiables
- Detecta cambios de comportamiento
- Permite filtrar por nivel de confianza

### 4. Gestión Manual

- Campo `nombre` editable para agregar nombres reales
- Visualización del perfil automático
- Score visible para análisis

## 🔍 Monitoreo

### En los Logs

Cuando se procesa un mensaje, verás:

```
📈 Perfil actualizado: +5493462677283 (+1 confianza, loader confirmado)
```

O cuando se rechaza:

```
📉 Perfil actualizado: +5493462677283 (-1 confianza, posible camionero)
```

### Consultas SQL Útiles

**Ver perfiles actualizados:**

```sql
SELECT real_phone, display_name, nombre, perfil, confianza
FROM phone_associations
ORDER BY confianza DESC;
```

**Ver solo loaders:**

```sql
SELECT real_phone, display_name, confianza
FROM phone_associations
WHERE perfil = 'loader'
ORDER BY confianza DESC;
```

**Ver solo camioneros:**

```sql
SELECT real_phone, display_name, confianza
FROM phone_associations
WHERE perfil = 'camionero'
ORDER BY confianza ASC;
```

**Estadísticas de perfiles:**

```sql
SELECT perfil, COUNT(*) as cantidad, AVG(confianza) as confianza_promedio
FROM phone_associations
GROUP BY perfil;
```

## 🎓 Ejemplo de Uso

### Escenario 1: Usuario Nuevo Envía Carga Válida

1. Usuario envía: "Tengo 20tn de soja de Rosario a Buenos Aires"
2. IA procesa y genera JSON válido
3. Sistema sube a Supabase ✅
4. **Confianza: 0 → +1**
5. **Perfil: desconocido → loader**

### Escenario 2: Usuario Busca Carga

1. Usuario envía: "Busco carga para mi camión semi, zona CABA"
2. IA detecta que es camionero buscando carga
3. IA devuelve array vacío: `[]`
4. Sistema NO crea carga ✅
5. **Confianza: 0 → -1**
6. **Perfil: desconocido → camionero**

### Escenario 3: Usuario Mixto

1. Usuario envía 3 cargas válidas → **Confianza: +3** (loader)
2. Usuario envía 1 mensaje buscando carga → **Confianza: +2** (sigue siendo loader)
3. Usuario envía 5 mensajes buscando carga → **Confianza: -3** (ahora es camionero)

## 💡 Próximas Mejoras Sugeridas

### Interfaz de Usuario

- Mostrar perfil y confianza en la lista de contactos
- Filtrar contactos por perfil (loader/camionero)
- Editar nombre desde la UI
- Visualizar historial de confianza

### Funcionalidades Adicionales

- Configurar umbral de confianza mínimo para procesar
- Alertas cuando un loader cambia a camionero
- Estadísticas de perfiles en dashboard
- Exportar lista de loaders confiables

### Optimizaciones

- Cache de perfiles en memoria
- Actualización asíncrona de perfiles
- Logs más detallados por perfil

## 📞 Soporte

Si tienes dudas o problemas:

1. Revisa los logs de la aplicación
2. Verifica que la migración se aplicó correctamente
3. Consulta las estadísticas SQL para validar el funcionamiento

---

**Fecha de implementación:** 17 de octubre de 2025  
**Versión:** 1.0
