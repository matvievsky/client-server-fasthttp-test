package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"client-server-fasthttp-test/internal/client/config"
	"client-server-fasthttp-test/internal/client/uploader"

	"github.com/bytedance/sonic"
)

type uploadHandler struct {
	client *uploader.Client
	cfg    config.AppConfig
}

type uploadResultPayload struct {
	Status           string `json:"status"`
	Files            int    `json:"files"`
	Size             string `json:"size"`
	Duration         string `json:"duration"`
	Speed            string `json:"speed"`
	SHA256           string `json:"sha256"`
	Error            string `json:"error"`
	ExpectedChecksum string `json:"expected_checksum"`
	ActualChecksum   string `json:"actual_checksum"`
}

func newUploadHandler(cfg config.AppConfig) (*uploadHandler, error) {
	client, err := uploader.New(nil, uploader.Config{
		ChunkSize:      cfg.ChunkSize,
		FormFieldName:  cfg.FieldName,
		RequestTimeout: cfg.RequestTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return &uploadHandler{
		client: client,
		cfg:    cfg,
	}, nil
}

func (h *uploadHandler) Handle(ctx context.Context) error {
	start := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, h.cfg.MaxConcurrent)
	responses := make([]*uploader.UploadResponse, len(h.cfg.Files))

	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for i, filePath := range h.cfg.Files {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			resp, err := h.client.UploadFileContext(ctx, uploader.UploadRequest{
				URL:      h.cfg.URL,
				FilePath: path,
			})
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("upload file %q: %w", path, err)
					cancel()
				}
				errMu.Unlock()
				return
			}

			responses[idx] = resp
		}(i, filePath)
	}

	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	for i, resp := range responses {
		if resp == nil {
			return fmt.Errorf("upload file %q: empty response", h.cfg.Files[i])
		}

		var payload uploadResultPayload
		if err := sonic.Unmarshal(resp.Body, &payload); err != nil {
			slog.Info("upload result",
				"file", h.cfg.Files[i],
				"http_status", resp.StatusCode,
				"response_parse_error", err.Error(),
				"response_bytes", len(resp.Body),
			)
			continue
		}

		slog.Info("upload result",
			"file", h.cfg.Files[i],
			"http_status", resp.StatusCode,
			"status", payload.Status,
			"files", payload.Files,
			"size", payload.Size,
			"duration", payload.Duration,
			"speed", payload.Speed,
			"sha256", payload.SHA256,
			"error", payload.Error,
			"expected_checksum", payload.ExpectedChecksum,
			"actual_checksum", payload.ActualChecksum,
		)
	}
	slog.Info("upload batch complete",
		"files", len(h.cfg.Files),
		"total_duration", time.Since(start).Round(time.Millisecond).String(),
	)

	return nil
}
