package mproxy

import (
	"net"
)

// tunnelTrafficReader 统计读取的流量
type tunnelTraffic struct {
	halfClosable
	nread int64
	nwrite int64
	printSumOnClose func(int64)
}

func newTunnelTraffic(conn net.Conn, ctx *Pcontext) (*tunnelTraffic, bool) {
	if clientReader, ok := conn.(halfClosable); ok {
		return &tunnelTraffic{
			halfClosable: clientReader,	
			nread: 0,
			nwrite: 0,
		}, true
	}else{
		return nil, false
	}
}

func (r *tunnelTraffic) Read(p []byte) (n int, err error) {
	n, err = r.halfClosable.Read(p)
	r.nread += int64(n)
	return n, err
}


func (w *tunnelTraffic) Write(p []byte) (n int, err error) {
	n, err = w.halfClosable.Write(p)
	w.nwrite += int64(n)
	return n, err
}


type tunnelTrafficNoClosable struct {
	conn net.Conn
	nread int64
	nwrite int64
	printSum func()
}

func newtunnelTrafficNoClosable(conn net.Conn) (*tunnelTrafficNoClosable){
	return &tunnelTrafficNoClosable{
		conn: conn,
		nread: 0,
		nwrite: 0,
		printSumOnClose: func() {

		},
	}
}

func (r *tunnelTrafficNoClosable) Read(p []byte) (n int, err error) {
	n, err = r.conn.Read(p)
	r.nread += int64(n)
	return n, err
}

func (w *tunnelTrafficNoClosable) Write(p []byte) (n int, err error) {
	n, err = w.conn.Write(p)
	return n, err
}

func (c *tunnelTrafficNoClosable) Close() error {
	
	return c.conn.Close()
}




