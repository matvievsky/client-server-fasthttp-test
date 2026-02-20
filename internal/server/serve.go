package server

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"

	serverconfig "client-server-fasthttp-test/internal/server/config"

	"github.com/valyala/fasthttp"
)

func Serve() error {
	cfg := serverconfig.Cfg

	uploadHandler := newHandlerConfig(cfg.FileField, cfg.MaxConcurrentUploads)

	if cfg.PprofEnabled {
		go runPprofServer(cfg.PprofAddr)
	}

	server := &fasthttp.Server{
		Name:               cfg.Name,
		Handler:            uploadHandler.handler,
		StreamRequestBody:  cfg.StreamRequestBody,
		MaxRequestBodySize: cfg.MaxRequestBodySize,
		ReadTimeout:        cfg.ReadTimeout,
		WriteTimeout:       cfg.WriteTimeout,
		IdleTimeout:        cfg.IdleTimeout,
	}

	log.Printf("server is listening on %s", cfg.Addr)
	if err := server.ListenAndServe(cfg.Addr); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}

func runPprofServer(addr string) {
	log.Printf("pprof is listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("pprof listen and serve: %v", err)
	}
}
