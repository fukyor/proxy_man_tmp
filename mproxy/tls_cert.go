package mproxy

import (
	"crypto/tls"
)

var tlsIgnoreVerify = &tls.Config{
	InsecureSkipVerify: true,
}