package mproxy

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"proxy_man/http1parser"
	"proxy_man/signer"
	"strings"
	"sync"
	"context"
	"sync/atomic"
)

type ConnectActionSelecter int

const (
	ConnectAccept = iota
	ConnectReject
	ConnectMitm
	ConnectHijack
	ConnectHTTPMitm
	ConnectProxyAuthHijack
)

var (
	OkConnect = &ConnectAction{Action: ConnectAccept, TLSConfig: TLSConfigFromCA(&Proxy_ManCa)}
	HTTPMitmConnect = &ConnectAction{Action: ConnectHTTPMitm, TLSConfig: TLSConfigFromCA(&Proxy_ManCa)}
)

type ConnectAction struct {
	Action    ConnectActionSelecter
	Hijack    func(req *http.Request, client net.Conn, ctx *Pcontext)
	TLSConfig func(host string, ctx *Pcontext) (*tls.Config, error)
}

// 这是处理tcp连接的核心接口和函数，把他们单独抽象为一个接口
type halfClosable interface {
	net.Conn
	CloseWrite() error
	CloseRead() error
}

var _ halfClosable = (*net.TCPConn)(nil)


func stripPort(s string) string {
	var ix int
	if strings.Contains(s, "[") && strings.Contains(s, "]") {
		// ipv6 address example: [2606:4700:4700::1111]:443
		// strip '[' and ']'
		s = strings.ReplaceAll(s, "[", "")
		s = strings.ReplaceAll(s, "]", "")

		ix = strings.LastIndexAny(s, ":")
		if ix == -1 {
			return s
		}
	} else {
		// ipv4
		ix = strings.IndexRune(s, ':')
		if ix == -1 {
			return s
		}
	}
	return s[:ix]
}

func TLSConfigFromCA(ca *tls.Certificate) func(host string, ctx *Pcontext) (*tls.Config, error) {
	return func(host string, ctx *Pcontext) (*tls.Config, error) {
		var err error
		var cert *tls.Certificate

		hostname := stripPort(host)
		config := defaultTLSConfig.Clone()
		ctx.Log_P("signing for %s", stripPort(host))

		genCert := func() (*tls.Certificate, error) {
			return signer.SignHost(*ca, []string{hostname})
		}
		if ctx.certStore != nil {
			cert, err = ctx.certStore.Fetch(hostname, genCert)
		} else {
			cert, err = genCert()
		}

		if err != nil {
			ctx.WarnP("Cannot sign host certificate with provided CA: %s", err)
			return nil, err
		}

		config.Certificates = append(config.Certificates, *cert)
		return config, nil
	}
}

func (proxy *CoreHttpServer) dial(ctx *Pcontext, network, addr string) (c net.Conn, err error) {
	// if ctx.Dialer != nil {
	// 	return ctx.Dialer(ctx.Req.Context(), network, addr)
	// }

	// if proxy.Tr != nil && proxy.Tr.DialContext != nil {
	// 	return proxy.Tr.DialContext(ctx.Req.Context(), network, addr)
	// }

	return net.Dial(network, addr)
}

func (proxy *CoreHttpServer) connectDial(ctx *Pcontext, network, addr string) (c net.Conn, err error) {
	if proxy.ConnectMutiDial == nil && proxy.ConnectWithReqDial == nil {
		return proxy.dial(ctx, network, addr)
	}

	if proxy.ConnectWithReqDial != nil {
		//return proxy.ConnectDialWithReq(ctx.Req, network, addr)
	}

	return proxy.ConnectMutiDial(network, addr)
}

func httpError(w io.WriteCloser, ctx *Pcontext, err error) {
	if ctx.core_proxy.ConnectionErrHandler != nil {
		ctx.core_proxy.ConnectionErrHandler(w, ctx, err)
	} else {
		errorMessage := err.Error()
		// 直接回写响应头
		errStr := fmt.Sprintf(
			"HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s",
			len(errorMessage),
			errorMessage,
		)
		if _, err := io.WriteString(w, errStr); err != nil {
			ctx.WarnP("Error responding to client: %s", err)
		}
	}
	if err := w.Close(); err != nil {
		ctx.WarnP("Error closing client connection: %s", err)
	}
}

func copyAndClose(ctx *Pcontext, dst, src halfClosable, wg *sync.WaitGroup) {
	_, err := io.Copy(dst, src)
	if err != nil && !errors.Is(err, net.ErrClosed) {
		ctx.WarnP("Error copying to client: %s", err.Error())
	}

	_ = dst.CloseWrite()
	_ = src.CloseRead()
	wg.Done()
}

// 缺陷一：
// 是直接调用close会导致io.Copy报错连接异常关闭，比如client FIN->proxy，proxy会立刻close target
// 导致target->proxy端的管道突然关闭，io.Copy报错
// 缺陷二：
// 没有半关闭的情况下，FIN_wait2无法收到额外的结束消息。
func copyOrWarn(ctx *Pcontext, dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	if err != nil && errors.Is(err, net.ErrClosed) {
		// Discard closed connection errors,在没有半关闭的情况下，并发时连接关闭是正常的，不可控的
		err = nil
	} else if err != nil {
		ctx.WarnP("Error copying to client: %s", err)
	}
	return err
}


