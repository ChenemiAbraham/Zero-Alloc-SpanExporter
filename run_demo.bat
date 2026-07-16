@echo off
REM Demo script - generates traces for TUI viewer
echo.
echo ╔══════════════════════════════════════════════════════════════╗
echo ║         LOCAL TRACE TAP - TRACE GENERATOR                    ║
echo ╚══════════════════════════════════════════════════════════════╝
echo.
echo Starting trace generator...
echo.
echo 💡 NOW: Open ANOTHER terminal and run:
echo    cd "%CD%"
echo    ltt.exe
echo.
echo Press Ctrl+C to stop generating traces
echo.

go run test_e2e.go
