package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var assets embed.FS

func Files() fs.FS { sub, _ := fs.Sub(assets, "dist"); return sub }
func Built() bool  { b, err := fs.ReadFile(Files(), "index.html"); return err == nil && len(b) > 0 }
