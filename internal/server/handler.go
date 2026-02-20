package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"client-server-fasthttp-test/internal/client/uploader"
	"client-server-fasthttp-test/internal/server/format"

	"github.com/bytedance/sonic"
	"github.com/valyala/fasthttp"
)

type handlerConfig struct {
	fileFieldName string
	uploadSlots   chan struct{}
}

type uploadSuccessResponse struct {
	Status   string `json:"status"`
	Files    int    `json:"files"`
	Size     string `json:"size"`
	Duration string `json:"duration"`
	Speed    string `json:"speed"`
	SHA256   string `json:"sha256"`
}

type errorResponse struct {
	Status           string `json:"status"`
	Error            string `json:"error"`
	ExpectedChecksum string `json:"expected_checksum,omitempty"`
	ActualChecksum   string `json:"actual_checksum,omitempty"`
}

func newHandlerConfig(fileFieldName string, maxConcurrentUploads int) *handlerConfig {
	h := &handlerConfig{fileFieldName: fileFieldName}
	if maxConcurrentUploads > 0 {
		h.uploadSlots = make(chan struct{}, maxConcurrentUploads)
	}

	return h
}

func (h *handlerConfig) tryAcquireUploadSlot() (func(), bool) {
	if h.uploadSlots == nil {
		return func() {}, true
	}

	select {
	case h.uploadSlots <- struct{}{}:
		return func() { <-h.uploadSlots }, true
	default:
		return nil, false
	}
}

func writeJSON(ctx *fasthttp.RequestCtx, statusCode int, payload any) {
	body, err := sonic.Marshal(payload)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetContentType("application/json; charset=utf-8")
		ctx.SetBodyString(`{"status":"error","error":"internal server error"}`)
		return
	}

	ctx.SetStatusCode(statusCode)
	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetBody(body)
}

func writeJSONError(ctx *fasthttp.RequestCtx, statusCode int, msg string) {
	writeJSON(ctx, statusCode, errorResponse{
		Status: "error",
		Error:  msg,
	})
}

func (h *handlerConfig) handler(ctx *fasthttp.RequestCtx) {
	switch {
	case ctx.IsGet() && string(ctx.Path()) == "/healthz":
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyString("ok")
	case ctx.IsPost() && string(ctx.Path()) == "/upload":
		h.handleUpload(ctx)
	default:
		writeJSONError(ctx, fasthttp.StatusNotFound, "not found")
	}
}

func (h *handlerConfig) handleUpload(ctx *fasthttp.RequestCtx) {
	start := time.Now()
	releaseUploadSlot, ok := h.tryAcquireUploadSlot()
	if !ok {
		writeJSONError(ctx, fasthttp.StatusServiceUnavailable, "too many concurrent uploads")
		return
	}
	defer releaseUploadSlot()

	form, err := ctx.MultipartForm()
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("read multipart form: %v", err))
		return
	}

	files := form.File[h.fileFieldName]
	if len(files) == 0 {
		writeJSONError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("multipart field %q is required", h.fileFieldName))
		return
	}

	var totalBytes int64
	actualChecksum := "n/a"
	aggregateHasher := sha256.New()
	expectedChecksums, checksumErr := expectedChecksumsForRequest(ctx, form.Value[uploader.ChecksumFieldSHA256], len(files))
	if checksumErr != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, checksumErr.Error())
		return
	}

	for idx, fileHeader := range files {
		f, err := fileHeader.Open()
		if err != nil {
			writeJSONError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("open uploaded file %q: %v", fileHeader.Filename, err))
			return
		}

		hash, n, hashErr := hashSHA256HexAndCount(f)
		closeErr := f.Close()
		if hashErr != nil {
			writeJSONError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("read uploaded file %q: %v", fileHeader.Filename, hashErr))
			return
		}
		if closeErr != nil {
			writeJSONError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("close uploaded file %q: %v", fileHeader.Filename, closeErr))
			return
		}

		totalBytes += n
		if len(files) == 1 {
			actualChecksum = hash
		} else {
			if _, err := fmt.Fprintf(aggregateHasher, "%s:%s\n", fileHeader.Filename, hash); err != nil {
				writeJSONError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("aggregate checksum: %v", err))
				return
			}
		}
		if len(expectedChecksums) > 0 && hash != expectedChecksums[idx] {
			writeJSON(ctx, fasthttp.StatusUnprocessableEntity, errorResponse{
				Status:           "error",
				Error:            "checksum mismatch",
				ExpectedChecksum: expectedChecksums[idx],
				ActualChecksum:   hash,
			})
			return
		}
	}
	if len(files) > 1 {
		actualChecksum = hex.EncodeToString(aggregateHasher.Sum(nil))
	}

	elapsed := time.Since(start)
	throughput := 0.0
	if elapsed > 0 {
		throughput = float64(totalBytes) / elapsed.Seconds()
	}
	speed := format.BytesPerSecond(throughput)

	log.Printf(
		"upload complete: files=%d size=%s duration=%s speed=%s sha256=%s",
		len(files),
		format.Bytes(totalBytes),
		elapsed.Round(time.Millisecond),
		speed,
		actualChecksum,
	)

	writeJSON(ctx, fasthttp.StatusCreated, uploadSuccessResponse{
		Status:   "ok",
		Files:    len(files),
		Size:     format.Bytes(totalBytes),
		Duration: elapsed.Round(time.Millisecond).String(),
		Speed:    speed,
		SHA256:   actualChecksum,
	})
}

func hashSHA256HexAndCount(r io.Reader) (string, int64, error) {
	hasher := sha256.New()
	n, err := io.Copy(io.MultiWriter(io.Discard, hasher), r)
	if err != nil {
		return "", n, fmt.Errorf("hash stream: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), n, nil
}

func expectedChecksumsForRequest(_ *fasthttp.RequestCtx, multipartChecksums []string, fileCount int) ([]string, error) {
	checksums := make([]string, 0, len(multipartChecksums))
	for _, checksum := range multipartChecksums {
		clean := strings.TrimSpace(checksum)
		if clean != "" {
			checksums = append(checksums, clean)
		}
	}
	if len(checksums) > 0 && len(checksums) != fileCount {
		return nil, fmt.Errorf("checksum count mismatch: got %d for %d file(s)", len(checksums), fileCount)
	}

	return checksums, nil
}
