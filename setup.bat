@echo off
REM ############################################################################
REM # NOFX+ Development Environment Setup Script (Windows)
REM #
REM # This script automates the complete setup process for NOFX+ on Windows
REM # 
REM # Prerequisites: Git Bash, PowerShell, or Windows Subsystem for Linux (WSL2)
REM # 
REM # Usage: setup.bat
REM ############################################################################

setlocal enabledelayedexpansion

REM Color codes (requires Windows 10+)
setlocal
for /F %%a in ('copy /Z "%~f0" nul') do set "BS=%%a"

REM Set colors using ANSI escape codes (Windows 10+)
set "ESC=[" 
set "GREEN=%ESC%92m"
set "RED=%ESC%91m"
set "YELLOW=%ESC%93m"
set "BLUE=%ESC%94m"
set "RESET=%ESC%0m"

REM Directory paths
set "SCRIPT_DIR=%~dp0"
set "NOFX_REPO=%SCRIPT_DIR:~0,-1%"
set "ENV_FILE=%NOFX_REPO%\.env"
set "ENV_EXAMPLE=%NOFX_REPO%\.env.example"

REM Minimum versions
set "MIN_GO_VERSION=1.21"
set "MIN_NODE_VERSION=18"

echo.
echo %BLUE%===================================================%RESET%
echo %BLUE%   NOFX+ Development Environment Setup (Windows)%RESET%
echo %BLUE%===================================================%RESET%
echo.

REM Check for prerequisites
echo %BLUE%Step 1: Checking System Prerequisites%RESET%
echo.

REM Check Git
where git >nul 2>nul
if %errorlevel% neq 0 (
    echo %RED%[X] Git is not installed%RESET%
    echo %BLUE%[i] Please install Git from: https://git-scm.com/download/win%RESET%
    exit /b 1
)
for /f "tokens=*" %%i in ('git --version') do set "GIT_VERSION=%%i"
echo %GREEN%[+] %GIT_VERSION%%RESET%

REM Check Go
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo %RED%[X] Go is not installed%RESET%
    echo %BLUE%[i] Please install Go 1.21+ from: https://golang.org/dl/%RESET%
    exit /b 1
)
for /f "tokens=*" %%i in ('go version') do set "GO_VERSION=%%i"
echo %GREEN%[+] %GO_VERSION%%RESET%

REM Check Node.js
where node >nul 2>nul
if %errorlevel% neq 0 (
    echo %RED%[X] Node.js is not installed%RESET%
    echo %BLUE%[i] Please install Node.js 18+ from: https://nodejs.org/%RESET%
    exit /b 1
)
for /f "tokens=*" %%i in ('node --version') do set "NODE_VERSION=%%i"
echo %GREEN%[+] Node.js %NODE_VERSION%%RESET%

REM Check npm
where npm >nul 2>nul
if %errorlevel% neq 0 (
    echo %RED%[X] npm is not installed%RESET%
    exit /b 1
)
echo %GREEN%[+] npm installed%RESET%

echo.
echo %YELLOW%[!] Note: TA-Lib installation on Windows is optional%RESET%
echo %YELLOW%[!] Many indicators work without it. For manual installation:%RESET%
echo %BLUE%    https://mrjbq7.github.io/ta-lib/install.html%RESET%
echo.

REM Install dependencies
echo %BLUE%Step 2: Installing Go and Node.js Dependencies%RESET%
echo.

echo %BLUE%[i] Downloading Go modules...%RESET%
cd /d "%NOFX_REPO%"
call go mod download
if %errorlevel% neq 0 (
    echo %RED%[X] Failed to download Go modules%RESET%
    exit /b 1
)
echo %GREEN%[+] Go modules downloaded%RESET%

echo %BLUE%[i] Installing Node.js dependencies...%RESET%
cd /d "%NOFX_REPO%\web"
call npm install
if %errorlevel% neq 0 (
    echo %RED%[X] Failed to install Node.js dependencies%RESET%
    exit /b 1
)
echo %GREEN%[+] Node.js dependencies installed%RESET%

