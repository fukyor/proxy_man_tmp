package mproxy

import (
	"net/http"
	"sync/atomic"
	"time"
)

var exchangeIDCounter int64

// HttpExchange 代表 MITM 模式下一次完整的 HTTP 请求-响应交互
type HttpExchange struct {
	ID        int64            `json:"id"`
	SessionID int64            `json:"sessionId"`
	ParentID  int64            `json:"parentId"`
	Time      int64            `json:"time"`
	Request   RequestSnapshot  `json:"request"`
	Response  ResponseSnapshot `json:"response"`
	Duration  int64            `json:"duration"`
	Error     string           `json:"error,omitempty"`
}

type RequestSnapshot struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Host    string              `json:"host"`
	Header  map[string][]string `json:"header"`
	SumSize int64               `json:"sumSize"`
}

type ResponseSnapshot struct {
	StatusCode int                 `json:"statusCode"`
	Status     string              `json:"status"`
	Header     map[string][]string `json:"header"`
	SumSize    int64               `json:"sumSize"`
}

var GlobalExchangeChan = make(chan *HttpExchange, 1000)

// ========== ExchangeCapture：封装捕获状态 ==========

// ExchangeCapture 封装单次请求的捕获状态
type ExchangeCapture struct {
	startTime time.Time
	reqSnap   RequestSnapshot
	parentID  int64
	skip      bool
	err       error
	sent      bool // 防止重复发送
}

// StartCapture 初始化捕获，在 Pcontext 创建后调用
func (ctx *Pcontext) StartCapture(parentSession int64) {
	ctx.exchangeCapture = &ExchangeCapture{
		startTime: time.Now(),
		parentID:  parentSession,
	}
}

// CaptureRequest 捕获请求快照，在 filterRequest 之后调用
func (ctx *Pcontext) CaptureRequest(req *http.Request) {
	if ctx.exchangeCapture == nil {
		return
	}
	ctx.exchangeCapture.reqSnap = RequestSnapshot{
		Method: req.Method,
		URL:    req.URL.String(),
		Host:   req.Host,
		Header: cloneHeader(req.Header),
	}
}

// SkipCapture 标记跳过捕获（用于 WebSocket）
func (ctx *Pcontext) SkipCapture() {
	if ctx.exchangeCapture != nil {
		ctx.exchangeCapture.skip = true
	}
}

// SetCaptureError 记录错误
func (ctx *Pcontext) SetCaptureError(err error) {
	if ctx.exchangeCapture != nil && err != nil {
		ctx.exchangeCapture.err = err
	}
}

// SendExchange 发送 Exchange 到全局通道，在响应完成后自动调用
// 这个方法会被 respBodyReader.onClose 触发
func (ctx *Pcontext) SendExchange() {
	cap := ctx.exchangeCapture
	if cap == nil || cap.skip || cap.sent {
		return
	}
	cap.sent = true

	exchange := &HttpExchange{
		ID:        atomic.AddInt64(&exchangeIDCounter, 1),
		SessionID: ctx.Session,
		ParentID:  cap.parentID,
		Time:      cap.startTime.UnixMilli(),
		Request:   cap.reqSnap,
		Duration:  time.Since(cap.startTime).Milliseconds(),
	}

	// 从 TrafficCounter 读取请求总大小（头部+Body）
	if ctx.TrafficCounter != nil {
		exchange.Request.SumSize = ctx.TrafficCounter.req_sum
	}

	// 从 ctx.Resp 读取响应信息
	if ctx.Resp != nil {
		exchange.Response = ResponseSnapshot{
			StatusCode: ctx.Resp.StatusCode,
			Status:     ctx.Resp.Status,
			Header:     cloneHeader(ctx.Resp.Header),
		}
		if ctx.TrafficCounter != nil {
			exchange.Response.SumSize = ctx.TrafficCounter.resp_sum
		}
	}

	if cap.err != nil {
		exchange.Error = cap.err.Error()
	}

	// 非阻塞发送
	select {
	case GlobalExchangeChan <- exchange:
	default:
	}
}

func cloneHeader(h http.Header) map[string][]string {
	if h == nil {
		return nil
	}
	result := make(map[string][]string, len(h))
	for k, v := range h {
		result[k] = append([]string{}, v...)
	}
	return result
}