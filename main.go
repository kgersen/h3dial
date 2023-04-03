package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptrace"

	"github.com/quic-go/quic-go/http3"
)

var url = "https://cloudflare-quic.com/"

func main() {
	// quic-go -  call this program with
	fmt.Println("QUIC-GO:")
	// is there a way to get the server real IP with quic-go ?
	// for now use logging with this env variable
	//    QUIC_GO_LOG_LEVEL="INFO" go run main.go
	// this will display the server IP.
	client := http.Client{Transport: &http3.RoundTripper{}}
	request, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	if resp == nil {
		log.Fatal("nil response")
	}
	fmt.Println("  got status: ", resp.Status)
	fmt.Println("  proto: ", resp.Proto)

	// net/http
	fmt.Println("net/http:")
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if connInfo.Conn != nil {
				fmt.Println("  connected to ", connInfo.Conn.RemoteAddr())
			}
		},
	}
	request, err = http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), "GET", url, http.NoBody)
	if err != nil {
		log.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	if resp == nil {
		log.Fatal("nil response")
	}
	fmt.Println("  got status: ", resp.Status)
	fmt.Println("  proto: ", resp.Proto)
	client = *http.DefaultClient
}
