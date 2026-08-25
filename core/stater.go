package core

// FasterEdge - 对称、可靠、安全的多场景边缘计算框架
// https://github.com/FasterEdge
// DontCrack Windows 版
// https://github.com/FasterEdge/DontCrack4Windows
// tyza66
// https://github.com/tyza66
import (
	"DontCrack/config"
	pmexec "DontCrack/exec"
	dclog "DontCrack/log"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const logo = `
 ________  ________           _______          ________  ___  ___
|\   ___ \|\   ____\         /  ___  \        |\   __  \|\  \|\  \
\ \  \_|\ \ \  \___|        /__/|_/  /|       \ \  \|\  \ \  \\\  \
 \ \  \ \\ \ \  \           |__|//  / /        \ \  \\\  \ \   __  \
  \ \  \_\\ \ \  \____          /  /_/__        \ \  \\\  \ \  \ \  \
   \ \_______\ \_______\       |\________\       \ \_______\ \__\ \__\
    \|_______|\|_______|        \|_______|        \|_______|\|__|\|__|
`

const rootMsg = "DontCrack_windows By FasterEdge"

var (
	procState   pmexec.Process
	logCache    []string
	logMu       sync.Mutex
	fileLogger  *dclog.FileLogger
)

// 初始化并启动管理器
func Start(cfg config.Config) {
	// 进程退出时优雅关闭日志文件
	defer func() {
		if fileLogger != nil {
			_ = fileLogger.Close()
		}
	}()
	err := config.CheckConfig(cfg)
	if err != nil {
		log.Fatalf("配置检查失败: %v\n", err)
		return
	}
	log.Printf(logo)
	log.Printf("DontCrack_windows v%s 启动中...\n", cfg.Version)
	log.Printf("管理进程正在管理的程序: %s\n", cfg.Path)

	procState.FileType, err = detectFileType(cfg.Path)
	if err != nil {
		log.Fatalf("无法检测文件类型: %v\n", err)
		return
	}

	// 启动健康探针(若启用)
	startHealthProbe(cfg)
	if cfg.FileLogEnabled {
		procName := filepath.Base(cfg.Path)
		fl, ferr := dclog.NewFileLogger(cfg.LocalLogPath, procName, cfg.LocalLogLifeDay)
		if ferr != nil {
			log.Printf("初始化文件日志失败，继续运行但不落盘: %v", ferr)
		} else {
			fileLogger = fl
			log.Printf("文件日志启用，目录=%s，保留天数=%d", cfg.LocalLogPath, cfg.LocalLogLifeDay)
		}
	}

	log.Printf("检测到文件类型: %s", procState.FileType)
	if cfg.Args != "" {
		log.Printf("程序参数: %s", cfg.Args)
	}
	log.Printf("自动重启: %v", cfg.AutoRestart)
	log.Printf("最大重试次数: %d", cfg.RestartTimes)
	log.Printf("立即启动: %v", cfg.StartNow)
	log.Printf("HTTP端口: %d", cfg.Port)

	if cfg.StartNow {
		log.Println("立即启动目标进程...")
		if err := startProcess(cfg); err != nil {
			log.Printf("立即启动失败: %v", err)
		}
	}

	// 信号处理: Windows 上 CTRL_BREAK / CTRL_C 通过 os.Interrupt 处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	mux := http.NewServeMux()
	// Web UI: 嵌入的单文件 HTML, /ui 与 /ui/ 都直接返回页面
	mux.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(webuiHTML)
	})
	mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(webuiHTML)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, rootMsg)
	})
	// /healthz: K8s/LB 健康检查. 管理器与子进程都健康才返回 200
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, cfg.Password); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, rootMsg)
			return
		}
		procState.ProcessMu.Lock()
		healthy := procState.IsRunning && procState.CurrentProcess != nil
		procState.ProcessMu.Unlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if healthy {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "ok")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "process not running")
		}
	})
	// /readyz: 同 healthz, 语义拆开便于 K8s 配置 livenessProbe vs readinessProbe
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, cfg.Password); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, rootMsg)
			return
		}
		procState.ProcessMu.Lock()
		ready := procState.IsRunning && procState.CurrentProcess != nil
		procState.ProcessMu.Unlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if ready {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "not ready")
		}
	})
	// /metrics: Prometheus 文本格式指标
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, cfg.Password); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, rootMsg)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		procState.ProcessMu.Lock()
		var (
			stateFloat float64
			pid        int
			restartCnt = procState.RestartCount
			fileType   = procState.FileType
			procPath   = cfg.Path
			lastExitTs string
			lastExitCd = procState.ExitInfo.LastExitCode
		)
		if procState.IsRunning && procState.CurrentProcess != nil {
			stateFloat = 1
			pid = procState.CurrentProcess.Process.Pid
		} else {
			stateFloat = 0
		}
		if !procState.ExitInfo.LastExitTime.IsZero() {
			lastExitTs = procState.ExitInfo.LastExitTime.Format(time.RFC3339)
		}
		procState.ProcessMu.Unlock()

		logMu.Lock()
		logLines := len(logCache)
		logMu.Unlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, "# HELP dontcrack_up 管理器是否在运行(始终 1)\n")
		fmt.Fprintf(w, "# TYPE dontcrack_up gauge\n")
		fmt.Fprintf(w, "dontcrack_up 1\n")
		fmt.Fprintf(w, "# HELP dontcrack_process_state 子进程运行状态 1=running 0=stopped\n")
		fmt.Fprintf(w, "# TYPE dontcrack_process_state gauge\n")
		fmt.Fprintf(w, "dontcrack_process_state %d\n", int(stateFloat))
		fmt.Fprintf(w, "# HELP dontcrack_process_pid 子进程 PID\n")
		fmt.Fprintf(w, "# TYPE dontcrack_process_pid gauge\n")
		fmt.Fprintf(w, "dontcrack_process_pid %d\n", pid)
		fmt.Fprintf(w, "# HELP dontcrack_restart_count 累计自动重启次数\n")
		fmt.Fprintf(w, "# TYPE dontcrack_restart_count counter\n")
		fmt.Fprintf(w, "dontcrack_restart_count %d\n", restartCnt)
		fmt.Fprintf(w, "# HELP dontcrack_last_exit_code 子进程上次退出码\n")
		fmt.Fprintf(w, "# TYPE dontcrack_last_exit_code gauge\n")
		fmt.Fprintf(w, "dontcrack_last_exit_code %d\n", lastExitCd)
		fmt.Fprintf(w, "# HELP dontcrack_last_exit_time_seconds 子进程上次退出 unix 时间戳\n")
		fmt.Fprintf(w, "# TYPE dontcrack_last_exit_time_seconds gauge\n")
		var exitTs float64
		if lastExitTs != "" {
			if t, err := time.Parse(time.RFC3339, lastExitTs); err == nil {
				exitTs = float64(t.Unix())
			}
		}
		fmt.Fprintf(w, "dontcrack_last_exit_time_seconds %d\n", int(exitTs))
		fmt.Fprintf(w, "# HELP dontcrack_log_lines 内存中缓存的日志行数\n")
		fmt.Fprintf(w, "# TYPE dontcrack_log_lines gauge\n")
		fmt.Fprintf(w, "dontcrack_log_lines %d\n", logLines)
		fmt.Fprintf(w, "# HELP dontcrack_info 版本/平台信息\n")
		fmt.Fprintf(w, "# TYPE dontcrack_info gauge\n")
		fmt.Fprintf(w, "dontcrack_info{version=%q,file_type=%q,process_path=%q} 1\n", cfg.Version, fileType, procPath)
	})
	mux.HandleFunc("/startup", func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, cfg.Password); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, rootMsg)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		procState.ProcessMu.Lock()
		procState.RestartCount = 0
		procState.ProcessMu.Unlock()
		if err := startProcess(cfg); err != nil {
			http.Error(w, fmt.Sprintf("启动进程失败: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, cfg.Password); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, rootMsg)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		logMu.Lock()
		logsCopy := logCache
		logCache = nil
		logMu.Unlock()

		procState.ProcessMu.Lock()
		state := "stopped"
		pid := 0
		if procState.IsRunning && procState.CurrentProcess != nil {
			state = "running"
			pid = procState.CurrentProcess.Process.Pid
		}
		exit := procState.ExitInfo
		restartCnt := procState.RestartCount
		fileType := procState.FileType
		procPath := cfg.Path
		procState.ProcessMu.Unlock()

		info := HeartbeatInfo{
			Version:          cfg.Version,
			State:            state,
			Info:             "进程管理器正常运行",
			Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
			Logs:             logsCopy,
			ProcessPID:       pid,
			ProcessPath:      procPath,
			RestartCount:     restartCnt,
			FileType:         fileType,
			LastExitCode:     exit.LastExitCode,
			LastExitBySignal: exit.LastExitBySignal,
			LastExitError:    exit.LastExitError,
			ProgramArgs:      cfg.Args,
			ExtraEnvRaw:      cfg.Env,
		}
		if !exit.LastExitTime.IsZero() {
			info.LastExitTime = exit.LastExitTime.Format("2006-01-02 15:04:05")
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		data, err := json.Marshal(info)
		if err != nil {
			http.Error(w, fmt.Sprintf("JSON序列化错误: %v", err), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, cfg.Password); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, rootMsg)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := stopProcess(); err != nil {
			http.Error(w, fmt.Sprintf("停止进程失败: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Printf("HTTP服务器启动在端口 %d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器启动失败: %v", err)
		}
	}()

	log.Println("进程管理器运行中...")

	sig := <-sigChan
	log.Printf("收到信号 %v，开始关闭...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if procState.IsRunning {
		log.Println("正在停止管理的进程...")
		if err := stopProcess(); err != nil {
			log.Printf("停止进程时出错: %v", err)
		}
	}

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP服务器关闭出错: %v", err)
	}

	log.Println("进程管理器已优雅关闭")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 合并环境变量并返回 PATH 便于日志打印（Windows 用 ; 分隔，Path 变量名大小写不敏感）
// 注意: 自定义 env 必须放在 base 前面，-env PATH=/foo 才能真正覆盖
func buildChildEnv(envStr string) ([]string, string) {
	base := os.Environ()
	add := parseExtraEnv(envStr)
	// 自定义环境放最前，覆盖基础环境
	env := append([]string{}, add...)
	env = append(env, base...)
	pathVal := ""
	for i := 0; i < len(env); i++ {
		// Windows 环境变量名 Path 不区分大小写
		idx := strings.Index(env[i], "=")
		if idx > 0 && strings.EqualFold(env[i][:idx], "PATH") {
			pathVal = env[i][idx+1:]
			break
		}
	}
	if pathVal == "" {
		pathVal = os.Getenv("Path")
		if pathVal == "" {
			pathVal = os.Getenv("PATH")
		}
	}
	return env, pathVal
}

// 解析以空格/分号分隔的 KEY=VAL 列表（Windows 下分号才是真正的分隔符）
func parseExtraEnv(s string) []string {
	if s == "" {
		return nil
	}
	// Windows 上 '; ' 才是 PATH/环境变量分隔；空格则按"键值内的空格"处理
	// 这里采用简单实现: 同时按 ; 与 空白 split，再过滤出包含 = 的项
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == ' '
	})
	var out []string
	for _, f := range fields {
		if !strings.Contains(f, "=") {
			continue
		}
		out = append(out, f)
	}
	return out
}

// getShellCmd 构造基于本机 cmd.exe 的 exec.Cmd（用于 -pre 命令）
func getShellCmd(dir string, args ...string) *osexec.Cmd {
	shellPath := findShell()
	if shellPath == "" {
		shellPath = "cmd"
	}
	fullArgs := append([]string{}, args...)
	cmd := osexec.Command(shellPath, fullArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd
}

// 在 cmd.exe 中执行启动前命令
// -pre "<command>" 通过 cmd.exe /C 执行
func runPreCommand(cfg config.Config) error {
	if cfg.Pre == "" {
		return nil
	}
	log.Printf("执行启动前命令: %s", cfg.Pre)
	cmd := getShellCmd(filepath.Dir(cfg.Path), "/C", cfg.Pre)
	if env, pathVal := buildChildEnv(cfg.Env); len(env) > 0 {
		cmd.Env = env
		log.Printf("PRE环境PATH: %s", pathVal)
	}
	out, err := cmd.CombinedOutput()

	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	logMu.Lock()
	for _, line := range lines {
		if line == "" {
			continue
		}
		logCache = append(logCache, "[PRE] "+line)
		if len(logCache) > cfg.LogCapacity {
			logCache = logCache[len(logCache)-cfg.LogCapacity:]
		}
	}
	logMu.Unlock()

	if err != nil {
		return fmt.Errorf("pre命令执行失败: %v", err)
	}
	return nil
}

// 根据文件类型构造 osexec.Cmd（Windows 风格）
func createCommand(cfg config.Config) *osexec.Cmd {
	var cmd *osexec.Cmd
	args := []string{}
	if cfg.Args != "" {
		args = strings.Fields(cfg.Args)
	}

	shellPath := findShell()
	if shellPath == "" {
		shellPath = "cmd"
	}

	switch procState.FileType {
	case "batch_script":
		// cmd.exe /C <script.bat> <args...>
		cmdArgs := append([]string{"/C", cfg.Path}, args...)
		cmd = osexec.Command(shellPath, cmdArgs...)
	case "powershell_script":
		// powershell.exe -NoProfile -ExecutionPolicy Bypass -File <script.ps1> <args...>
		psPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
		if _, err := os.Stat(psPath); err != nil {
			psPath = "powershell"
		}
		cmdArgs := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", cfg.Path}, args...)
		cmd = osexec.Command(psPath, cmdArgs...)
	default:
		// 二进制 / .exe / 未知: 直接执行
		if len(args) > 0 {
			cmd = osexec.Command(cfg.Path, args...)
		} else {
			cmd = osexec.Command(cfg.Path)
		}
	}

	cmd.Dir = filepath.Dir(cfg.Path)
	return cmd
}

// startHealthProbe 启动健康探针 goroutine（若配置启用）
// Windows 平台用 cmd.exe /C 包装探针命令
func startHealthProbe(cfg config.Config) {
	if cfg.ProbeCmd == "" {
		return
	}
	interval := time.Duration(cfg.ProbeInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timeout := time.Duration(cfg.ProbeTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	limit := cfg.ProbeFailureLimit
	if limit <= 0 {
		limit = 3
	}
	log.Printf("健康探针已启用：%q, 间隔=%s, 超时=%s, 失败阈值=%d", cfg.ProbeCmd, interval, timeout, limit)

	go func() {
		consecutiveFails := 0
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			procState.ProcessMu.Lock()
			running := procState.IsRunning && procState.CurrentProcess != nil
			cmd := procState.CurrentProcess
			procState.ProcessMu.Unlock()
			if !running {
				consecutiveFails = 0
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			shellPath := findShell()
			if shellPath == "" {
				shellPath = "cmd"
			}
			probe := osexec.CommandContext(ctx, shellPath, "/C", cfg.ProbeCmd)
			err := probe.Run()
			cancel()
			if err != nil {
				consecutiveFails++
				log.Printf("健康探针失败 (%d/%d): %v", consecutiveFails, limit, err)
				if consecutiveFails >= limit {
					log.Printf("健康探针连续失败 %d 次, 主动 Kill 子进程", limit)
					if cmd != nil && cmd.Process != nil {
						_ = cmd.Process.Kill()
					}
					consecutiveFails = 0
				}
			} else {
				consecutiveFails = 0
			}
		}
	}()
}

// startProcess 启动并监控子进程
func startProcess(cfg config.Config) error {
	hooks := pmexec.Hooks{
		RunPre:    func() error { return runPreCommand(cfg) },
		CreateCmd: func() (*osexec.Cmd, error) { return createCommand(cfg), nil },
		ApplyEnv: func(cmd *osexec.Cmd) error {
			if env, pathVal := buildChildEnv(cfg.Env); len(env) > 0 {
				cmd.Env = env
				log.Printf("子进程环境PATH: %s", pathVal)
			}
			return nil
		},
		OnStarted: func(cmd *osexec.Cmd) {
			log.Printf("进程启动成功，PID: %d, 路径: %s, 类型: %s", cmd.Process.Pid, cfg.Path, procState.FileType)
			if cfg.Args != "" {
				log.Printf("程序参数: %s", cfg.Args)
			}
		},
		OnStdout: func(r io.ReadCloser) { readProcessOutput(r, "STDOUT", cfg) },
		OnStderr: func(r io.ReadCloser) { readProcessOutput(r, "STDERR", cfg) },
		OnExit: func(exit pmexec.ProcessExit) {
			switch {
			case exit.StoppedByRequest:
				log.Printf("进程被请求停止，code=%d", exit.LastExitCode)
			case exit.LastExitError != "":
				log.Printf("进程异常退出，code=%d, 错误: %v", exit.LastExitCode, exit.LastExitError)
			default:
				log.Printf("进程正常退出，code=%d", exit.LastExitCode)
			}
		},
		Logf:         log.Printf,
		AutoRestart:  cfg.AutoRestart,
		RestartTimes: cfg.RestartTimes,
		RestartDelay: 2 * time.Second,
	}

	return procState.StartManagedProcess(hooks)
}

func stopProcess() error {
	return procState.StopManagedProcess(5 * time.Second)
}

// readProcessOutput 读取并缓存子进程输出
func readProcessOutput(reader io.ReadCloser, prefix string, cfg config.Config) {
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, cfg.LogMaxLineBytes)
	scanner.Buffer(buf, cfg.LogMaxLineBytes)

	for scanner.Scan() {
		logMsg := fmt.Sprintf("[%s] %s", prefix, scanner.Text())
		logMu.Lock()
		logCache = append(logCache, logMsg)
		if len(logCache) > cfg.LogCapacity {
			logCache = logCache[len(logCache)-cfg.LogCapacity:]
		}
		logMu.Unlock()
		if fileLogger != nil {
			fileLogger.WriteLine(logMsg)
		}
		log.Println(logMsg)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("读取进程输出时出错: %v", err)
	}
}