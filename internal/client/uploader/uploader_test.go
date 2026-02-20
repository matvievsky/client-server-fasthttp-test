package uploader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func validUploaderConfig(chunkSize int) Config {
	return Config{
		ChunkSize:      chunkSize,
		FormFieldName:  "file",
		RequestTimeout: 30 * time.Second,
	}
}

func TestUploadFile(t *testing.T) {
	expectedContent := bytes.Repeat([]byte("abcdef0123456789"), 1024)

	tempFilePath := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(tempFilePath, expectedContent, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var receivedFile []byte
	var receivedContentType string
	var receivedChecksum string
	var handlerErr error
	var mu sync.Mutex

	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			mu.Lock()
			receivedContentType = string(ctx.Request.Header.ContentType())
			mu.Unlock()

			form, err := ctx.MultipartForm()
			if err != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("multipart form: %w", err)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			files := form.File["file"]
			if len(files) != 1 {
				mu.Lock()
				handlerErr = fmt.Errorf("expected one file in multipart payload")
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}
			checksums := form.Value[ChecksumFieldSHA256]
			if len(checksums) != 1 {
				mu.Lock()
				handlerErr = fmt.Errorf("expected one checksum value in multipart payload")
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			file, err := files[0].Open()
			if err != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("open uploaded file: %w", err)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}
			content, err := io.ReadAll(file)
			closeErr := file.Close()
			if err != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("read uploaded file: %w", err)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}
			if closeErr != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("close uploaded file: %w", closeErr)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			mu.Lock()
			receivedFile = content
			receivedChecksum = checksums[0]
			mu.Unlock()

			ctx.SetStatusCode(fasthttp.StatusCreated)
			ctx.SetBodyString("ok")
		},
	}

	ln := fasthttputil.NewInmemoryListener()
	defer ln.Close()

	go func() {
		_ = server.Serve(ln)
	}()
	defer server.Shutdown()

	httpClient := &fasthttp.Client{
		Dial: func(_ string) (net.Conn, error) {
			return ln.Dial()
		},
	}

	client, err := New(httpClient, validUploaderConfig(64))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.UploadFileContext(context.Background(), UploadRequest{
		URL:      "http://inmemory/upload",
		FilePath: tempFilePath,
	})
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}

	if resp.StatusCode != fasthttp.StatusCreated {
		t.Fatalf("unexpected status code: got %d want %d", resp.StatusCode, fasthttp.StatusCreated)
	}

	mu.Lock()
	defer mu.Unlock()

	if handlerErr != nil {
		t.Fatalf("handler error: %v", handlerErr)
	}

	if !bytes.Equal(receivedFile, expectedContent) {
		t.Fatalf("uploaded file content mismatch")
	}
	expectedChecksumBytes := sha256.Sum256(expectedContent)
	expectedChecksum := hex.EncodeToString(expectedChecksumBytes[:])
	if receivedChecksum != expectedChecksum {
		t.Fatalf("unexpected checksum header: got %q want %q", receivedChecksum, expectedChecksum)
	}

	if !strings.HasPrefix(receivedContentType, "multipart/form-data; boundary=") {
		t.Fatalf("unexpected content type: %q", receivedContentType)
	}
}

func TestUploadFileMultipleFiles(t *testing.T) {
	payloads := map[string][]byte{
		"payload-1.bin": bytes.Repeat([]byte("a1b2c3"), 512),
		"payload-2.bin": bytes.Repeat([]byte("x9y8z7"), 1024),
		"payload-3.bin": bytes.Repeat([]byte("qwerty"), 256),
	}

	tempDir := t.TempDir()
	paths := make(map[string]string, len(payloads))
	for name, content := range payloads {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write temp file %q: %v", name, err)
		}
		paths[name] = path
	}

	var handlerErr error
	received := make(map[string][]byte, len(payloads))
	receivedChecksums := make(map[string]string, len(payloads))
	var mu sync.Mutex

	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			form, err := ctx.MultipartForm()
			if err != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("multipart form: %w", err)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			files := form.File["file"]
			checksums := form.Value[ChecksumFieldSHA256]
			if len(files) != 1 {
				mu.Lock()
				handlerErr = fmt.Errorf("expected 1 file, got %d", len(files))
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}
			if len(checksums) != 1 {
				mu.Lock()
				handlerErr = fmt.Errorf("expected 1 checksum value, got %d", len(checksums))
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			fh := files[0]
			file, err := fh.Open()
			if err != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("open uploaded file: %w", err)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			content, err := io.ReadAll(file)
			closeErr := file.Close()
			if err != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("read uploaded file: %w", err)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}
			if closeErr != nil {
				mu.Lock()
				handlerErr = fmt.Errorf("close uploaded file: %w", closeErr)
				mu.Unlock()
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				return
			}

			mu.Lock()
			received[fh.Filename] = content
			receivedChecksums[fh.Filename] = checksums[0]
			mu.Unlock()

			ctx.SetStatusCode(fasthttp.StatusCreated)
		},
	}

	ln := fasthttputil.NewInmemoryListener()
	defer ln.Close()

	go func() {
		_ = server.Serve(ln)
	}()
	defer server.Shutdown()

	httpClient := &fasthttp.Client{
		Dial: func(_ string) (net.Conn, error) {
			return ln.Dial()
		},
	}

	client, err := New(httpClient, validUploaderConfig(64))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	for _, path := range paths {
		resp, err := client.UploadFileContext(context.Background(), UploadRequest{
			URL:      "http://inmemory/upload",
			FilePath: path,
		})
		if err != nil {
			t.Fatalf("upload file: %v", err)
		}
		if resp.StatusCode != fasthttp.StatusCreated {
			t.Fatalf("unexpected status code: got %d want %d", resp.StatusCode, fasthttp.StatusCreated)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if handlerErr != nil {
		t.Fatalf("handler error: %v", handlerErr)
	}
	if len(received) != len(payloads) {
		t.Fatalf("unexpected received files count: got %d want %d", len(received), len(payloads))
	}

	for name, expectedContent := range payloads {
		gotContent, ok := received[name]
		if !ok {
			t.Fatalf("file %q was not received", name)
		}
		if !bytes.Equal(gotContent, expectedContent) {
			t.Fatalf("uploaded file content mismatch for %q", name)
		}
		expectedChecksumBytes := sha256.Sum256(expectedContent)
		expectedChecksum := hex.EncodeToString(expectedChecksumBytes[:])
		if receivedChecksums[name] != expectedChecksum {
			t.Fatalf("unexpected checksum for %q: got %q want %q", name, receivedChecksums[name], expectedChecksum)
		}
	}
}

func TestValidConfig(t *testing.T) {
	_, err := New(nil, validUploaderConfig(64))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