func (proxy *CoreHttpServer) MyHttpsHandle(w http.ResponseWriter, r *http.Request){
		ctxt := &Pcontext{
		core_proxy: proxy,
		Req: r,
		TrafficCounter: &TrafficCounter{},
		Session: atomic.AddInt64(&proxy.sess, 1),
	}
	
	// 创建hijack
	hijk, ok := w.(http.Hijacker) 
	if !ok {
		panic("httpsSever not support hijack")
	}

	connFromClinet, _, err := hijk.Hijack()

	if err != nil {
		panic("hijack connection fail" + err.Error())
    }
	ctxt.Log_P("处理器数量Have %d CONNECT handlers", len(proxy.httpsHandlers))

	strategy, host := OkConnect, r.URL.Host

	// 
	for i, h := range proxy.httpsHandlers {
		new_strategy, newhost := h.HandleConnect(host, ctxt)
		// 和resphook一样返回nil说明没有匹配上条件，如果不等于nil则匹配上了就替换新的策略
		if new_strategy != nil {
			strategy, host = new_strategy, newhost
			ctxt.Log_P("Excuted %dth handler: %v %s",i, strategy, host)
			break
		}
	}

	switch strategy.Action {
	case ConnectAccept:
		if !Port.MatchString(host) {
			host += ":80"
		}
		connRemoteSite, err := proxy.connectDial(ctxt, "tcp", host)
		if err != nil {
			ctxt.WarnP("拨号获取套接字错误Error dialing to %s: %s", host, err.Error())
			httpError(connRemoteSite, ctxt, err) // 如果出错手动关闭连接
			return
		}
		ctxt.Log_P("Accepting CONNECT to %s", host)

		_, err = connFromClinet.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n"))
		if err != nil{
			ctxt.WarnP("200 Connection fail established")
			return
		}
		// 断言是否实现了半连接接口，net.conn一般是自动实现了的。实现半连接接口就是实现用系统调用shutdown来对socket进行半关闭
		targetTCP, targetOK := connRemoteSite.(halfClosable)
		proxyClientTCP, clientOK := connFromClinet.(halfClosable)
		if targetOK && clientOK {
			go func() {
				var wg sync.WaitGroup
				wg.Add(2)
				// 向remote写完所有数据后，proxy关闭写发出FIN，remote收到FIN，完成读取后，调用close回复FIN，完成挥手
				go copyAndClose(ctxt, targetTCP, proxyClientTCP, &wg) 
				// 向client写完所有数据后，proxy关闭写发出FIN，client收到FIN，完成读取后，调用close回复FIN，完成挥手
				go copyAndClose(ctxt, proxyClientTCP, targetTCP, &wg)
				wg.Wait()
				// 最后调用close保证了连接能够正常断开
				proxyClientTCP.Close()
				targetTCP.Close()
			}()
		}else{
			go func() {
				err := copyOrWarn(ctxt, connRemoteSite, connFromClinet)
				if err != nil && proxy.ConnectionErrHandler != nil {
					proxy.ConnectionErrHandler(connFromClinet, ctxt, err)
				}
				// 关闭server端，因为server对client的关闭是无法感知的，需要proxy通知
				_ = connRemoteSite.Close()
			}()

			go func() {
				_ = copyOrWarn(ctxt, connFromClinet, connRemoteSite)
				_ = connFromClinet.Close()
			}()
		}
	// 调用用户自己的劫持逻辑，暂时没想到怎么使用	
	// case ConnectHijack:
	// 	strategy.Hijack(r, connFromClinet, ctxt)
	// http隧道透传，有利于server和client的HTTP1.1连接复用。减轻proxy压力。
	case ConnectHTTPMitm:
		_, _ = connFromClinet.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
		ctxt.Log_P("HTTP Tunnel established, http MITM")

		var connRemoteSite net.Conn
		var remote_res *bufio.Reader

		// 构建自定义RquestReader
		reqM := http1parser.NewRequestReader(proxy.PreventParseHeader, connFromClinet)
		
		// 模拟阻塞read，连续维持隧道，只要client不发送FIN，就保持
		for !reqM.IsEOF() {
			// 从client个性化提取请求头部
			req, err := reqM.ReadRequest()
			if err != nil && !errors.Is(err, io.EOF) {
				ctxt.WarnP("http协议解析错误http parser errror: %+#v", err)
			}
			if err != nil {
				return
			}
			requestOk := func(req *http.Request) bool {

				// 模仿标准库为request绑定上下文
				requestContext, CancelR := context.WithCancel(req.Context())
				req = req.WithContext(requestContext)
				defer CancelR()
				req.RemoteAddr = r.RemoteAddr
				ctxt.Log_P("req %v", r.Host)
				ctxt.Req = req

				req, resp := proxy.filterRequest(req, ctxt)
				if resp != nil {
					if connRemoteSite == nil {
						// 和target建立tcp连接
						connRemoteSite, err = proxy.connectDial(ctxt, "tcp", host)
						if err != nil {
							ctxt.WarnP("MITM Error dialing to %s: %s", host, err.Error())
							return false
						}
						remote_res = bufio.NewReader(connRemoteSite)
					}
				}

				// 向target发起请求
				if err := req.Write(connRemoteSite); err != nil {
					httpError(connFromClinet, ctxt, err)
					return false
				}
				// 根据req类型，接受响应
				resp, err = func() (*http.Response, error) {
					defer req.Body.Close()
					return http.ReadResponse(remote_res, req)
				}()
				if err != nil {
					httpError(connFromClinet, ctxt, err)
					return false
				}

				// 响应处理
				resp = proxy.filterResponse(resp, ctxt)
				defer resp.Body.Close()

				// 将resp的响应头和body完整发送给客户端
				err = resp.Write(connFromClinet)
				if err != nil {
					httpError(connFromClinet, ctxt, err)
					return false
				}

				return true

			}
			if !requestOk(req) {
				break
			}
		}
	}
}