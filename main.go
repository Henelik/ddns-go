package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func main() {
	configPathPtr := flag.String("config", "ddns-go.yaml", "Path to the ddns-go config")

	flag.Parse()

	config, err := readConfig(*configPathPtr)
	if err != nil {
		panic(errors.Wrap(err, "failed to get config"))
	}

	level, err := zap.ParseAtomicLevel(config.LogLevel)
	if err != nil {
		panic(errors.Wrap(err, "failed to parse logger level"))
	}

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Encoding = "json"
	loggerConfig.Level = level
	loggerConfig.DisableCaller = true

	logger, err := loggerConfig.Build()
	if err != nil {
		panic(errors.Wrap(err, "failed to create logger"))
	}

	logger = logger.With(
		zap.String("host", config.Host),
		zap.String("domain_name", config.DomainName),
	)

	ticker := time.NewTicker(config.UpdatePeriod)

	url := fmt.Sprintf("https://dynamicdns.park-your-domain.com/update?host=%s&domain=%s&password=%s",
		config.Host,
		config.DomainName,
		config.DDNSPassword)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case <-ticker.C:
			resp, err := http.Get(url)
			if err != nil {
				logger.Error("failed to send DNS update request", zap.Error(err))

				continue
			}

			if resp.StatusCode >= 400 {
				body, readErr := io.ReadAll(resp.Body)
				if readErr != nil {
					logger.Error("failed to read response body", zap.Error(readErr))
				}

				logger.Error("got error response from DNS server",
					zap.Int("status_code", resp.StatusCode),
					zap.String("status", resp.Status),
					zap.ByteString("response_body", body))

				continue
			}

			logger.Info("successfully updated DNS entry",
				zap.String("status", resp.Status),
			)

		case <-signalChannel:
			logger.Info("shutting down")
			ticker.Stop()
			os.Exit(0)
		}
	}
}

type Config struct {
	UpdatePeriod time.Duration `yaml:"update_period"`
	Host         string        `yaml:"host"`
	DomainName   string        `yaml:"domain_name"`
	DDNSPassword string        `yaml:"ddns_password"`
	LogLevel     string        `yaml:"log_level"`
}

func readConfig(path string) (*Config, error) {
	configBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}

	result := &Config{}

	if err = yaml.Unmarshal(configBytes, result); err != nil {
		return nil, errors.Wrap(err, "failed to parse config")
	}

	return result, nil
}
