@echo off
echo ========================================
echo   Loader Meow - Setup Script
echo ========================================
echo.

echo [1/3] Downloading Go dependencies...
go mod download
if errorlevel 1 (
    echo ERROR: Failed to download dependencies
    pause
    exit /b 1
)
echo.

echo [2/3] Tidying go.mod...
go mod tidy
if errorlevel 1 (
    echo ERROR: Failed to tidy modules
    pause
    exit /b 1
)
echo.

echo [3/3] Checking environment...
echo.
echo Please make sure you have:
echo   1. Wails CLI installed: go install github.com/wailsapp/wails/v2/cmd/wails@latest
echo   2. WebView2 Runtime installed (Windows)
echo   3. GEMINI_API_KEY environment variable set
echo.

echo Setup completed successfully!
echo.
echo Next steps:
echo   1. Set your GEMINI_API_KEY: $env:GEMINI_API_KEY="your_key_here"
echo   2. Run in dev mode: wails dev
echo   3. Or build for production: wails build
echo.
pause


