package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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

const (
	registryHttpsKeyFile  = "/etc/registry_https.key"
	registryHttpsCertFile = "/etc/registry_https.crt"
)

func LoadWebServer(registryBackendAddr string) error {
	registryBackendURL, err := url.Parse(registryBackendAddr)
	if err != nil {
		return fmt.Errorf("Docker Registry 地址解析失败: %s", err)
	}

	//Registry需要的认证请求处理
	http.HandleFunc("/auth", AuthHandler)

	//代理Registry的请求
	registryProxy := httputil.NewSingleHostReverseProxy(registryBackendURL)
	http.HandleFunc("/v2/", registryProxy.ServeHTTP)
	http.HandleFunc("/v1/", registryProxy.ServeHTTP)

	//UI界面API
	if enable, _ := GetBoolConfig("enable_ui"); enable {
		http.HandleFunc("/api/repo/index.json", ApiRepoIndexHandler)
		http.HandleFunc("/api/repo/show.json", ApiRepoShowHandler)
		http.HandleFunc("/api/image/show.json", ApiImageShowHandler)
		http.HandleFunc("/api/image/delete.json", ApiImageDeleteHandler)
	}

	//如果提供了HTTPS的密钥对，则需要监听HTTPS
	registryHttpsKeyBlock, _ := GetStringConfig("registry_https_key")
	registryHttpsCertBlock, _ := GetStringConfig("registry_https_cert")
	if registryHttpsKeyBlock != "" || registryHttpsCertBlock != "" {
		httpsListenAddr := os.Getenv("HTTPS_LISTEN_ADDR")
		if httpsListenAddr == "" {
			httpsListenAddr = ":443"
		}

		ioutil.WriteFile(registryHttpsKeyFile, []byte(registryHttpsKeyBlock), 0755)
		ioutil.WriteFile(registryHttpsCertFile, []byte(registryHttpsCertBlock), 0755)

		log.Println("在", httpsListenAddr, "启动HTTPS服务")

		go http.ListenAndServeTLS(httpsListenAddr, registryHttpsCertFile, registryHttpsKeyFile, nil)
	}

	//默认监听HTTP
	httpListenAddr := os.Getenv("HTTP_LISTEN_ADDR")
	if httpListenAddr == "" {
		httpListenAddr = ":80"
	}

	log.Println("在", httpListenAddr, "启动HTTP服务")
	return http.ListenAndServe(httpListenAddr, nil)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("未知请求", r.URL, "重定向到/view")
	http.Redirect(w, r, "/view", http.StatusMovedPermanently)
}

func renderInfo(w http.ResponseWriter, info interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data := map[string]interface{}{
		"success": true,
		"info":    info,
	}

	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderSuccess(w http.ResponseWriter) {
	renderInfo(w, nil)
}

func renderError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	data := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}
	renderErr := json.NewEncoder(w).Encode(data)
	if renderErr != nil {
		http.Error(w, renderErr.Error(), http.StatusInternalServerError)
	}
}
