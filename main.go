package main

import (
	"flag"
	"log"
	"net/http"
	"proxy_man/mproxy"
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
	proxy.KeepHeader = false  // 不保留代理头部
	mproxy.PrintReqHeader(proxy)
	mproxy.PrintRespHeader(proxy)
	mproxy.AddTrafficMonitor(proxy)
	//mproxy.StatusChange(proxy)
	mproxy.MitmMode(proxy)

	s := http.Server{
		Addr: *addr,
		Handler: proxy,
	}
	if err := s.ListenAndServe(); err != nil{
		log.Fatal("服务器错误", err)
	}
	
}