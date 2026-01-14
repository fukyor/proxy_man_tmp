package mproxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"proxy_man/signer"
	"strings"
	"sync/atomic"
	"io"
	"fmt"
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
)

type ConnectAction struct {
	Action    ConnectActionSelecter
	Hijack    func(req *http.Request, client net.Conn, ctx *Pcontext)
	TLSConfig func(host string, ctx *Pcontext) (*tls.Config, error)
}

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

func (proxy *CoreHttpServer) MyHttpsHandle(w http.ResponseWriter, r *http.Request){
		ctxt := &Pcontext{
		core_proxy: proxy,
		Req: r,
		TrafficCounter: &TrafficCounter{},
		Session: atomic.AddInt64(&proxy.sess, 1),
	}
	
	// 创建hijack
	hijk, ok := w.(http.Hijacker) 
	if ok {
		panic("httpsSever not support hijack")
	}

	connFromClinet, _, err := hijk.Hijack()

	if err != nil {
		panic("hijack connection fail" + err.Error())
    }
	ctxt.Log_P("Running %d CONNECT handlers", len(proxy.httpsHandlers))

	strategy, host := OkConnect, r.URL.Host
	for i, h := range proxy.httpsHandlers {
		new_strategy, newhost := h.HandleConnect(host, ctxt)

		// 和resphook一样返回nil说明没有匹配上条件，如果匹配上了就替换新的策略
		if new_strategy != nil {
			strategy, host = new_strategy, newhost
			ctxt.Log_P("on %dth handler: %v %s",i, strategy, host)
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
			ctxt.WarnP("Error dialing to %s: %s", host, err.Error())
			httpError(connRemoteSite, ctxt, err) // 手动关闭连接
			return
		}
		ctxt.Log_P("Accepting CONNECT to %s", host)

		_, err = connFromClinet.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n"))
		if err != nil{
			ctxt.WarnP("200 Connection fail established")
			return
		}
		

	}


}