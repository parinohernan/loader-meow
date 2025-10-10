@echo off
echo ============================================
echo RESETEO DE BASE DE DATOS - Loader Meow
echo ============================================
echo.
echo ADVERTENCIA: Esto eliminara TODOS los mensajes almacenados.
echo.
pause
echo.

echo [1/2] Eliminando base de datos antigua...
if exist "store\messages.db" (
    del /f /q "store\messages.db"
    echo Base de datos eliminada correctamente.
) else (
    echo No se encontro base de datos antigua.
)

echo.
echo [2/2] La nueva base de datos se creara automaticamente al iniciar la aplicacion.
echo.
echo ============================================
echo Reseteo completado
echo ============================================
echo.
echo Ahora puedes ejecutar run-with-cgo.bat para iniciar la aplicacion.
echo.
pause

