package mproxy

import (
	"crypto/tls"
)

var Proxy_ManCa tls.Certificate

var defaultTLSConfig = &tls.Config{
	InsecureSkipVerify: true,
}
