package telegram

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// healthServer exposes the liveness and readiness probes of the bot. The
// Telegram adapter is the only component that binds a port: in webhook mode
// the dedicated webhook route shares the same listener.
type healthServer struct {
	server   *http.Server
	listener net.Listener
}

// startHealthServer binds the probe listener. The webhook route is optional
// and mounted under its own dedicated path.
func startHealthServer(addr string, ready *atomic.Bool, webhookPath string, webhook http.Handler) (*healthServer, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if webhook != nil && webhookPath != "" {
		mux.Handle(webhookPath, webhook)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	return &healthServer{server: server, listener: listener}, nil
}

// Shutdown stops the probe listener and releases the address.
func (h *healthServer) Shutdown(ctx context.Context) {
	if h == nil {
		return
	}
	_ = h.server.Shutdown(ctx)
	_ = h.listener.Close()
}
