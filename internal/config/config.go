package config

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type HTTPServer struct {
	Addr string `yaml:"address" env-required:"true"`
}

type Database struct {
	Host            string `yaml:"host" env-required:"true"`
	Port            int    `yaml:"port" env-required:"true"`
	User            string `yaml:"user" env-required:"true"`
	Password        string `yaml:"password" env-required:"true"`
	Name            string `yaml:"name" env-required:"true"`
	Charset         string `yaml:"charset" env-default:"utf8mb4"`
	Collation       string `yaml:"collation" env-default:"utf8mb4_general_ci"`
	MaxIdleConns    int    `yaml:"max_idle_conns" env-default:"10"`
	MaxOpenConns    int    `yaml:"max_open_conns" env-default:"100"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime" env-default:"3600"`
}

type Config struct {
	Env        string   `yaml:"env" env:"ENV" env-required:"true" env-default:"production"`
	Database   Database `yaml:"database" env-required:"true"`
	HTTPServer `yaml:"http_server"`
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&collation=%s&parseTime=true",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
		c.Database.Charset,
		c.Database.Collation,
	)
}

func MustLoad() *Config {
	var configPath string

	configPath = os.Getenv("CONFIG_PATH")

	if configPath == "" {
		flags := flag.String("config", "", "path to config file")
		flag.Parse()
		configPath = *flags

		if configPath == "" {
			log.Fatal("Config path is not set")
		}
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist: %s", configPath)
	}

	var cfg Config

	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		log.Fatalf("Unable to load config: %s", err.Error())
	}

	return &cfg
}
