@echo off
echo ========================================
echo   Building Loader Meow
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

echo Compilando aplicacion...
echo.
wails build
echo.

if errorlevel 1 (
    echo [ERROR] Fallo la compilacion
    pause
    exit /b 1
)

echo ========================================
echo   Compilacion Exitosa!
echo ========================================
echo.
echo El ejecutable esta en: build\bin\loader-meow.exe
echo.
pause


