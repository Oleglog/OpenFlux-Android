package utils

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

// ConfigureWebsocketDialerProxy configures proxy settings on a gorilla websocket.Dialer.
// It checks environment variables (ALL_PROXY, HTTPS_PROXY, etc.) for targetURL.
// For SOCKS5 proxies, Gorilla websocket does not natively support socks5 in dialProxy,
// so this sets NetDialContext to dial via proxy.FromURL and sets Proxy to nil to prevent errors.
func ConfigureWebsocketDialerProxy(dialer *websocket.Dialer, targetURL string) {
	if dialer == nil {
		return
	}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return
	}
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil || proxyURL == nil {
		return
	}

	switch strings.ToLower(proxyURL.Scheme) {
	case "socks5", "socks5h":
		forward := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		socksDialer, err := proxy.FromURL(proxyURL, forward)
		if err != nil {
			log.Printf("[PROXY] Failed to create SOCKS5 dialer for %s: %v", proxyURL.Redacted(), err)
			return
		}
		dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socksDialer.Dial(network, addr)
		}
		dialer.Proxy = func(r *http.Request) (*url.URL, error) {
			return nil, nil // Handled by NetDialContext
		}
		log.Printf("[PROXY] WebSocket configured via SOCKS5 proxy: %s", proxyURL.Redacted())
	case "http", "https":
		dialer.Proxy = http.ProxyURL(proxyURL)
		log.Printf("[PROXY] WebSocket configured via HTTP proxy: %s", proxyURL.Redacted())
	}
}

// CreateSocks5Dialer creates a proxy.Dialer from a given proxy address string.
func CreateSocks5Dialer(proxyAddr string, timeout time.Duration) (proxy.Dialer, error) {
	proxyAddr = strings.TrimSpace(proxyAddr)
	if proxyAddr == "" {
		return nil, fmt.Errorf("empty proxy address")
	}
	if !strings.Contains(proxyAddr, "://") {
		proxyAddr = "socks5://" + proxyAddr
	}
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyAddr, err)
	}
	forward := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return proxy.FromURL(u, forward)
}
