package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientIPv4AndIPv6(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		t.Run(network, func(t *testing.T) {
			address := "127.0.0.1:0"
			if network == "tcp6" {
				address = "[::1]:0"
			}
			listener, err := net.Listen(network, address)
			if err != nil {
				if network == "tcp6" {
					t.Skipf("IPv6 unavailable: %v", err)
				}
				t.Fatal(err)
			}
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("connected"))
			}))
			server.Listener.Close()
			server.Listener = listener
			server.StartTLS()
			defer server.Close()
			client := newHTTPClient()
			transport := client.Transport.(*http.Transport)
			defer transport.CloseIdleConnections()
			// Trust only this test server's certificate, retaining hostname checks.
			transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != 200 || string(body) != "connected" {
				t.Fatalf("TLS request failed: %d %q %v", response.StatusCode, body, err)
			}
		})
	}
}
