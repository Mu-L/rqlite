package http

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rqlite/rqlite/v8/internal/rtls"
)

// Test_IsServingHTTP_HTTPServer tests that we correctly detect an HTTP server running.
func Test_IsServingHTTP_HTTPServer(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != isServingTestPath {
			t.Fatalf("Expected %s, got %s", isServingTestPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer httpServer.Close()

	addr := httpServer.Listener.Addr().String()
	if !IsServingHTTP(addr) {
		t.Fatalf("Expected true for HTTP server running on %s", addr)
	}
	if a, ok := AnyServingHTTP([]string{addr}); !ok || a != addr {
		t.Fatalf("Expected %s for AnyServingHTTP", addr)
	}
}

// Test_IsServingHTTP_HTTPSServer tests that we correctly detect an HTTPS server running.
func Test_IsServingHTTP_HTTPSServer(t *testing.T) {
	httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != isServingTestPath {
			t.Fatalf("Expected %s, got %s", isServingTestPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer httpsServer.Close()

	addr := httpsServer.Listener.Addr().String()
	if !IsServingHTTP(addr) {
		t.Error("Expected true for HTTPS server running")
	}
	if a, ok := AnyServingHTTP([]string{addr}); !ok || a != addr {
		t.Fatalf("Expected %s for AnyServingHTTP", addr)
	}
}

// Test_IsServingHTTP_NoServersRunning tests that we correctly detect no servers running.
func Test_IsServingHTTP_NoServersRunning(t *testing.T) {
	addr := "127.0.0.1:9999" // Assume this address is not used
	if IsServingHTTP(addr) {
		t.Error("Expected false for no servers running")
	}
	if _, ok := AnyServingHTTP([]string{addr}); ok {
		t.Error("Expected false for no servers running")
	}
}

// Test_IsServingHTTP_InvalidAddress tests invalid address format.
func Test_IsServingHTTP_InvalidAddress(t *testing.T) {
	addr := "invalid-address"
	if IsServingHTTP(addr) {
		t.Error("Expected false for invalid address")
	}
	if _, ok := AnyServingHTTP([]string{addr}); ok {
		t.Error("Expected false for invalid address")
	}
}

// Test_IsServingHTTP_HTTPErrorStatusCode tests we correctly detect an HTTP server
// running even when that server returns error status codes.
func Test_IsServingHTTP_HTTPErrorStatusCode(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized} {
		func() {
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer httpServer.Close()

			addr := httpServer.Listener.Addr().String()
			if !IsServingHTTP(addr) {
				t.Error("Expected true for HTTP server running, even with error status code")
			}
			if a, ok := AnyServingHTTP([]string{addr}); !ok || a != addr {
				t.Fatalf("Expected %s for AnyServingHTTP, even with error status code", addr)
			}
		}()
	}
}

// Test_IsServingHTTP_HTTPSErrorStatusCode tests we correctly detect an HTTPS server
// running even when that server returns error status codes.
func Test_IsServingHTTP_HTTPSErrorStatusCode(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized} {
		func() {
			httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer httpsServer.Close()

			addr := httpsServer.Listener.Addr().String()
			if !IsServingHTTP(addr) {
				t.Error("Expected true for HTTPS server running, even with error status code")
			}
			if a, ok := AnyServingHTTP([]string{addr}); !ok || a != addr {
				t.Fatalf("Expected %s for AnyServingHTTP, even with error status code", addr)
			}
		}()
	}
}

// Test_IsServingHTTP_OpenPort tests that we correctly detect a non-functioning HTTP
// server when the other side is just an open TCP port.
func Test_IsServingHTTP_OpenPort(t *testing.T) {
	// Create a TCP listener on a random port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	if IsServingHTTP(addr) {
		t.Error("Expected false for open port")
	}
	if _, ok := AnyServingHTTP([]string{addr}); ok {
		t.Error("Expected false for open port")
	}
}

// Test_IsServingHTTP_OpenPort tests that we correctly detect a non-functioning HTTP
// server when the other side is just an open TLS TCP port.
func Test_IsServingHTTP_OpenPortTLS(t *testing.T) {
	cert, key, err := rtls.GenerateSelfSignedCert(pkix.Name{CommonName: "rqlite"}, time.Hour, 2048)
	if err != nil {
		t.Fatalf("failed to generate self-signed cert: %s", err)
	}
	certFile := mustWriteTempFile(t, cert)
	keyFile := mustWriteTempFile(t, key)
	tlsConfig, err := rtls.CreateServerConfig(certFile, keyFile, rtls.NoCACert, rtls.MTLSStateEnabled)
	if err != nil {
		t.Fatalf("failed to create TLS config: %s", err)
	}

	ln, err := tls.Listen("tcp", ":0", tlsConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %s", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	if IsServingHTTP(addr) {
		t.Error("Expected false for open TLS port")
	}
	if _, ok := AnyServingHTTP([]string{addr}); ok {
		t.Error("Expected false for open TLS port")
	}
}

func Test_IsServingHTTP_HTTPServerTCPPort(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer httpServer.Close()

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	httpAddr := httpServer.Listener.Addr().String()
	tcpAddr := ln.Addr().String()
	if a, ok := AnyServingHTTP([]string{httpAddr, tcpAddr}); !ok || a != httpAddr {
		t.Fatalf("Expected %s for AnyServingHTTP", httpAddr)
	}
}
