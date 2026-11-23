@echo off
rem VSTodo Installer Script
rem Simple installer that copies binary to user PATH and creates shortcut

set "VSTODO_BIN=vstodo.exe"
set "DEST_DIR=%USERPROFILE%\AppData\Local\VSTodo"
set "SHORTCUT=%APPDATA%\Microsoft\Windows\Start Menu\Programs\VSTodo\VSTodo.lnk"

rem Create install directory
mkdir "%DEST_DIR%"

rem Copy binary from current directory
copy "%~dp0%VSTODO_BIN%" "%DEST_DIR%\%VSTODO_BIN%"
if errorlevel 1 (
    echo Error: Could not copy binary.
    pause
    exit /b 1
)

rem Add to PATH (user level) if not already there
setx PATH "%PATH%;%DEST_DIR%" /M

rem Create startup folder shortcut (optional)
mkdir "%APPDATA%\Microsoft\Windows\Start Menu\Programs\VSTodo" 2>nul
echo @echo off > "%SHORTCUT%"
echo "%DEST_DIR%\vstodo.exe %*" >> "%SHORTCUT%"

rem Verify installation
echo.
echo VSTodo installed to: %DEST_DIR%
echo.
echo Verification:
vstodo --version
echo.
echo Installation complete!
pause