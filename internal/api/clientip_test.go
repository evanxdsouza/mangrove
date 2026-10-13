package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForwardedClientIP(t *testing.T) {
	cases := []struct {
		name   string
		peer   string
		xff    []string
		realIP string
		want   string
	}{
		{"direct peer ignores headers", "203.0.113.9:5000", []string{"1.2.3.4"}, "5.6.7.8", ""},
		{"X-Real-IP never trusted", "127.0.0.1:5000", nil, "5.6.7.8", ""},
		{"single proxy", "127.0.0.1:5000", []string{"198.51.100.7"}, "", "198.51.100.7"},
		{"client-prepended entries ignored", "127.0.0.1:5000", []string{"1.1.1.1, 2.2.2.2, 198.51.100.7"}, "", "198.51.100.7"},
		{"skips private proxy hops", "127.0.0.1:5000", []string{"9.9.9.9, 198.51.100.7, 10.0.0.2"}, "", "198.51.100.7"},
		{"split across header lines", "[::1]:5000", []string{"9.9.9.9", "198.51.100.7"}, "", "198.51.100.7"},
		{"LAN client", "127.0.0.1:5000", []string{"192.168.1.20"}, "", "192.168.1.20"},
		{"garbage stops the walk", "127.0.0.1:5000", []string{"9.9.9.9, nonsense, 10.0.0.2"}, "", "10.0.0.2"},
		{"no header", "127.0.0.1:5000", nil, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.peer
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if tc.realIP != "" {
				r.Header.Set("X-Real-IP", tc.realIP)
			}
			if got := forwardedClientIP(r); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
