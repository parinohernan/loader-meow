@echo off
REM Script para aplicar la migración de perfilado de teléfonos
REM Agrega las columnas: nombre, perfil, confianza

echo ========================================
echo Aplicando migración de perfilado de teléfonos
echo ========================================
echo.

REM Cargar configuración de MySQL
call load-env.bat

echo Ejecutando migración SQL...
echo.

mysql -h %MYSQL_HOST% -P %MYSQL_PORT% -u %MYSQL_USER% -p%MYSQL_PASSWORD% %MYSQL_DATABASE% < migrations\add_phone_profiling_columns.sql

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo ✅ Migración aplicada exitosamente!
    echo ========================================
    echo.
    echo Nuevas columnas agregadas a phone_associations:
    echo   - nombre: Nombre del contacto
    echo   - perfil: Tipo de usuario (desconocido/loader/camionero^)
    echo   - confianza: Score de confiabilidad
    echo.
    echo El sistema ahora:
    echo   ✓ Filtra mensajes de camioneros buscando carga
    echo   ✓ Actualiza automáticamente el perfil de usuarios
    echo   ✓ Mantiene un score de confianza por contacto
    echo.
) else (
    echo.
    echo ========================================
    echo ❌ Error al aplicar la migración
    echo ========================================
    echo.
    echo Revisa los errores mostrados arriba.
    echo.
)

pause

