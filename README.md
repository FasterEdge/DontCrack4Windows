<div align="center">
<img src="https://s2.loli.net/2025/10/30/whQl7sJryj1GbHU.png" style="width:100px;" width="100"/>
<h2>DontCrack4Windows</h2>
<h3>Windows 进程管理器 (Windows 7/8/10/11 / Server 2016+)</h3>
</div>


### 一、功能简介

- Windows 下的进程管理器，用于提高后台/守护进程的健壮性、可用性、时序稳定性
- 通过计划任务或 NSSM 把 DontCrack 注册为开机自启服务；进程崩溃时自动拉起
- 支持管理这些类型的进程：`.exe`、`.bat`、`.cmd`、`.ps1`
- 实现将进程对应到端口号，可通过 Restful API 实现获取日志、开关进程等操作
- 启动的进程可以配置独立的：程序路径、环境变量、启动参数、预处理脚本、是否自动重启、崩溃自动重启次数、是否立即启动、端口号、日志最大缓存行数、单行日志最大字节数、日志本地存储路径、日志本地存储周期等
- 支持跨架构，免 CGO，GOOS=windows + GOARCH=amd64/arm64 均可
- 与 OpenHarmony / Android / manylinux 版共享同一套命令行、HTTP 接口、JSON 字段

### 二、基础用法

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

| 配置项                | 类型     | 默认值                  | 说明                                                            |
| ------------------ | ------ | -------------------- | ------------------------------------------------------------- |
| path               | string | ""                   | 要管理的程序路径（支持 .exe、.bat、.cmd、.ps1 等）                     |
| args               | string | ""                   | 传递给程序的参数（可选）                                                  |
| pre                | string | ""                   | 启动前要执行的命令（在 cmd.exe 中执行，可用 && 连接多条命令，默认为空）              |
| env                | string | ""                   | 为子进程追加环境变量，如: "PATH=C:\Windows\System32;FOO=bar"            |
| auto-restart       | bool   | false                | 是否自动重启                                                        |
| max-retries        | int    | 3                    | 最大重试次数（-1表示无限次，默认3次）                                          |
| start-now          | bool   | false                | 是否立即启动                                                        |
| password           | string | ""                   | 管理进程的密码（可选，默认为空且不开启密码保护）                                      |
| port               | int    | 11883                | HTTP服务端口                                                      |
| log-capacity       | int    | 200                  | 日志缓存的最大行数（默认200）                                              |
| log-max-line-bytes | int    | 1048576              | 单行日志的最大字节数（用于bufio.Scanner，默认1MiB）                            |
| file-log           | bool   | false                | 是否启用文件日志（默认false）                                             |
| log-path           | string | logs\proc_manager\ | 本地日志文件目录（默认 logs\proc_manager\，按进程名创建子目录）              |
| log-life-day       | int    | 7                    | 本地日志文件保存天数（默认7天，新日志写入时会清理过期文件）                                |

### 三、接口文档

> /startup

- 接口说明：启动进程，同时会重置重启次数
- 请求方式：get、post
- 请求参数
  ```
  password: 密钥（可选params参数）
  ```
- 返回类型：文本
- 返回示例：
    ```
    ok
    ```

> /heartbeat

- 接口说明：获得心跳信息，会输出启动情况和缓存中的日志（同时会清除缓存）
- 请求方式：get、post
- 请求参数
  ```
  password: 密钥（可选params参数）
  ```
- 返回类型：JSON
- 返回示例：
     ```
	{
	"version": "1.0.20260826",
	"state": "stopped",
	"info": "进程管理器正常运行",
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

- 接口说明：终止进程
- 请求方式：get、post
- 请求参数
  ```
  password: 密钥（可选params参数）
  ```
- 返回类型：文本
- 返回示例：
  ```
  ok
  ```

### 四、细节说明

- 目标管理的进程的 Path 尽量使用全路径
- Windows 默认 shell 是 `cmd.exe` (本机探测路径 `C:\Windows\System32\cmd.exe`)，本管理器在 -pre 与批处理脚本调用时使用它
- `.bat` / `.cmd` 通过 `cmd.exe /C` 启动；`.ps1` 通过 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File` 启动
- 开启密码时，接口请求需要在 URL 参数中携带 `password` 参数，例如 `xxx/startup?password=123456`
- Windows 没有 POSIX 信号:
  - 优雅停止走 `os.Interrupt` (CTRL_BREAK_EVENT)，需要与子进程同控制台组
  - 不成功时 5s 超时后 `cmd.Process.Kill()` 强杀 (`TerminateProcess`)
- 与 OH / Android / manylinux 版的差异:
  - 启动横幅、根路径消息改为 `DontCrack_windows`
  - 二进制魔数检查改为 `MZ` (PE) 而非 `ELF`
  - 文件类型识别新增 `.bat` / `.cmd` / `.ps1`
  - 环境变量分隔符兼容 `;` 和空格
  - 默认日志目录改相对路径 `logs\proc_manager\`

### 五、安装与服务化

#### 方式 A: 计划任务（推荐，最低成本）

参见 `example/install.bat`，运行后会:
1. 把 `DontCrack.exe` 与 `childproc.exe` 复制到 `C:\ProgramData\DontCrack\bin\`
2. 创建日志目录
3. 注册名为 `DontCrack_Sample` 的开机自启任务（SYSTEM 身份）
4. 启动一次测试并查询心跳

**注意：必须以管理员身份运行 cmd / PowerShell。**

#### 方式 B: NSSM（更稳，真正的服务）

[NSSM](https://nssm.cc/) 可把任意 exe 注册为真正的 Windows 服务：

```cmd
nssm install DontCrack "C:\ProgramData\DontCrack\bin\DontCrack.exe" ^
  "-path C:\ProgramData\DontCrack\bin\childproc.exe -args -mode normal -port 11883 -start-now -auto-restart"
nssm set DontCrack AppStdout "C:\ProgramData\DontCrack\logs\service.out.log"
nssm set DontCrack AppStderr "C:\ProgramData\DontCrack\logs\service.err.log"
nssm start DontCrack
```

NSSM 提供: 失败自动重启、服务崩溃可执行恢复脚本、启动前延时等，比 schtasks 更专业。

#### 方式 C: 直接前台运行

```cmd
DontCrack.exe -path C:\...\childproc.exe -args "-mode hang" -start-now -port 11883
```

适合调试；关掉 cmd 窗口即停止。

### 六、构建

```powershell
# 在 Windows 上
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags='-s -w' -o DontCrack.exe .
cd .\example\childproc
go build -ldflags='-s -w' -o childproc.exe .
```

或在 macOS / Linux 上交叉编译：

```bash
cd DontCrack4Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-s -w' -o DontCrack.exe .
cd example/childproc
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-s -w' -o childproc.exe .
```

> ARM64 Windows (Surface Pro X / Snapdragon X) 改 `GOARCH=arm64`。

### 七、使用技巧

- 单独使用时如果在 cmd / PowerShell 中使用 `start /b` 作为命令结尾时一般会话结束的时候这个操作也会被杀死
- Go 语言程序的 `log.Printf` 默认将数据写到 `os.Stderr`，所以子进程中日志类型会显示为 `[STDERR]`，可以换成 `fmt.Println` 得到非 `[STDERR]` 的消息
- 因为可以通过 HTTP 带加密的形式操作进程，你可以根据此文档将进程操作结合 Windows 服务角色，再结合 AI + MCP（或 Skills）完成各种操作
- 除了可以使用直接功能保证进程健壮性，还可以利用反复重启机制实现进程轮询，预先脚本也能实现延迟启动、等待依赖进程等操作