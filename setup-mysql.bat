@echo off
echo ========================================
echo    CONFIGURACION DE MYSQL PARA LOADER-MEOW
echo ========================================
echo.

REM Verificar si MySQL está instalado
mysql --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: MySQL no está instalado o no está en el PATH
    echo.
    echo Por favor instala MySQL desde: https://dev.mysql.com/downloads/mysql/
    echo O instala XAMPP/WAMP que incluye MySQL
    pause
    exit /b 1
)

echo MySQL detectado correctamente
echo.

REM Leer configuración
set /p DB_HOST="Host de MySQL (default: localhost): "
if "%DB_HOST%"=="" set DB_HOST=localhost

set /p DB_PORT="Puerto de MySQL (default: 3306): "
if "%DB_PORT%"=="" set DB_PORT=3306

set /p DB_USER="Usuario de MySQL (default: root): "
if "%DB_USER%"=="" set DB_USER=root

set /p DB_PASSWORD="Contraseña de MySQL: "

set /p DB_NAME="Nombre de la base de datos (default: whatsapp_loader): "
if "%DB_NAME%"=="" set DB_NAME=whatsapp_loader

echo.
echo ========================================
echo Creando base de datos y usuario...
echo ========================================

REM Crear base de datos
mysql -h %DB_HOST% -P %DB_PORT% -u %DB_USER% -p%DB_PASSWORD% -e "CREATE DATABASE IF NOT EXISTS %DB_NAME% CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

if errorlevel 1 (
    echo ERROR: No se pudo crear la base de datos
    pause
    exit /b 1
)

echo Base de datos '%DB_NAME%' creada exitosamente
echo.

REM Crear archivo de configuración
echo # Configuración de Base de Datos MySQL > mysql-config.env
echo DB_HOST=%DB_HOST% >> mysql-config.env
echo DB_PORT=%DB_PORT% >> mysql-config.env
echo DB_USER=%DB_USER% >> mysql-config.env
echo DB_PASSWORD=%DB_PASSWORD% >> mysql-config.env
echo DB_NAME=%DB_NAME% >> mysql-config.env
echo DB_CHARSET=utf8mb4 >> mysql-config.env

echo Archivo de configuración 'mysql-config.env' creado
echo.

echo ========================================
echo CONFIGURACION COMPLETADA
echo ========================================
echo.
echo Para usar la aplicación:
echo 1. Copia mysql-config.env a .env (opcional)
echo 2. Ejecuta: wails dev
echo.
echo La aplicación se conectará automáticamente a MySQL
echo y creará las tablas necesarias.
echo.
pause
