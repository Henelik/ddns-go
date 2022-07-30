package main

import (
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

func main() {
	configPathPtr := flag.String("config", "/etc/ddns-go.yaml", "Path to the ddns-go config")

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

	for {
		select {
		case <-ticker.C:
			resp, err := http.Get(url)
			if err != nil {
				logger.Error("failed to send DNS update request", zap.Error(err))
			}

			if resp.StatusCode >= 400 {
				body, _ := ioutil.ReadAll(resp.Body)

				logger.Error("got error response from DNS server",
					zap.Int("status_code", resp.StatusCode),
					zap.String("status", resp.Status),
					zap.ByteString("response_body", body))
			}

			logger.Info("successfully updated DNS entry")
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
