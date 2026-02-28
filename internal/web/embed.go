//go:build embed
// +build embed

package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS 返回嵌入的文件系统 (兼容 Fiber)
func FS() fs.FS {
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return fsys
}
