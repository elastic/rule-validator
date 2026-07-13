// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

// The rule-validator command parses and validates fields used in detection rules.
package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"

	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/rule-validator/internal/cache"
)

type config struct {
	Cache string `config:"cache"`
}

var (
	configFile string
	debug      bool
)

func getStringEnv(key string, target *string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}

func getBoolEnv(key string, target *bool) {
	if value, ok := os.LookupEnv(key); ok {
		if v, err := strconv.ParseBool(value); err == nil {
			*target = v
		} else {
			slog.Warn("Unable to parse boolean from environment variable", slog.String("env", key))
		}
	}
}

func parseArgs() {
	flag.StringVar(&configFile, "config", "config.yml", "path to config file")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")

	flag.Parse()
}

func readEnv() {
	getStringEnv("RV_CONFIG_FILE", &configFile)
	getBoolEnv("RV_DEBUG", &debug)
}

func main() {
	parseArgs()
	readEnv()

	rawCfg, err := yaml.NewConfigWithFile(configFile)
	if err != nil {
		slog.Error("Failed to open config file", slog.String("error", err.Error()))
		os.Exit(1)
	}
	var cfg config
	if err = rawCfg.Unpack(&cfg); err != nil {
		slog.Error("Failed to unpack config file", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if cfg.Cache == "" {
		cfg.Cache = ".cache"
	}

	_, err = cache.New(cfg.Cache)
	if err != nil {
		slog.Error("Failed to create cache", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
