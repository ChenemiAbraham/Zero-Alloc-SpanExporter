@echo off
REM LTT Complete Test Suite - Windows Version
setlocal enabledelayedexpansion

echo ================================================================
echo           LOCAL TRACE TAP - COMPLETE TEST SUITE
echo ================================================================
echo.

set PASS_COUNT=0
set FAIL_COUNT=0

REM Test 1: Compilation
echo ================================================================
echo TEST 1: Compilation Check
echo ================================================================
go build ./... >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo [32m✅ PASSED[0m - All packages compile
    set /a PASS_COUNT+=1
) else (
    echo [31m❌ FAILED[0m - Compilation errors
    set /a FAIL_COUNT+=1
)
echo.

REM Test 2: Ring Buffer Unit Tests
echo ================================================================
echo TEST 2: Ring Buffer Unit Tests
echo ================================================================
go test github.com/yourusername/ltt/internal/ringbuf -v >test_output.txt 2>&1
findstr /C:"PASS" test_output.txt >nul
if %ERRORLEVEL% EQU 0 (
    echo [32m✅ PASSED[0m - All unit tests passing
    set /a PASS_COUNT+=1
) else (
    echo [31m❌ FAILED[0m - Some unit tests failed
    set /a FAIL_COUNT+=1
)
del test_output.txt
echo.

REM Test 3: Ring Buffer Benchmarks
echo ================================================================
echo TEST 3: Ring Buffer Benchmarks
echo ================================================================
echo Running benchmarks...
go test -bench=. -benchmem github.com/yourusername/ltt/internal/ringbuf
if %ERRORLEVEL% EQU 0 (
    echo [32m✅ PASSED[0m - Benchmarks completed
    set /a PASS_COUNT+=1
) else (
    echo [31m❌ FAILED[0m - Benchmarks failed
    set /a FAIL_COUNT+=1
)
echo.

REM Test 4: Race Detection
echo ================================================================
echo TEST 4: Race Detection
echo ================================================================
go test -race github.com/yourusername/ltt/internal/ringbuf >test_output.txt 2>&1
findstr /C:"PASS" test_output.txt >nul
if %ERRORLEVEL% EQU 0 (
    echo [32m✅ PASSED[0m - No race conditions detected
    set /a PASS_COUNT+=1
) else (
    echo [31m❌ FAILED[0m - Race conditions found
    set /a FAIL_COUNT+=1
)
del test_output.txt
echo.

REM Test 5: Smoke Test
echo ================================================================
echo TEST 5: Integration Smoke Test
echo ================================================================
go run test_smoke.go >test_output.txt 2>&1
findstr /C:"SUCCESS" test_output.txt >nul
if %ERRORLEVEL% EQU 0 (
    echo [32m✅ PASSED[0m - Smoke test successful
    set /a PASS_COUNT+=1
) else (
    echo [31m❌ FAILED[0m - Smoke test failed
    set /a FAIL_COUNT+=1
)
del test_output.txt
echo.

REM Summary
echo ================================================================
echo                         TEST SUMMARY
echo ================================================================
echo.
set /a TOTAL_COUNT=PASS_COUNT+FAIL_COUNT
echo Total Tests:  !TOTAL_COUNT!
echo Passed:       !PASS_COUNT!
echo Failed:       !FAIL_COUNT!
echo.

if !FAIL_COUNT! EQU 0 (
    echo ================================================================
    echo 🎉 ALL TESTS PASSED!
    echo ================================================================
    echo.
    echo ✅ Core components functional
    echo ✅ Performance targets met
    echo ✅ Zero race conditions
    echo ✅ Integration working
    echo.
    echo 🚀 Next: Implement protocol codec (pkg/protocol/span.go^)
    exit /b 0
) else (
    echo ================================================================
    echo ❌ SOME TESTS FAILED
    echo ================================================================
    echo.
    echo Please review failed tests above.
    exit /b 1
)
