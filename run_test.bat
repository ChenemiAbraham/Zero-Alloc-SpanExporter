@echo off
REM Quick test script - verifies everything works
echo.
echo ╔══════════════════════════════════════════════════════════════╗
echo ║         LOCAL TRACE TAP - INTEGRATION TEST                   ║
echo ╚══════════════════════════════════════════════════════════════╝
echo.

go run test_integration.go

echo.
echo ═══════════════════════════════════════════════════════════════
echo.
echo If all tests passed, you can now run the full demo:
echo.
echo   1. Open a NEW terminal window
echo   2. Run: run_demo.bat
echo.
pause
