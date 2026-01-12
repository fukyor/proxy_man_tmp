package main

import (
	"flag"
	"log"
	"net/http"
	"proxy_man/mproxy"
	"net/http/httputil"
	"fmt"
)

func main() {
	verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":8080", "proxy listen address")
	flag.Parse()
	proxy := mproxy.NewCoreHttpSever()
	proxy.Verbose = *verbose

	proxy.HookOnReq(mproxy.UrlRegHook("baidu\\.c*")).DoFunc(func(req *http.Request, ctx *mproxy.Pcontext) (*http.Request, *http.Response) {
		dumpBytes, err := httputil.DumpRequest(req, true)
		if err != nil {
        fmt.Println("DumpRequest error:", err)
		} else {
			// 打印出来的就是标准的 HTTP 协议文本
			fmt.Printf("\n=== [DEBUG] Request Dump ===\n%s\n============================\n", dumpBytes)
		}
		return req, nil
	})

	proxy.OnResponse(mproxy.ContentTypeHook("text/html")).OnRespByReq(mproxy.UrlRegHook("baidu\\.c*")).DoFunc(func(resp *http.Response, ctx *mproxy.Pcontext) *http.Response {
		dumpBytes, err := httputil.DumpResponse(resp, true)
		if err != nil {
			fmt.Println("DumpResponse error:", err)
		} else {
			// 打印出来的就是标准的 HTTP 协议文本
			fmt.Printf("\n=== [DEBUG] Response Dump ===\n%s\n============================\n", dumpBytes)
		}
		return resp
	})

	proxy.OnResponse().OnRespByReq().DoFunc(func(resp *http.Response, ctx *mproxy.Pcontext) *http.Response {
		resp.Header.Del("Content-Length")
		resp.Header.Set("Transfer-Encoding", "chunked")
		return resp
	})

	s := http.Server{
		Addr: *addr,
		Handler: proxy,
	}
	if err := s.ListenAndServe(); err != nil{
		log.Fatal("服务器错误", err)
	}
	
}