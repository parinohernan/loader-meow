@echo off
echo ========================================
echo   Loader Meow - WhatsApp Desktop
echo ========================================
echo.

REM Verificar GCC
gcc --version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] GCC no esta instalado!
    echo.
    echo Ejecuta primero: setup-cgo.bat
    echo.
    pause
    exit /b 1
)

REM Habilitar CGO
set CGO_ENABLED=1
echo [OK] CGO_ENABLED=1
echo.

echo Iniciando aplicacion en modo desarrollo...
echo.
wails dev


