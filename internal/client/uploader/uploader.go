package uploader

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	ChecksumFieldSHA256 = "checksum_sha256"
)

type Config struct {
	ChunkSize      int
	FormFieldName  string
	RequestTimeout time.Duration
}

type Client struct {
	httpClient *fasthttp.Client
	cfg        Config
}

type uploadFile struct {
	path string
	name string
}

type UploadRequest struct {
	URL      string
	FilePath string
	FileName string
}

type UploadResponse struct {
	StatusCode int
	Body       []byte
}

func New(httpClient *fasthttp.Client, cfg Config) (*Client, error) {
	if httpClient == nil {
		httpClient = &fasthttp.Client{}
	}

	return &Client{
		httpClient: httpClient,
		cfg:        cfg,
	}, nil
}

func (c *Client) UploadFileContext(ctx context.Context, uploadReq UploadRequest) (*UploadResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	url, fileMeta, err := validateUploadRequest(uploadReq)
	if err != nil {
		return nil, err
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI(url)

	boundary := multipart.NewWriter(io.Discard).Boundary()
	req.Header.SetContentType("multipart/form-data; boundary=" + boundary)

	streamErrCh := make(chan error, 1)
	req.SetBodyStreamWriter(func(w *bufio.Writer) {
		streamErrCh <- c.writeMultipartBody(w, boundary, fileMeta)
	})

	doErr := c.doRequest(ctx, req, resp)
	if doErr != nil {
		var streamErr error
		select {
		case streamErr = <-streamErrCh:
		default:
		}

		if streamErr != nil {
			return nil, fmt.Errorf("send request: %v (stream error: %w)", doErr, streamErr)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("send request: %w", ctxErr)
		}
		return nil, fmt.Errorf("send request: %w", doErr)
	}

	streamErr := <-streamErrCh
	if streamErr != nil {
		return nil, fmt.Errorf("stream multipart body: %w", streamErr)
	}

	return &UploadResponse{
		StatusCode: resp.StatusCode(),
		Body:       append([]byte(nil), resp.Body()...),
	}, nil
}

func (c *Client) doRequest(ctx context.Context, req *fasthttp.Request, resp *fasthttp.Response) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if deadline, ok := ctx.Deadline(); ok {
		return c.httpClient.DoDeadline(req, resp, deadline)
	}

	if c.cfg.RequestTimeout > 0 {
		return c.httpClient.DoTimeout(req, resp, c.cfg.RequestTimeout)
	}

	return c.httpClient.Do(req, resp)
}

func (c *Client) writeMultipartBody(w *bufio.Writer, boundary string, fileMeta uploadFile) error {
	mw := multipart.NewWriter(w)
	if err := mw.SetBoundary(boundary); err != nil {
		return fmt.Errorf("set multipart boundary: %w", err)
	}

	buf := make([]byte, c.cfg.ChunkSize)
	partWriter, err := mw.CreateFormFile(c.cfg.FormFieldName, fileMeta.name)
	if err != nil {
		return fmt.Errorf("create form file part: %w", err)
	}

	file, err := os.Open(fileMeta.path)
	if err != nil {
		return fmt.Errorf("open file %q: %w", fileMeta.path, err)
	}

	hasher := sha256.New()
	if _, err := io.CopyBuffer(io.MultiWriter(partWriter, hasher), file, buf); err != nil {
		_ = file.Close()
		return fmt.Errorf("copy file to multipart body: %w", err)
	}
	fileSHA256 := hex.EncodeToString(hasher.Sum(nil))

	if err := file.Close(); err != nil {
		return fmt.Errorf("close file %q: %w", fileMeta.path, err)
	}

	if err := mw.WriteField(ChecksumFieldSHA256, fileSHA256); err != nil {
		return fmt.Errorf("write checksum form field: %w", err)
	}

	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	return nil
}

func validateUploadRequest(uploadReq UploadRequest) (string, uploadFile, error) {
	if uploadReq.URL == "" {
		return "", uploadFile{}, fmt.Errorf("url is required")
	}

	if uploadReq.FilePath == "" {
		return "", uploadFile{}, fmt.Errorf("file path is required")
	}

	fileInfo, err := os.Stat(uploadReq.FilePath)
	if err != nil {
		return "", uploadFile{}, fmt.Errorf("stat file %q: %w", uploadReq.FilePath, err)
	}
	if fileInfo.IsDir() {
		return "", uploadFile{}, fmt.Errorf("file path %q points to a directory", uploadReq.FilePath)
	}

	fileName := uploadReq.FileName
	if fileName == "" {
		fileName = filepath.Base(uploadReq.FilePath)
	}

	return uploadReq.URL, uploadFile{
		path: uploadReq.FilePath,
		name: fileName,
	}, nil
}
