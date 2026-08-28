package core

import (
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 通过魔数检查是不是 Windows 可执行文件 (PE / MZ header)
func isBinaryExecutable(data []byte) bool {
	// Windows PE 文件: 起始为 'M' 'Z' (IMAGE_DOS_HEADER.e_magic)
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		return true
	}
	// 兜底: 在前 100 字节内出现空字节 → 可能是二进制
	for _, b := range data[:min(len(data), 100)] {
		if b == 0 {
			return true
		}
	}
	return false
}

// 基于文件头/扩展名识别类型，用于决定如何启动
func detectFileType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	content := string(buffer[:n])

	// PowerShell 脚本: 通常以 #Requires / <# / # 开头
	if strings.HasPrefix(content, "#Requires") || strings.HasPrefix(strings.TrimSpace(content), "# ") {
		if strings.HasSuffix(strings.ToLower(path), ".ps1") {
			return "powershell_script", nil
		}
	}

	// 批处理: 通常以 @echo off / rem 开头
	lower := strings.ToLower(content)
	if strings.HasPrefix(lower, "@echo") || strings.HasPrefix(lower, "rem ") || strings.HasPrefix(lower, "::") {
		if ext := strings.ToLower(filepath.Ext(path)); ext == ".bat" || ext == ".cmd" {
			return "batch_script", nil
		}
	}

	// 通过文件扩展名检查
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".bat", ".cmd":
		return "batch_script", nil
	case ".ps1":
		return "powershell_script", nil
	case ".exe", ".com":
		return "binary_executable", nil
	default:
		if isBinaryExecutable(buffer[:n]) {
			return "binary_executable", nil
		}
		return "unknown", nil
	}
}

// 校验请求中携带的 password，如果未配置密码则直接通过
func checkPassword(r *http.Request, expected string) error {
	if expected == "" {
		return nil
	}
	pw := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		pw = strings.TrimPrefix(auth, "Bearer ")
	} else if v := r.Header.Get("X-DontCrack-Password"); v != "" {
		pw = v
	} else {
		pw = r.URL.Query().Get("password")
	}
	if subtle.ConstantTimeCompare([]byte(pw), []byte(expected)) == 1 {
		return nil
	}
	return errors.New("unauthorized")
}

// findShell 在 Windows 上寻找可用的 shell 可执行文件。
// 优先使用 cmd.exe (兼容所有 Windows)，再 powershell.exe（可选）。
// 注意: pwsh.exe 不在 System32 下，它装在 %ProgramFiles%\PowerShell\<ver>\pwsh.exe，
//       所以这里不做硬编码探测，需要时靠 createCommand 的 os.Stat 回退 + PATH。
// 都找不到时回退到 "cmd"，由 PATH 决定。
func findShell() string {
	candidates := []string{
		`C:\Windows\System32\cmd.exe`,
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}