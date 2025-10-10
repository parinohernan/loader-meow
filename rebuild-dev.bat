@echo off
echo ============================================
echo Reconstruyendo aplicacion Loader Meow
echo ============================================
echo.

echo [1/3] Configurando CGO...
set CGO_ENABLED=1

echo [2/3] Limpiando cache de Wails...
if exist "%USERPROFILE%\.wails\cache" (
    echo Limpiando cache de Wails...
    rmdir /s /q "%USERPROFILE%\.wails\cache" 2>nul
)

echo [3/3] Iniciando en modo desarrollo...
echo.
wails dev

pause

