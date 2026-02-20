package client

import (
	"context"

	clientconfig "client-server-fasthttp-test/internal/client/config"
)

func Serve() error {
	cfg := clientconfig.Cfg

	handler, err := newUploadHandler(cfg)
	if err != nil {
		return err
	}

	return handler.Handle(context.Background())
}
