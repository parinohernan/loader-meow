@echo off
echo ========================================
echo   Inicializando Repositorio Git
echo ========================================
echo.

REM Verificar si ya existe un repositorio
if exist ".git" (
    echo [!] Ya existe un repositorio Git en este directorio.
    echo.
    choice /C SN /M "Deseas reiniciar el repositorio"
    if errorlevel 2 goto :end
    echo.
    echo Eliminando repositorio existente...
    rmdir /s /q .git
)

echo [1/5] Inicializando repositorio Git...
git init
if errorlevel 1 (
    echo [ERROR] Fallo al inicializar Git. Verifica que Git este instalado.
    pause
    exit /b 1
)
echo.

echo [2/5] Configurando rama principal como 'main'...
git branch -M main
echo.

echo [3/5] Agregando archivos al staging...
git add .
echo.

echo [4/5] Creando commit inicial...
git commit -m "Initial commit: Loader Meow - WhatsApp Desktop Client"
if errorlevel 1 (
    echo [ERROR] Fallo al crear el commit.
    echo Asegurate de tener Git configurado:
    echo   git config --global user.name "Tu Nombre"
    echo   git config --global user.email "tu@email.com"
    pause
    exit /b 1
)
echo.

echo ========================================
echo   Repositorio Git Creado Exitosamente!
echo ========================================
echo.
echo Proximos pasos:
echo.
echo 1. Crea un nuevo repositorio en GitHub
echo    https://github.com/new
echo.
echo 2. Conecta tu repositorio local con GitHub:
echo    git remote add origin https://github.com/TU-USUARIO/loader-meow.git
echo.
echo 3. Sube tu codigo:
echo    git push -u origin main
echo.
echo Comandos utiles:
echo   git status          - Ver estado del repositorio
echo   git log             - Ver historial de commits
echo   git branch          - Ver ramas
echo.

:end
pause

