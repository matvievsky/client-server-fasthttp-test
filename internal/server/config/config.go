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
	defaultConfigFile           = ".env.server"
	defaultServerAddr           = ":8080"
	defaultServerName           = "multipart-upload-test-server"
	defaultFileField            = "file"
	defaultMaxRequestBodySize   = 1024 * 1024 * 1024 // 1 GiB
	defaultPprofAddr            = ":6060"
	defaultReadTimeout          = 30 * time.Second
	defaultWriteTimeout         = 30 * time.Second
	defaultIdleTimeout          = 60 * time.Second
	defaultMaxConcurrentUploads = 4

	keyAddr                 = "UPLOAD_SERVER_ADDR"
	keyName                 = "UPLOAD_SERVER_NAME"
	keyStreamRequestBody    = "UPLOAD_SERVER_STREAM_REQUEST_BODY"
	keyMaxRequestBodySize   = "UPLOAD_SERVER_MAX_REQUEST_BODY_SIZE"
	keyFileField            = "UPLOAD_SERVER_FILE_FIELD"
	keyPprofEnabled         = "UPLOAD_SERVER_PPROF_ENABLED"
	keyPprofAddr            = "UPLOAD_SERVER_PPROF_ADDR"
	keyReadTimeout          = "UPLOAD_SERVER_READ_TIMEOUT"
	keyWriteTimeout         = "UPLOAD_SERVER_WRITE_TIMEOUT"
	keyIdleTimeout          = "UPLOAD_SERVER_IDLE_TIMEOUT"
	keyMaxConcurrentUploads = "UPLOAD_SERVER_MAX_CONCURRENT_UPLOADS"
)

var Cfg AppConfig

type AppConfig struct {
	Addr                 string
	Name                 string
	StreamRequestBody    bool
	MaxRequestBodySize   int
	FileField            string
	PprofEnabled         bool
	PprofAddr            string
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	MaxConcurrentUploads int
}

func init() {
	appViper := viper.New()

	appViper.AutomaticEnv()

	appViper.SetDefault(keyAddr, defaultServerAddr)
	appViper.SetDefault(keyName, defaultServerName)
	appViper.SetDefault(keyStreamRequestBody, true)
	appViper.SetDefault(keyMaxRequestBodySize, defaultMaxRequestBodySize)
	appViper.SetDefault(keyFileField, defaultFileField)
	appViper.SetDefault(keyPprofEnabled, false)
	appViper.SetDefault(keyPprofAddr, defaultPprofAddr)
	appViper.SetDefault(keyReadTimeout, defaultReadTimeout)
	appViper.SetDefault(keyWriteTimeout, defaultWriteTimeout)
	appViper.SetDefault(keyIdleTimeout, defaultIdleTimeout)
	appViper.SetDefault(keyMaxConcurrentUploads, defaultMaxConcurrentUploads)

	appViper.SetConfigFile(defaultConfigFile)
	appViper.SetConfigType("env")

	if err := appViper.ReadInConfig(); err != nil {
		var configNotFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFoundErr) && !os.IsNotExist(err) {
			log.Panicf("read %s: %v", defaultConfigFile, err)
		}
	}
	Cfg = AppConfig{
		Addr:                 appViper.GetString(keyAddr),
		Name:                 appViper.GetString(keyName),
		StreamRequestBody:    appViper.GetBool(keyStreamRequestBody),
		MaxRequestBodySize:   appViper.GetInt(keyMaxRequestBodySize),
		FileField:            appViper.GetString(keyFileField),
		PprofEnabled:         appViper.GetBool(keyPprofEnabled),
		PprofAddr:            appViper.GetString(keyPprofAddr),
		ReadTimeout:          appViper.GetDuration(keyReadTimeout),
		WriteTimeout:         appViper.GetDuration(keyWriteTimeout),
		IdleTimeout:          appViper.GetDuration(keyIdleTimeout),
		MaxConcurrentUploads: appViper.GetInt(keyMaxConcurrentUploads),
	}

	if strings.TrimSpace(Cfg.Addr) == "" {
		log.Panic("invalid server config: addr is required")
	}
	if strings.TrimSpace(Cfg.Name) == "" {
		log.Panic("invalid server config: name is required")
	}
	if strings.TrimSpace(Cfg.FileField) == "" {
		log.Panic("invalid server config: file_field is required")
	}
	if Cfg.MaxRequestBodySize <= 0 {
		log.Panic("invalid server config: max_request_body_size must be positive")
	}
	if Cfg.PprofEnabled && strings.TrimSpace(Cfg.PprofAddr) == "" {
		log.Panic("invalid server config: pprof_addr is required when pprof_enabled=true")
	}
	if Cfg.ReadTimeout <= 0 {
		log.Panic("invalid server config: read_timeout must be positive")
	}
	if Cfg.WriteTimeout <= 0 {
		log.Panic("invalid server config: write_timeout must be positive")
	}
	if Cfg.IdleTimeout <= 0 {
		log.Panic("invalid server config: idle_timeout must be positive")
	}
	if Cfg.MaxConcurrentUploads <= 0 {
		log.Panic("invalid server config: max_concurrent_uploads must be positive")
	}
}
