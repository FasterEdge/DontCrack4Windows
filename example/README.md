# 示例子进程

这是一个用来全面测试进程管理器行为的示例子进程。

## 功能
- 打印启动参数与关键环境变量，便于确认传参是否生效
- 周期性输出 stdout/stderr，帮助验证日志抓取
- 支持模拟崩溃、优雅退出、挂起不退出等场景，便于测试重启和超时
- 响应 CTRL_C_EVENT / CTRL_BREAK_EVENT，执行清理后退出
- 可选通过文件记录重启计数，方便观察自动重启

## 构建

在 PowerShell 中:

```powershell
cd .\childproc
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags='-s -w' -o childproc.exe .
```

或交叉编译（在 macOS / Linux 上）:

```bash
cd ./childproc
GOOS=windows GOARCH=amd64 go build -ldflags='-s -w' -o childproc.exe .
```

> 默认 amd64；若是 ARM64 设备 (例如 Windows on ARM) 可改为 `GOARCH=arm64`。

## 运行示例

```cmd
childproc.exe -mode normal -interval 1s -lifetime 10s
childproc.exe -mode crash -interval 500ms            REM 模拟崩溃退出(退出码=42)
childproc.exe -mode hang                             REM 挂起，不退出，用于测试管理器超时/强杀
childproc.exe -mode graceful -lifetime 5s            REM 依赖信号优雅退出
childproc.exe -mode normal -state-file C:\Temp\counter.txt  REM 重启计数写入文件
```

## 重要参数
- `-mode`：`normal`(默认) / `crash` / `hang` / `graceful`
- `-interval`：输出间隔，默认 `1s`
- `-lifetime`：在 normal/crash 模式下运行多久后退出；空值表示一直运行。
- `-state-file`：若设置，则会读写该文件的计数用于观测重启次数。
- `-message`：自定义输出前缀，便于区分不同实例。

## 环境变量
- `EXTRA_INFO`：会被打印出来，方便验证环境传递。
- `RESTART_ENV_COUNT`：若存在，会尝试解析为整数，自增后打印回显，用于观察管理器是否覆盖/继承环境。

## 信号
- Windows 上 Go 通过 `os.Interrupt` (CTRL_BREAK_EVENT / CTRL_C_EVENT) 触发优雅退出。
- `hang` 模式下信号会被记录但不会退出，便于测试管理器的强制 kill。

## 与管理器联动
- 将管理器的 `-path` 指向构建好的 `childproc.exe`。
- 使用管理器的 `-args` 传递上述参数，例如：
  ```cmd
  -args "-mode crash -interval 200ms -message manager"
  ```
- 配合 `-env "EXTRA_INFO=from_manager RESTART_ENV_COUNT=0"` 观察环境是否生效。

## Windows 安装

参见 `example/install.bat`，会自动:
1. 把 `DontCrack.exe` 与 `childproc.exe` 复制到 `C:\ProgramData\DontCrack\bin\`
2. 创建日志目录 `C:\ProgramData\DontCrack\logs\`
3. 注册一个开机自启的计划任务
4. 启动一次测试并查询心跳

> ⚠️ 必须以**管理员身份**运行 cmd / PowerShell 才能注册计划任务。

## Windows 服务化（可选）

更彻底的服务化方案：
- 使用 `nssm.exe` (the Non-Sucking Service Manager) 把 `DontCrack.exe` 注册为系统服务:
  ```cmd
  nssm install DontCrack "C:\ProgramData\DontCrack\bin\DontCrack.exe" "-path C:\...\childproc.exe -args -mode normal -port 11883 -start-now -auto-restart"
  nssm set DontCrack AppStdout "C:\ProgramData\DontCrack\logs\service.out.log"
  nssm set DontCrack AppStderr "C:\ProgramData\DontCrack\logs\service.err.log"
  nssm start DontCrack
  ```
- 或用 PowerShell 自己写一个 Wrapper Service，调用 `DontCrack.exe`。

## 跨平台注意
- 同 OH / Android / manylinux 版一致，命令行参数、HTTP 接口 (`/startup` `/heartbeat` `/shutdown`)、JSON 字段全部相同
- 唯一区别：Windows 下 `-pre` 通过 `cmd.exe /C` 执行；脚本类型识别 `.bat` / `.cmd` / `.ps1`