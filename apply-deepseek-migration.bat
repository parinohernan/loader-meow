@echo off
echo ============================================
echo Agregando DeepSeek como proveedor de IA
echo ============================================
echo.

REM Intentar encontrar MySQL en ubicaciones comunes
set MYSQL_PATH=
if exist "C:\xampp\mysql\bin\mysql.exe" set MYSQL_PATH=C:\xampp\mysql\bin\mysql.exe
if exist "C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe" set MYSQL_PATH=C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe
if exist "C:\Program Files\MariaDB 10.11\bin\mysql.exe" set MYSQL_PATH=C:\Program Files\MariaDB 10.11\bin\mysql.exe

if "%MYSQL_PATH%"=="" (
    echo ERROR: No se encontro MySQL/MariaDB en las ubicaciones comunes.
    echo.
    echo Por favor, ejecuta la migracion manualmente desde PHPMyAdmin:
    echo 1. Abre PHPMyAdmin
    echo 2. Selecciona la base de datos 'whatsapp_cargas'
    echo 3. Ve a la pestana SQL
    echo 4. Copia y pega el contenido de migrations/add_deepseek_provider.sql
    echo 5. Click en Ejecutar
    echo.
    pause
    exit /b 1
)

echo MySQL encontrado en: %MYSQL_PATH%
echo.

REM Ejecutar migración
"%MYSQL_PATH%" -u root -p whatsapp_cargas < migrations\add_deepseek_provider.sql

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ============================================
    echo DeepSeek agregado exitosamente!
    echo ============================================
    echo.
    echo Ahora puedes:
    echo 1. Abrir la app
    echo 2. Ir a "Configuracion IA"
    echo 3. Agregar una API key de DeepSeek
    echo.
) else (
    echo.
    echo ERROR: No se pudo ejecutar la migracion.
    echo Por favor, ejecutala manualmente desde PHPMyAdmin.
    echo.
)

pause

