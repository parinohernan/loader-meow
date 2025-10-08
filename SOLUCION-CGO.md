# 🔧 Solución al Error de CGO con SQLite3

## El Problema

```
Error: Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work.
```

Este error ocurre porque:

1. `go-sqlite3` necesita **CGO** (para compilar código C)
2. Wails deshabilita CGO por defecto en Windows
3. Necesitas **GCC** instalado en tu sistema

## ✅ Solución Paso a Paso

### 1. Instalar GCC

**OPCIÓN RECOMENDADA - TDM-GCC (Más fácil para Windows):**

1. Ve a: https://jmeubank.github.io/tdm-gcc/download/
2. Descarga el instalador (tdm64-gcc-XX.X.X.exe)
3. Ejecuta el instalador
4. Durante la instalación:
   - Selecciona "Create" (instalación nueva)
   - Marca la opción **"Add to PATH"**
   - Instala en la ubicación predeterminada
5. **Reinicia tu terminal PowerShell**

**Verificar instalación:**

```powershell
gcc --version
```

Deberías ver algo como:

```
gcc.exe (tdm64-1) 10.3.0
```

### 2. Configurar CGO

Ejecuta el script de configuración:

```powershell
.\setup-cgo.bat
```

Este script:

- ✅ Verifica que GCC esté instalado
- ✅ Configura `CGO_ENABLED=1`
- ✅ Descarga dependencias
- ✅ Limpia el cache de Wails

### 3. Ejecutar la Aplicación

**IMPORTANTE:** Debes usar los scripts especiales:

```powershell
# Modo desarrollo
.\run-with-cgo.bat

# O compilar para producción
.\build-with-cgo.bat
```

**NO uses** `wails dev` directamente, siempre usa `run-with-cgo.bat`

## 🎯 Scripts Disponibles

### `setup-cgo.bat`

Configura el entorno y verifica que todo esté listo

### `run-with-cgo.bat`

Ejecuta la app en modo desarrollo con CGO habilitado

### `build-with-cgo.bat`

Compila la app para producción con CGO habilitado

## 🔍 Verificación Manual

Si prefieres hacerlo manualmente:

```powershell
# 1. Verificar GCC
gcc --version

# 2. Habilitar CGO y ejecutar
$env:CGO_ENABLED=1
wails dev
```

## 🐛 Problemas Comunes

### "gcc: command not found"

- GCC no está en el PATH
- Solución: Reinstala TDM-GCC y marca "Add to PATH"
- O agrega manualmente a PATH:
  ```
  C:\TDM-GCC-64\bin
  ```

### "undefined reference to..."

- Versión incorrecta de GCC
- Solución: Usa TDM-GCC 10.3.0 o superior

### El error persiste

1. Cierra todas las terminales
2. Ejecuta `setup-cgo.bat`
3. Abre una nueva terminal PowerShell
4. Ejecuta `run-with-cgo.bat`

## 📚 Información Técnica

### ¿Por qué CGO?

`go-sqlite3` es un driver de SQLite escrito en C. Para que Go pueda usarlo, necesita CGO (C Go) que permite llamar a código C desde Go.

### ¿Por qué GCC?

GCC (GNU Compiler Collection) es el compilador de C que CGO usa para compilar el código de SQLite.

### Variables de Entorno

- `CGO_ENABLED=1`: Habilita CGO
- `PATH`: Debe incluir la ruta a `gcc.exe`

## ✅ Checklist

- [ ] GCC instalado (`gcc --version` funciona)
- [ ] Ejecutado `setup-cgo.bat`
- [ ] Usando `run-with-cgo.bat` (NO `wails dev`)
- [ ] Terminal reiniciada después de instalar GCC

## 🎉 Resultado Esperado

Cuando todo funciona correctamente, verás:

```
Using DevServer URL: http://localhost:34115
Watching directory: C:\dev\go\loader-meow
INF | Aplicación iniciada
INF | Inicializando WhatsApp...
INF | Conectando a WhatsApp...
```

Y la aplicación se abrirá mostrando el botón "Conectar WhatsApp".

---

**Si sigues teniendo problemas, verifica:**

1. ¿Reiniciaste la terminal después de instalar GCC?
2. ¿Estás usando `run-with-cgo.bat` y no `wails dev`?
3. ¿`gcc --version` funciona en la terminal?

