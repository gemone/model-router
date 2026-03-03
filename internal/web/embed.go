//go:build embed
// +build embed

package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS 返回嵌入的文件系统 (兼容 Fiber)
// Returns error if the embedded filesystem is not available
func FS() (fs.FS, error) {
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded files: %w", err)
	}
	return fsys, nil
}
