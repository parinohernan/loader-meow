@echo off
REM Script para cargar variables de entorno desde mysql-config.env

if not exist mysql-config.env (
    echo ERROR: Archivo mysql-config.env no encontrado
    echo Ejecuta primero setup-mysql.bat
    pause
    exit /b 1
)

echo ========================================
echo   CARGANDO CONFIGURACION DE MYSQL
echo ========================================
echo.

REM Leer y establecer cada variable
for /f "usebackq tokens=1* delims==" %%a in ("mysql-config.env") do (
    set "line=%%a"
    setlocal enabledelayedexpansion
    if not "!line:~0,1!"=="#" if not "%%a"=="" (
        endlocal
        set "%%a=%%b"
        echo [OK] %%a = %%b
    ) else (
        endlocal
    )
)

echo.
echo ========================================
echo   VERIFICANDO VARIABLES
echo ========================================
echo DB_HOST = %DB_HOST%
echo DB_PORT = %DB_PORT%
echo DB_USER = %DB_USER%
echo DB_NAME = %DB_NAME%
echo.

if "%DB_HOST%"=="" (
    echo ERROR: No se cargaron las variables correctamente
    pause
    exit /b 1
)

echo ========================================
echo   EJECUTANDO APLICACION
echo ========================================
echo.

REM Ejecutar la aplicación con las variables de entorno
wails dev
