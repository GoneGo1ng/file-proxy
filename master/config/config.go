package config

import (
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

func Load(s string) (*Config, error) {
	cfg := &Config{}
	*cfg = DefaultConfig

	err := yaml.UnmarshalStrict([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadFile(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg, err := Load(string(content))
	if err != nil {
		return nil, errors.Wrapf(err, "parsing YAML file %s", filename)
	}
	if err = cfg.setLog(); err != nil {
		return nil, err
	}
	return cfg, nil
}

var (
	DefaultConfig = Config{
		ServerConfig: DefaultServerConfig,
	}

	DefaultServerConfig = ServerConfig{
		TcpAddress:  "0.0.0.0:9630",
		HttpAddress: "0.0.0.0:9631",
		Hostname:    "",
	}

	DefaultLogConfig = LogConfig{
		Level:      "debug",
		OutputPath: "stdout",
	}
)

type Config struct {
	ServerConfig ServerConfig `yaml:"server"`
	LogConfig    LogConfig    `yaml:"log,omitempty"`
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if c.ServerConfig.isZero() {
		c.ServerConfig = DefaultServerConfig
	}

	return nil
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}

type ServerConfig struct {
	TcpAddress  string `yaml:"tcp_address,omitempty"`
	HttpAddress string `yaml:"http_address,omitempty"`
	Hostname    string `yaml:"hostname,omitempty"`
}

func (c *ServerConfig) isZero() bool {
	return c.TcpAddress == "" &&
		c.HttpAddress == "" &&
		c.Hostname == ""
}

func (c *ServerConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultServerConfig
	type plain ServerConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if c.Hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return errors.Errorf("error getting hostname: %q", err)
		}
		c.Hostname = hostname
	}

	return nil
}

type LogConfig struct {
	Level      string `yaml:"level,omitempty"`
	OutputPath string `yaml:"output_path,omitempty"`
}

func (c *LogConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultLogConfig
	type plain LogConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return nil
}

func (c *Config) setLog() error {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel) // TODO
	cfg.OutputPaths = []string{c.LogConfig.OutputPath}
	logger, err := cfg.Build()
	if err != nil {
		return errors.Wrap(err, "Build logger")
	}
	zap.ReplaceGlobals(logger)
	return nil
}
