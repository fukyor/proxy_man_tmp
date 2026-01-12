package mproxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
)

// 流量计数器
func AddTrafficMonitor(proxy *CoreHttpServer) {
	// 请求阶段
	proxy.HookOnReq().DoFunc(func(req *http.Request, ctx *Pcontext) (*http.Request, *http.Response) {
		// 记录请求头大小
		ctx.TrafficCounter.req_header = GetHeaderSize(req, ctx)
		// 如果有请求体，包装它
		if req.Body != nil {
			// roundripe自动调用req.Body.read读取body
			// roundripe从req的map中读取header
			req.Body = &reqBodyReader{
				ReadCloser: req.Body,
				counter:    ctx.TrafficCounter,
			}
		}
		return req, nil
	})

	// 响应阶段
	proxy.HookOnResp().DoFunc(func(resp *http.Response, ctx *Pcontext) *http.Response {
		if ctx.TrafficCounter == nil {
			return resp
		}
		// 更新请求汇总
		ctx.TrafficCounter.UpdateReqSum()

		// 记录响应头大小
		ctx.TrafficCounter.resp_header = GetHeaderSize(resp, ctx)

		if resp.Body == nil {
			ctx.TrafficCounter.UpdateTotal()
			ctx.Log_P("[流量统计] 上行: %d (header:%d body:%d) | 下行: %d (header:%d body:0) | 总计: %d | %s | %s | %s",
				ctx.TrafficCounter.req_sum, ctx.TrafficCounter.req_header, ctx.TrafficCounter.req_body,
				ctx.TrafficCounter.resp_header,
				ctx.TrafficCounter.resp_header,
				ctx.TrafficCounter.total, ctx.Req.Method, ctx.Req.URL.String(), "未响应")
			return resp
		}

		// 包装响应体
		resp.Body = &TrafficCounter{
			ReadCloser: resp.Body,
			onClose: func(bodyBytes int64) {
				ctx.TrafficCounter.resp_sum = bodyBytes
				ctx.TrafficCounter.UpdateTotal()
				ctx.Log_P("[流量统计] 上行: %d (header:%d body:%d) | 下行: %d (header:%d body:%d) | 总计: %d | %s | %s | %s",
					ctx.TrafficCounter.req_sum, ctx.TrafficCounter.req_header, ctx.TrafficCounter.req_body,
					ctx.TrafficCounter.resp_sum, ctx.TrafficCounter.resp_header, bodyBytes,
					ctx.TrafficCounter.total,
					ctx.Req.Method, ctx.Req.URL.String(), resp.Status)
			},
		}
		return resp
	})
}


func PrintReqHeader(proxy *CoreHttpServer) {
	proxy.HookOnReq().DoFunc(func(req *http.Request, ctx *Pcontext) (*http.Request, *http.Response) {
		dumpBytes, err := httputil.DumpRequest(req, false)
		if err != nil {
			fmt.Println("DumpRequest error:", err)
		} else {
			// 打印出来的就是标准的 HTTP 协议文本
			fmt.Printf("\n=== [DEBUG] Request Dump ===\n%s\n============================\n", dumpBytes)
		}
		return req, nil
	})
}

func PrintRespHeader(proxy *CoreHttpServer) {
	proxy.HookOnResp().OnRespByReq().DoFunc(func(resp *http.Response, ctx *Pcontext) *http.Response {
		dumpBytes, err := httputil.DumpResponse(resp, false)
		if err != nil {
			fmt.Println("DumpResponse error:", err)
		} else {
			// 打印出来的就是标准的 HTTP 协议文本
			fmt.Printf("\n=== [DEBUG] Response Dump ===\n%s\n============================\n", dumpBytes)
		}
		return resp
	})
}
