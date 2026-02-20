package config

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultConfigFile     = ".env.client"
	defaultChunkSize      = 256
	defaultFormFieldName  = "file"
	defaultRequestTimeout = 30 * time.Second

	keyURL            = "UPLOAD_CLIENT_URL"
	keyFiles          = "UPLOAD_CLIENT_FILES"
	keyFile           = "UPLOAD_CLIENT_FILE"
	keyChunkSize      = "UPLOAD_CLIENT_CHUNK_SIZE"
	keyField          = "UPLOAD_CLIENT_FIELD"
	keyRequestTimeout = "UPLOAD_CLIENT_REQUEST_TIMEOUT"
	keyMaxConcurrent  = "UPLOAD_CLIENT_MAX_CONCURRENT_UPLOADS"
)

var Cfg AppConfig

type AppConfig struct {
	URL            string
	Files          []string
	ChunkSize      int
	FieldName      string
	RequestTimeout time.Duration
	MaxConcurrent  int
}

func init() {
	appViper := viper.New()

	appViper.AutomaticEnv()

	appViper.SetDefault(keyURL, "http://localhost:8080/upload")
	appViper.SetDefault(keyChunkSize, defaultChunkSize)
	appViper.SetDefault(keyField, defaultFormFieldName)
	appViper.SetDefault(keyRequestTimeout, defaultRequestTimeout)
	appViper.SetDefault(keyMaxConcurrent, 4)

	appViper.SetConfigFile(defaultConfigFile)
	appViper.SetConfigType("env")

	if err := appViper.ReadInConfig(); err != nil {
		var configNotFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFoundErr) && !os.IsNotExist(err) {
			log.Panicf("read %s: %v", defaultConfigFile, err)
		}
	}

	files := normalizeFiles(appViper.GetStringSlice(keyFiles))
	if len(files) == 0 {
		files = parseCSV(appViper.GetString(keyFiles))
	}
	if len(files) == 0 {
		filePath := strings.TrimSpace(appViper.GetString(keyFile))
		if filePath != "" {
			files = []string{filePath}
		}
	}

	Cfg = AppConfig{
		URL:            appViper.GetString(keyURL),
		Files:          files,
		ChunkSize:      appViper.GetInt(keyChunkSize),
		FieldName:      appViper.GetString(keyField),
		RequestTimeout: appViper.GetDuration(keyRequestTimeout),
		MaxConcurrent:  appViper.GetInt(keyMaxConcurrent),
	}

	if len(Cfg.Files) == 0 {
		log.Panic("invalid client config: files are required")
	}
	if Cfg.URL == "" {
		log.Panic("invalid client config: url is required")
	}
	if Cfg.ChunkSize <= 0 {
		log.Panic("invalid client config: chunk_size must be positive")
	}
	if Cfg.FieldName == "" {
		log.Panic("invalid client config: field is required")
	}
	if Cfg.RequestTimeout <= 0 {
		log.Panic("invalid client config: request_timeout must be positive")
	}
	if Cfg.MaxConcurrent <= 0 {
		log.Panic("invalid client config: max_concurrent_uploads must be positive")
	}
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}

func normalizeFiles(raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, parseCSV(item)...)
	}

	return out
}
