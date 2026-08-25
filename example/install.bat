@echo off
REM ============================================================
REM DontCrack Windows 安装脚本 (示例)
REM ============================================================
REM 使用方式（在管理员 PowerShell / cmd 中执行）：
REM   1. 把 DontCrack.exe 放到本目录
REM   2. 双击 install.bat，或在 cmd 中执行 install.bat
REM   3. 脚本会:
REM      - 创建 C:\ProgramData\DontCrack\bin\
REM      - 复制 DontCrack.exe 与 childproc.exe
REM      - 创建 C:\ProgramData\DontCrack\logs\
REM      - 创建一个简单的 Windows 计划任务，开机自启
REM ============================================================

setlocal

set "DEST_DIR=C:\ProgramData\DontCrack"
set "BIN_DIR=%DEST_DIR%\bin"
set "LOG_DIR=%DEST_DIR%\logs"
set "SAMPLE_PROC=childproc.exe"
set "SAMPLE_ARGS=-mode normal -interval 2s"
set "SAMPLE_PORT=11883"

echo === DontCrack 安装脚本 ===

REM 检查管理员权限
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [错误] 需要管理员权限运行
    pause
    exit /b 1
)

REM 创建目录
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
if not exist "%LOG_DIR%" mkdir "%LOG_DIR%"

REM 复制可执行文件
if exist "DontCrack.exe" (
    copy /y "DontCrack.exe" "%BIN_DIR%\DontCrack.exe"
) else (
    echo [警告] 未找到 DontCrack.exe，请先把编译产物放到本目录
)
if exist "%SAMPLE_PROC%" (
    copy /y "%SAMPLE_PROC%" "%BIN_DIR%\%SAMPLE_PROC%"
) else (
    echo [警告] 未找到 %SAMPLE_PROC%
)

REM 注册计划任务：开机自启
set "TASK_NAME=DontCrack_Sample"
schtasks /delete /tn "%TASK_NAME%" /f >nul 2>&1
schtasks /create ^
    /tn "%TASK_NAME%" ^
    /tr "\"%BIN_DIR%\DontCrack.exe\" -path \"%BIN_DIR%\%SAMPLE_PROC%\" -args \"%SAMPLE_ARGS%\" -start-now -auto-restart -max-retries 3 -port %SAMPLE_PORT% -file-log -log-path \"%LOG_DIR%\" -log-life-day 7" ^
    /sc onstart ^
    /ru SYSTEM ^
    /rl HIGHEST ^
    /f
if %errorlevel% equ 0 (
    echo [成功] 已创建计划任务 %TASK_NAME%
) else (
    echo [错误] 创建计划任务失败
)

REM 启动一次测试
echo === 启动测试（5秒后自动停止） ===
start "" "%BIN_DIR%\DontCrack.exe" -path "%BIN_DIR%\%SAMPLE_PROC%" -args "%SAMPLE_ARGS%" -start-now -auto-restart -port %SAMPLE_PORT%
timeout /t 5 /nobreak >nul
echo === 通过 HTTP 查询心跳 ===
curl -s "http://127.0.0.1:%SAMPLE_PORT%/heartbeat"
echo.
echo === 安装完成 ===

endlocal
pause