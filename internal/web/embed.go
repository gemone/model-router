//go:build embed
// +build embed

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var distFS embed.FS

// FS 返回嵌入的文件系统
func FS() http.FileSystem {
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}
