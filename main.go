package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/logging"
)

var sUrl = "https://cloudflare-quic.com/"

func tracer(ctx context.Context, p logging.Perspective, ci quic.ConnectionID) *logging.ConnectionTracer {
	return &logging.ConnectionTracer{
		StartedConnection: func(local, remote net.Addr, srcConnID, destConnID logging.ConnectionID) {
			fmt.Println("  connected to", remote)
		},
	}
}

func main() {

	if len(os.Args) > 1 {
		sUrl = os.Args[1]
	}
	u, err := url.Parse(sUrl)
	if err != nil {
		panic(err)
	}
	// quic-go - basic http3.RoundTripper
	fmt.Println("QUIC-GO:")

	client := http.Client{
		Transport: &http3.RoundTripper{
			TLSClientConfig: &tls.Config{ServerName: u.Hostname()},
			QuicConfig:      &quic.Config{Tracer: tracer}},
	}
	dial(context.Background(), &client, sUrl)

	// net/http
	fmt.Println("net/http:")
	// the httptrace package allows use to catch the connection and display the server IP
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if connInfo.Conn != nil {
				fmt.Println("  connected to ", connInfo.Conn.RemoteAddr())
			}
		},
	}
	ctx := httptrace.WithClientTrace(context.Background(), trace)
	dial(ctx, http.DefaultClient, sUrl)

	// quic-go - basic http3.RoundTripper with custom Dial
	fmt.Println("QUIC-GO custom Dial:")

	// with use http3.RoundTripper but it doesnt expose its udpConn member field so we must use our own
	var udpConn *net.UDPConn
	defer func() {
		if udpConn != nil {
			fmt.Println("closing local udp conn")
			udpConn.Close()
		}
	}()

	daeTransport := &http3.RoundTripper{
		QuicConfig:      &quic.Config{Tracer: tracer},
		TLSClientConfig: &tls.Config{ServerName: u.Hostname()},
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
			udpAddr, err := net.Dial("udp", addr)
			if err != nil {
				return nil, err
			}
			if udpConn == nil {
				// check for Zone on IPv6 link-local for instance, some OS might need the Zone too
				udpConn, err = net.ListenUDP("udp", nil)
				if err != nil {
					return nil, err
				}
			}
			return quic.DialEarly(ctx, udpConn, udpAddr.RemoteAddr(), tlsCfg, cfg)
		},
	}
	client = http.Client{Transport: daeTransport}
	dial(context.Background(), &client, sUrl)
}

func dial(ctx context.Context, client *http.Client, url string) {
	fmt.Println("  dialing", url)
	request, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		log.Println(err)
		return
	}
	resp, err := client.Do(request)
	if err != nil {
		log.Println(err)
		return
	}
	if resp == nil {
		log.Println("nil response")
		return
	}
	fmt.Println("  got status: ", resp.Status)
	fmt.Println("  proto: ", resp.Proto)
}
