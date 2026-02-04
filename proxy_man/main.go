package main

import (
	"flag"
	"log"
	"net/http"
	"proxy_man/minio"
	"proxy_man/mproxy"
	"proxy_man/proxysocket"
	// "net/http/httputil"
	// "fmt"
)

func main() {
	verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":8080", "proxy listen address")
	flag.Parse()
	proxy := mproxy.NewCoreHttpSever()
	proxy.Verbose = *verbose
	proxy.AllowHTTP2 = false
	proxy.KeepAcceptEncoding = false
	proxy.PreventParseHeader = false
	proxy.KeepDestHeaders = false
	proxy.ConnectMaintain = false

	// 使用 LogCollector 包装原有 Logger
	proxy.Logger = mproxy.NewLogCollector(proxy.Logger)

	// 初始化 MinIO
	minioConfig := minio.Config{
		Endpoint:        "127.0.0.1:9000",
		AccessKeyID:     "root",
		SecretAccessKey: "12345678",
		UseSSL:          false,
		Bucket:          "bodydata",
		Enabled:         true,
	}
	client, err := minio.NewClient(minioConfig)
	if err != nil {
		log.Printf("警告: MinIO 初始化失败: %v，Body 捕获功能将被禁用", err)
		return
	} else {
		minio.GlobalClient = client
		log.Printf("MinIO 存储已启用: %s/%s", minioConfig.Endpoint, minioConfig.Bucket)
	}

	// mproxy.PrintReqHeader(proxy)
	// mproxy.PrintRespHeader(proxy)
	mproxy.AddTrafficMonitor(proxy)
	//mproxy.StatusChange(proxy)
	mproxy.HttpMitmMode(proxy)
	//mproxy.HttpsMitmMode(proxy)


	// 启动 WebSocket 控制服务
	ws := &proxysocket.WebsocketServer{
		Proxy: proxy,
		Addr: ":8000",
		Secret: "123",	
	}
	if !ws.StartControlServer() {
		log.Fatal("websocket server启动失败")
	}

	s := http.Server{
		Addr: *addr,
		Handler: proxy,
	}
	if err := s.ListenAndServe(); err != nil{
		log.Fatal("服务器错误", err)
	}

}