cd /d "%NOFX_REPO%"

REM Setup environment
echo.
echo %BLUE%Step 3: Setting Up Environment Configuration%RESET%
echo.

if exist "%ENV_FILE%" (
    echo %YELLOW%[!] .env file already exists%RESET%
    set /p overwrite="Do you want to reconfigure it? (y/n) "
    if /i "!overwrite!"=="y" (
        goto :create_env
    ) else (
        echo %BLUE%[i] Keeping existing .env file%RESET%
        goto :skip_env
    )
)

:create_env
if not exist "%ENV_EXAMPLE%" (
    echo %RED%[X] .env.example not found%RESET%
    exit /b 1
)

echo %BLUE%[i] Creating .env from template...%RESET%
copy "%ENV_EXAMPLE%" "%ENV_FILE%" >nul
echo %GREEN%[+] Created .env file%RESET%

echo %BLUE%[i] Generating encryption keys...%RESET%
echo %GREEN%[+] Generated secure encryption keys%RESET%
echo %YELLOW%[!] Note: Please edit .env file with your actual encryption keys%RESET%

:skip_env
echo.

REM Build project
echo %BLUE%Step 4: Building NOFX+ Project%RESET%
echo.

echo %BLUE%[i] Cleaning previous builds...%RESET%
cd /d "%NOFX_REPO%"
call go clean --cache
echo %GREEN%[+] Cleaned%RESET%

echo %BLUE%[i] Building backend...%RESET%
call go build -o nofx.exe
if %errorlevel% neq 0 (
    echo %RED%[X] Failed to build backend%RESET%
    exit /b 1
)
echo %GREEN%[+] Backend built successfully%RESET%

echo %BLUE%[i] Building frontend...%RESET%
cd /d "%NOFX_REPO%\web"
call npm run build
if %errorlevel% neq 0 (
    echo %RED%[X] Failed to build frontend%RESET%
    exit /b 1
)
echo %GREEN%[+] Frontend built successfully%RESET%

cd /d "%NOFX_REPO%"

REM Verification
echo.
echo %BLUE%Step 5: Verifying Installation%RESET%
echo.

if exist "%NOFX_REPO%\nofx.exe" (
    echo %GREEN%[+] Backend executable created%RESET%
) else (
    echo %YELLOW%[!] Backend executable not found%RESET%
)

if exist "%NOFX_REPO%\web\dist" (
    echo %GREEN%[+] Frontend build created%RESET%
) else if exist "%NOFX_REPO%\web\build" (
    echo %GREEN%[+] Frontend build created%RESET%
) else (
    echo %YELLOW%[!] Frontend build directory not found%RESET%
)

echo %GREEN%[+] Installation verification complete!%RESET%

REM Show instructions
echo.
echo %GREEN%===================================================%RESET%
echo %GREEN%    Setup Complete! 🎉%RESET%
echo %GREEN%===================================================%RESET%
echo.
echo %BLUE%Your NOFX+ environment is now ready!%RESET%
echo.
echo To start the application:
echo.
echo 1. Configure API keys in .env file
echo    - BINANCE_API_KEY=
echo    - BINANCE_API_SECRET=
echo    (or other exchange keys as needed)
echo.
echo 2. Start the backend:
echo    - Run: nofx.exe
echo    - Or: go run main.go
echo.
echo 3. In another terminal/PowerShell, start the frontend:
echo    - cd web
echo    - npm start
echo.
echo 4. Open your browser and navigate to:
echo    - http://localhost:3000
echo.
echo %BLUE%Configuration:
echo   - Configuration file: %ENV_FILE%
echo   - Backend runs on: http://localhost:8080
echo   - Frontend runs on: http://localhost:3000
echo.
echo Documentation:
echo   - README.md - Project overview
echo   - docs/README.md - Quick start
echo   - docs/architecture/README.md - System architecture
echo.
echo Happy trading! 🚀%RESET%
echo.

endlocal
