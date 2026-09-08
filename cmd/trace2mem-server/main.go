package main

import (
	"context"
	"errors"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/mohit-lendmind/trace2mem/internal/app"
	"github.com/mohit-lendmind/trace2mem/internal/dream"
	"github.com/mohit-lendmind/trace2mem/internal/server"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, e := app.Open(ctx)
	if e != nil {
		return e
	}
	defer a.Close()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return a.Store.Migrate(ctx)
	}
	if a.Config.BootstrapToken == "" && a.Config.OIDCIssuer == "" {
		return errors.New("configure a bootstrap token or OIDC identity")
	}
	s := &server.Server{Store: a.Store, Blob: a.Blob, Vault: a.Vault, Config: a.Config, Engine: &dream.Engine{Store: a.Store, Vault: a.Vault, Config: a.Config, Blob: a.Blob}}
	if a.Config.OIDCIssuer != "" {
		s.OIDC, e = oidc.NewProvider(ctx, a.Config.OIDCIssuer)
		if e != nil {
			return e
		}
	}
	srv := &http.Server{BaseContext: func(net.Listener) context.Context { return ctx }, Addr: a.Config.Addr, Handler: h2c.NewHandler(s.Handler(), &http2.Server{}), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32 << 10}
	done := make(chan error, 1)
	go func() { slog.Info("server listening", "address", srv.Addr); done <- srv.ListenAndServe() }()
	select {
	case e := <-done:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			srv.Close()
			return err
		}
		return nil
	}
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if e := run(); e != nil {
		slog.Error("server stopped", "error", e)
		os.Exit(1)
	}
}
