@echo off
echo ========================================
echo   Configuracion CGO para SQLite
echo ========================================
echo.

echo Verificando GCC...
gcc --version >nul 2>&1
if errorlevel 1 (
    echo.
    echo [ERROR] GCC no esta instalado!
    echo.
    echo Por favor instala GCC siguiendo estas opciones:
    echo.
    echo OPCION 1 - TDM-GCC (Recomendado para Windows):
    echo   1. Descarga: https://jmeubank.github.io/tdm-gcc/download/
    echo   2. Instala TDM-GCC (marca "Add to PATH")
    echo   3. Reinicia la terminal
    echo.
    echo OPCION 2 - MinGW-w64:
    echo   1. Descarga: https://www.mingw-w64.org/downloads/
    echo   2. Instala y agrega al PATH
    echo.
    echo Despues de instalar GCC, ejecuta este script nuevamente.
    pause
    exit /b 1
)

echo [OK] GCC encontrado!
gcc --version
echo.

echo Configurando variables de entorno para CGO...
set CGO_ENABLED=1
echo [OK] CGO_ENABLED=1
echo.

echo Descargando dependencias de Go...
go mod download
if errorlevel 1 (
    echo [ERROR] Fallo al descargar dependencias
    pause
    exit /b 1
)
echo.

echo Limpiando cache de Wails...
wails clean
echo.

echo ========================================
echo   Configuracion Completa!
echo ========================================
echo.
echo IMPORTANTE: Para ejecutar la aplicacion, usa:
echo.
echo   run-with-cgo.bat
echo.
echo O manualmente con:
echo   set CGO_ENABLED=1
echo   wails dev
echo.
pause


