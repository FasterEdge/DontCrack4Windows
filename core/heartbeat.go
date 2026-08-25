package core

type HeartbeatInfo struct {
	Version          string   `json:"version"`       // 当前版本
	State            string   `json:"state"`         // 当前进程状态
	Info             string   `json:"info"`          // 传递信息
	Timestamp        string   `json:"timestamp"`     // 时间戳
	Logs             []string `json:"logs"`          // 当前的一批日志
	ProcessPID       int      `json:"process_pid"`   // 进程PID
	ProcessPath      string   `json:"process_path"`  // 进程路径
	RestartCount     int      `json:"restart_count"` // 重启次数
	FileType         string   `json:"file_type"`     // 文件类型
	LastExitCode     int      `json:"last_exit_code,omitempty"`
	LastExitTime     string   `json:"last_exit_time,omitempty"`
	LastExitBySignal bool     `json:"last_exit_by_signal,omitempty"`
	LastExitError    string   `json:"last_exit_error,omitempty"`
	ProgramArgs      string   `json:"program_args,omitempty"`
	ExtraEnvRaw      string   `json:"extra_env_raw,omitempty"`
}