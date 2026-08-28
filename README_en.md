<div align="center">
<img src="./Logo.png" alt="logo" width="100"/>
<h2>DontCrack4Windows</h2>
<h3>Windows Process Manager (Windows 7/8/10/11 / Server 2016+)</h3>
</div>


### 1. Features

- A process manager for Windows, designed to improve the robustness, availability and timing stability of background/daemon processes
- Register DontCrack as an auto-start service via Scheduled Tasks or NSSM; automatically restarts on crash
- Supports managing these process types: `.exe`, `.bat`, `.cmd`, `.ps1`
- Maps processes to port numbers; RESTful API for getting logs, starting/stopping processes, etc.
- Each managed process can be configured independently with: program path, environment variables, startup arguments, pre-processing script, auto-restart toggle, max crash-retry count, start-now flag, port number, log cache line limit, per-line byte limit, local log storage path, log retention period, etc.
- Cross-architecture, no CGO required; `GOOS=windows + GOARCH=amd64/arm64` both supported
- Shares the same CLI, HTTP API and JSON field structure with the OpenHarmony / Android / manylinux versions

### 2. Basic Usage

```cmd
DontCrack.exe ^
  -path "C:\ProgramData\DontCrack\bin\childproc.exe" ^
  -args "-mode normal -interval 2s" ^
  -env "EXTRA_INFO=from_manager RESTART_ENV_COUNT=0" ^
  -file-log -log-path C:\ProgramData\DontCrack\logs\ -log-life-day 7 ^
  -auto-restart -max-retries 2 ^
  -start-now ^
  -password 123456
```

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| path | string | "" | Path of the program to manage (supports .exe, .bat, .cmd, .ps1 etc.) |
| args | string | "" | Arguments passed to the program (optional) |
| pre | string | "" | Command to run before startup (executed in cmd.exe; multiple commands can be chained with &&, default empty) |
| env | string | "" | Environment variables to append for the child process, e.g. "PATH=C:\Windows\System32;FOO=bar" |
| auto-restart | bool | false | Whether to auto-restart on crash |
| max-retries | int | 3 | Max retry count (-1 means unlimited, default 3) |
| start-now | bool | false | Whether to start immediately |
| password | string | "" | Password for managing the process (optional; no password protection if empty) |
| port | int | 11883 | HTTP service port |
| log-capacity | int | 200 | Max lines of cached logs (default 200) |
| log-max-line-bytes | int | 1048576 | Max bytes per log line (for bufio.Scanner, default 1 MiB) |
| file-log | bool | false | Whether to enable file logging (default false) |
| log-path | string | logs\proc_manager\ | Local log file directory (default logs\proc_manager\, subdirectories created per process name) |
| log-life-day | int | 7 | Log retention in days (default 7; expired logs cleaned when new logs are written) |

### 3. API Documentation

> /startup

- Description: Starts the process and resets the retry count
- Method: GET, POST
- Request parameters:
  ```
  password: secret key (optional params parameter)
  ```
- Response type: text
- Example response:
    ```
    ok
    ```

> /heartbeat

- Description: Returns heartbeat information, including startup status and cached logs (logs are cleared after reading)
- Method: GET, POST
- Request parameters:
  ```
  password: secret key (optional params parameter)
  ```
- Response type: JSON
- Example response:
     ```
	{
	"version": "1.0.20260826",
	"state": "stopped",
	"info": "Process manager running normally",
	"timestamp": "2026-08-25 15:28:04",
	"logs": [
	"[STDOUT] 2026/08/25 15:27:55.647714 env restart count -> 1",
	"[STDOUT] 2026/08/25 15:27:55.648316 childproc start | pid=32054 | mode=normal | interval=1s | lifetime=5s | msg=",
	"[STDOUT] 2026/08/25 15:28:00.686300 lifetime reached, exiting normally"
	],
	"process_pid": 0,
	"process_path": "C:\\ProgramData\\DontCrack\\bin\\childproc.exe",
	"restart_count": 0,
	"file_type": "binary_executable",
	"last_exit_time": "2026-08-25 15:28:00",
	"program_args": "-mode normal -interval 1s -lifetime 5s",
	"extra_env_raw": "EXTRA_INFO=from_manager RESTART_ENV_COUNT=0"
	}
	```

