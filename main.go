package main

import (
	//"encoding/base64"
	"flag"
	"log"
	"net/http"
    "proxy_man/mproxy"
)

func main(){
    verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":8080", "proxy listen address")
	flag.Parse()
    proxy := mproxy.NewCoreHttpSever()
    proxy.Verbose = *verbose
    log.Fatal(http.ListenAndServe(*addr, proxy))
}