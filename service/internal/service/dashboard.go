package service

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var dashboardFiles embed.FS

func newDashboardHandler() http.Handler {
	contents, err := fs.Sub(dashboardFiles, "web")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(contents))
	return http.StripPrefix("/yinqiao/ui/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; frame-ancestors *")
		writer.Header().Set("Referrer-Policy", "same-origin")
		files.ServeHTTP(writer, request)
	}))
}