> /shutdown

- Description: Terminates the process
- Method: GET, POST
- Request parameters:
  ```
  password: secret key (optional params parameter)
  ```
- Response type: text
- Example response:
  ```
  ok
  ```

### 4. Details

- Use full paths for the managed process's Path whenever possible
- The default shell on Windows is `cmd.exe` (detected at `C:\Windows\System32\cmd.exe`); it is used for the `-pre` command and batch script execution
- `.bat` / `.cmd` files are launched via `cmd.exe /C`; `.ps1` files via `powershell.exe -NoProfile -ExecutionPolicy Bypass -File`
- When password protection is enabled, API requests must include the `password` parameter in the URL, e.g. `xxx/startup?password=123456`
- Windows does not have POSIX signals:
  - Graceful stop uses `os.Interrupt` (CTRL_BREAK_EVENT), which requires sharing the same console group with the child process
  - If unsuccessful, `cmd.Process.Kill()` (TerminateProcess) is called after a 5s timeout
- Differences from the OH / Android / manylinux versions:
  - Startup banner and root path messages changed to `DontCrack_windows`
  - Binary magic number check changed to `MZ` (PE) instead of `ELF`
  - File type recognition adds `.bat` / `.cmd` / `.ps1`
  - Environment variable separator supports both `;` and space
  - Default log directory changed to relative path `logs\proc_manager\`

### 5. Installation and Service Registration

#### Method A: Scheduled Task (Recommended, Lowest Cost)

See `example/install.bat`. After running it:
1. Copies `DontCrack.exe` and `childproc.exe` to `C:\ProgramData\DontCrack\bin\`
2. Creates the log directory
3. Registers a boot-time auto-start task named `DontCrack_Sample` (running as SYSTEM)
4. Runs a test startup and queries the heartbeat

**Note: Must run cmd / PowerShell as Administrator.**

#### Method B: NSSM (More Stable, True Service)

[NSSM](https://nssm.cc/) can register any exe as a real Windows service:

```cmd
nssm install DontCrack "C:\ProgramData\DontCrack\bin\DontCrack.exe" ^
  "-path C:\ProgramData\DontCrack\bin\childproc.exe -args -mode normal -port 11883 -start-now -auto-restart"
nssm set DontCrack AppStdout "C:\ProgramData\DontCrack\logs\service.out.log"
nssm set DontCrack AppStderr "C:\ProgramData\DontCrack\logs\service.err.log"
nssm start DontCrack
```

NSSM provides: automatic restart on failure, recovery scripts on service crash, startup delay, etc., making it more professional than schtasks.

#### Method C: Direct Foreground Run

```cmd
DontCrack.exe -path C:\...\childproc.exe -args "-mode hang" -start-now -port 11883
```

Suitable for debugging; closes when the cmd window is closed.

### 6. Build

```powershell
# On Windows
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags='-s -w' -o DontCrack.exe .
cd .\example\childproc
go build -ldflags='-s -w' -o childproc.exe .
```

Or cross-compile on macOS / Linux:

```bash
cd DontCrack4Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-s -w' -o DontCrack.exe .
cd example/childproc
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-s -w' -o childproc.exe .
```

> For ARM64 Windows (Surface Pro X / Snapdragon X), change `GOARCH=arm64`.

### 7. Tips

- When used standalone, if `start /b` is used at the end of a command in cmd / PowerShell, the process may be killed when the session ends
- Go's `log.Printf` writes to `os.Stderr` by default, so log entries from child processes will show as `[STDERR]`; switch to `fmt.Println` to get non-`[STDERR]` messages
- Since processes can be controlled via encrypted HTTP, you can combine this with Windows service roles, AI + MCP (or Skills) to implement various automation operations
- Besides using direct functionality for process robustness, the repeated restart mechanism can also be used for process polling, and the pre-script can implement delayed startup, waiting for dependent processes, etc.