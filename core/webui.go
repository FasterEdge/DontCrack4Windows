// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
package core

import _ "embed"

// webuiHTML 嵌入的单文件 HTML 控制台, 直接通过 w.Write 输出即可
//
//go:embed webui/index.html
var webuiHTML []byte
