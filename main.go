package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"

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
	// quic-go
	fmt.Println("QUIC-GO:")
	tr := tracer{}
	client := http.Client{Transport: &http3.RoundTripper{
		QuicConfig: &quic.Config{Tracer: tr}}}
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
	// the httptrace package allows use to catch the connection and display the server IP
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

	// with quic.DialAddrEarlyContext
	fmt.Println("quic.DialAddrEarlyContext:", url)

	daeTransport := &http3.RoundTripper{
		QuicConfig: &quic.Config{Tracer: tr},
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
			// quic.DialAddrEarlyContext will prefer IPv4
			// if we do the resolution of addr here we also need to pass addr in tls.Cfg.ServerName for sni to work
			udpAddr, err := net.Dial("udp", addr)
			if err != nil {
				return nil, err
			}
			// fix sni for quic
			if tlsCfg != nil {
				if tlsCfg.ServerName == "" {
					sni, _, err := net.SplitHostPort(addr)
					if err != nil {
						// It's ok if net.SplitHostPort returns an error - it could be a hostname/IP address without a port.
						sni = addr
					}
					tlsCfg.ServerName = sni
				}
			}
			return quic.DialAddrEarlyContext(ctx, udpAddr.RemoteAddr().String(), tlsCfg, cfg)
		},
	}
	client = http.Client{Transport: daeTransport}
	request, err = http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		log.Fatal(err)
	}
	resp, err = client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	if resp == nil {
		log.Fatal("nil response")
	}
	fmt.Println("  got status: ", resp.Status)
	fmt.Println("  proto: ", resp.Proto)
}
