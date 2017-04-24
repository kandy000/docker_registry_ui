package main

import (
	"fmt"
	"net/http"
)

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
)

func HumanSize(size int) string {
	switch {
	case size > GiB:
		return fmt.Sprintf("%.2f GiB", float32(size)/float32(GiB))
	case size > MiB:
		return fmt.Sprintf("%.2f MiB", float32(size)/float32(MiB))
	case size > KiB:
		return fmt.Sprintf("%.2f KiB", float32(size)/float32(KiB))
	default:
		return fmt.Sprintf("%.0f", float32(size))
	}
}

func LoadWebServer(addr string) {
	http.HandleFunc("/", IndexHandler)
	http.HandleFunc("/auth", AuthHandler)
	http.HandleFunc("/view", ViewIndexHandler)
	http.HandleFunc("/view/repo", ViewRepoHandler)
	http.HandleFunc("/view/image", ViewImageHandler)

	http.ListenAndServe(addr, nil)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("Hallo"))
}