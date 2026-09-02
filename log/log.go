// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
package log

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	rotateSizeBytes = 10 * 1024 * 1024 // 10MB
	timeLayout      = "20060102-150405"
)

// 供内存日志缓存使用。
type LogCache struct {
	logsCache []string
	logsMu    sync.Mutex
}

// 负责本地日志落盘与过期清理。
type FileLogger struct {
	mu       sync.Mutex
	dir      string
	procName string
	lifeDays int

	curFile *os.File
	curSize int64
	curDay  string
	seq     int
}

// 创建日志目录并准备第一个文件。
func NewFileLogger(baseDir, procName string, lifeDays int) (*FileLogger, error) {
	dir := filepath.Join(baseDir, procName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	fl := &FileLogger{dir: dir, procName: procName, lifeDays: lifeDays}
	if err := fl.rotateLocked(time.Now()); err != nil {
		return nil, err
	}
	fl.cleanupLocked(time.Now())
	return fl, nil
}

// 写入一行日志并根据规则自动轮转。
func (f *FileLogger) WriteLine(line string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	if f.curFile == nil || f.curDay != now.Format("20060102") || f.curSize >= rotateSizeBytes {
		_ = f.rotateLocked(now)
	}

	if f.curFile == nil {
		return
	}
	data := []byte(line)
	if !strings.HasSuffix(line, "\n") {
		data = append(data, '\n')
	}
	n, _ := f.curFile.Write(data)
	f.curSize += int64(n)

	if f.curSize >= rotateSizeBytes {
		_ = f.rotateLocked(time.Now())
	}
}

// Close 关闭当前文件。
func (f *FileLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.curFile != nil {
		err := f.curFile.Close()
		f.curFile = nil
		return err
	}
	return nil
}

// 在持锁状态下生成新文件。
func (f *FileLogger) rotateLocked(now time.Time) error {
	if f.curFile != nil {
		_ = f.curFile.Close()
	}
	f.curDay = now.Format("20060102")
	f.seq++
	name := fmt.Sprintf("%s-%s-%02d.log", f.procName, now.Format(timeLayout), f.seq)
	path := filepath.Join(f.dir, name)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}
	f.curFile = file
	f.curSize = 0

	f.cleanupLocked(now)
	return nil
}

// 删除超过 lifeDays 的日志文件（按文件名中的日期判断）。
func (f *FileLogger) cleanupLocked(now time.Time) {
	if f.lifeDays <= 0 {
		return
	}
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return
	}
	expireBefore := now.AddDate(0, 0, -f.lifeDays)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		ts, ok := parseTimestampFromName(e.Name(), f.procName)
		if !ok {
			continue
		}
		if ts.Before(expireBefore) {
			_ = os.Remove(filepath.Join(f.dir, e.Name()))
		}
	}
}

// 提取文件名中的时间戳。
func parseTimestampFromName(name, procName string) (time.Time, bool) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	lastDash := strings.LastIndex(base, "-")
	if lastDash <= 0 {
		return time.Time{}, false
	}
	seqPart := base[lastDash+1:]
	if _, err := strconv.Atoi(seqPart); err != nil {
		return time.Time{}, false
	}
	base = base[:lastDash]

	lastDash = strings.LastIndex(base, "-")
	if lastDash <= 0 {
		return time.Time{}, false
	}
	tsPart := base[lastDash+1:]
	prefix := base[:lastDash]
	if prefix != procName {
		return time.Time{}, false
	}

	t, err := time.Parse(timeLayout, tsPart)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// 便于测试或外部查询当前日志文件列表。
func (f *FileLogger) WalkFiles(fn func(fs.DirEntry)) {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fn(e)
	}
}
