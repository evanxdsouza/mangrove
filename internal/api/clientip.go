package api

import (
	"net"
	"net/http"
	"strings"
)

// clientIP replaces chi's middleware.RealIP, which believes True-Client-IP,
// X-Real-IP, and the leftmost X-Forwarded-For entry from anyone -- all
// client-controlled, so a fresh value per request walked straight past the
// per-IP rate limit on /api/auth/login and /api/auth/setup.
//
// Forwarding headers are only honored when the direct peer is itself a
// proxy (loopback or private address -- Mangrove listens on 127.0.0.1
// behind Caddy). X-Forwarded-For is then read right to left, skipping
// proxy hops, and the first public address is the client: entries to its
// left were supplied by the client and are never trusted. With no public
// entry (a LAN client), the rightmost entry is used.
func clientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := forwardedClientIP(r); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

func forwardedClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if peer := net.ParseIP(host); peer == nil || !isProxyAddr(peer) {
		return ""
	}
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	rightmost := ""
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			// Anything unparseable at or left of here is untrustworthy.
			break
		}
		if rightmost == "" {
			rightmost = ip.String()
		}
		if !isProxyAddr(ip) {
			return ip.String()
		}
	}
	return rightmost
}

func isProxyAddr(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
