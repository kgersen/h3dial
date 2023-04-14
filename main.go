package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/logging"
)

var url = "https://cloudflare-quic.com/"

type tracer struct {
	logging.NullTracer
}

func (t tracer) TracerForConnection(context.Context, logging.Perspective, logging.ConnectionID) logging.ConnectionTracer {
	return ConnectionTracer{}
}

type ConnectionTracer struct {
	logging.NullConnectionTracer
}

func (n ConnectionTracer) StartedConnection(local, remote net.Addr, srcConnID, destConnID logging.ConnectionID) {
	fmt.Println("  connected to", remote)
}

func main() {
	url := url
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	// quic-go - basic http3.RoundTripper
	fmt.Println("QUIC-GO:")
	tr := tracer{}
	client := http.Client{Transport: &http3.RoundTripper{
		QuicConfig: &quic.Config{Tracer: tr}}}
	dial(context.Background(), &client, url)

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
	dial(ctx, http.DefaultClient, url)

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
		QuicConfig: &quic.Config{Tracer: tr},
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
			udpAddr, err := net.Dial("udp", addr)
			if err != nil {
				return nil, err
			}
			if udpConn == nil {
				// check for Zone on IPv6 link-local for instance, some OS might the Zone too
				udpConn, err = net.ListenUDP("udp", nil)
				if err != nil {
					return nil, err
				}
			}
			return quic.DialEarlyContext(ctx, udpConn, udpAddr.RemoteAddr(), addr, tlsCfg, cfg)
		},
	}
	client = http.Client{Transport: daeTransport}
	dial(context.Background(), &client, url)
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
