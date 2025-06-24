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
	Password        string `yaml:"password"`
	Name            string `yaml:"name" env-required:"true"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime" env-default:"3600"`
}

type Config struct {
	Env        string   `yaml:"env" env:"ENV" env-required:"true" env-default:"production"`
	Database   Database `yaml:"database" env-required:"true"`
	HTTPServer `yaml:"http_server"`
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
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
