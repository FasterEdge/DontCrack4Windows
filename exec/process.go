package exec

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// 进程的结构体
type Process struct {
	CurrentProcess *exec.Cmd  // 当前进程对象
	ProcessMu      sync.Mutex // 进程锁
	IsRunning      bool       // 进程是否在运行
	RestartCount   int        // 已重试次数
	FileType       string     // 可执行文件类型
	Pid            int        // 进程ID
	ExitInfo       ProcessExit
	stoppedByReq   bool
}

// 进程退出信息
type ProcessExit struct {
	LastExitCode     int       // 进程最后退出代码
	LastExitTime     time.Time // 进程最后退出时间
	LastExitBySignal bool      // 进程是否被信号终止
	LastExitError    string    // 进程最后退出的错误信息
	StoppedByRequest bool      // 进程是否是被请求停止的
}

// Hooks 聚合了进程启动/退出时需要的回调和策略
type Hooks struct {
	RunPre       func() error
	CreateCmd    func() (*exec.Cmd, error)
	ApplyEnv     func(*exec.Cmd) error
	OnStarted    func(*exec.Cmd)
	OnStdout     func(io.ReadCloser)
	OnStderr     func(io.ReadCloser)
	OnExit       func(ProcessExit)
	Logf         func(string, ...interface{})
	AutoRestart  bool
	RestartTimes int
	RestartDelay time.Duration
}

// StartManagedProcess 启动子进程并在内部启动等待/回调逻辑
func (p *Process) StartManagedProcess(h Hooks) error {
	p.ProcessMu.Lock()
	defer p.ProcessMu.Unlock()

	if p.IsRunning && p.CurrentProcess != nil {
		return fmt.Errorf("进程已在运行，PID: %d", p.CurrentProcess.Process.Pid)
	}
	p.stoppedByReq = false

	if h.RunPre != nil {
		if err := h.RunPre(); err != nil {
			return err
		}
	}

	var err error
	var cmd *exec.Cmd
	if h.CreateCmd != nil {
		cmd, err = h.CreateCmd()
		if err != nil {
			return err
		}
	}
	if cmd == nil {
		return fmt.Errorf("CreateCmd 未提供")
	}
	if h.ApplyEnv != nil {
		if err := h.ApplyEnv(cmd); err != nil {
			return err
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建stdout管道失败: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建stderr管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动进程失败: %v", err)
	}

	p.CurrentProcess = cmd
	p.IsRunning = true
	p.Pid = cmd.Process.Pid
	if h.OnStarted != nil {
		h.OnStarted(cmd)
	}
	if h.OnStdout != nil {
		go h.OnStdout(stdout)
	}
	if h.OnStderr != nil {
		go h.OnStderr(stderr)
	}

	go p.monitor(cmd, h)
	return nil
}

// StopManagedProcess 尝试优雅停止进程
// Windows 特殊性:
//   - syscall.SIGTERM 在 Windows 上不被支持 (会返回 error)
//   - os.Interrupt 对应 CTRL_BREAK_EVENT，可用于同控制台组的子进程
//   - 兜底: 超时后调用 Kill() 强制 TerminateProcess
// 注意：cmd.Wait() 只能被调用一次。monitor goroutine 已经持有 Wait 的唯一所有权，
// 这里只负责发信号 + 等待 IsRunning 被 monitor 清掉，超时后 Kill。
func (p *Process) StopManagedProcess(timeout time.Duration) error {
	p.ProcessMu.Lock()
	if !p.IsRunning || p.CurrentProcess == nil {
		p.ProcessMu.Unlock()
		return fmt.Errorf("进程未运行")
	}
	// 标记为"主动停止"，monitor 在 OnExit 后会跳过自动重启
	p.stoppedByReq = true
	cmd := p.CurrentProcess
	p.ProcessMu.Unlock()

	// Windows 上发送 CTRL_BREAK_EVENT (os.Interrupt) 给同控制台组的子进程
	// 失败也无所谓，超时后会强杀
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		// ignore
	}

	// 轮询等待 monitor 把 IsRunning 置为 false（说明进程已退出并被 Wait 收走）
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.ProcessMu.Lock()
		running := p.IsRunning && p.CurrentProcess != nil
		p.ProcessMu.Unlock()
		if !running {
			return nil // monitor 已完成清理
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 超时仍未退出 → 强制 Kill
	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("强制终止进程失败: %v", err)
	}
	// Kill 后再等 monitor 收走
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		p.ProcessMu.Lock()
		running := p.IsRunning && p.CurrentProcess != nil
		p.ProcessMu.Unlock()
		if !running {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("进程在强杀后仍未被回收")
}

// monitor 等待进程退出并回调 OnExit；如果需要，自动重启
func (p *Process) monitor(cmd *exec.Cmd, h Hooks) {
	err := cmd.Wait()

	p.ProcessMu.Lock()
	p.IsRunning = false
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	exitInfo := ProcessExit{
		LastExitTime: time.Now(),
		LastExitCode: 0,
	}
	if err != nil {
		exitInfo.LastExitError = err.Error()
		if ee, ok := err.(*exec.ExitError); ok {
			exitInfo.LastExitCode = ee.ExitCode()
		}
	}
	exitInfo.StoppedByRequest = p.stoppedByReq
	p.ExitInfo = exitInfo
	p.ProcessMu.Unlock()

	if h.OnExit != nil {
		h.OnExit(exitInfo)
	}

	// 手动停止(HTTP /shutdown 或管理器关机触发的停止)后不再自动重启。
	// 否则 auto-restart 开启时 /shutdown 杀掉当前进程后 monitor 又把它拉起来，
	// 造成"关都关不掉"的暗病。
	if exitInfo.StoppedByRequest {
		return
	}

	if h.AutoRestart && (h.RestartTimes < 0 || p.RestartCount < h.RestartTimes) {
		p.RestartCount++
		delay := h.RestartDelay
		if delay == 0 {
			delay = 2 * time.Second
		}
		if h.Logf != nil {
			h.Logf("准备重启进程，当前重试次数: %d/%d", p.RestartCount, h.RestartTimes)
		}
		time.Sleep(delay)
		_ = p.StartManagedProcess(h)
	} else if h.AutoRestart && h.Logf != nil {
		h.Logf("已达到最大重试次数 %d，停止自动重启 (pid=%d)", h.RestartTimes, pid)
	}
}
