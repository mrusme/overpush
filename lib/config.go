package lib

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Debug string

	Redis struct {
		Connection string
		Username   string
		Password   string
	}

	Server struct {
		BindIP string
		Port   string
	}

	Users []struct {
		Key          string
		Applications []struct {
			Token    string
			Name     string
			IconPath string

			Target string
		}
	}

	Targets []struct {
		ID   string
		Type string
		Args map[string]string
	}
}

func Cfg() (Config, error) {
	viper.SetDefault("Debug", "false")

	viper.SetDefault("Redis.Connection", "localhost:6380")
	viper.SetDefault("Redis.Username", "default")
	viper.SetDefault("Redis.Password", "")

	viper.SetDefault("Server.BindIP", "127.0.0.1")
	viper.SetDefault("Server.Port", "8080")

	viper.SetConfigName("overpush.toml")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/")
	viper.AddConfigPath("$XDG_CONFIG_HOME/")
	viper.AddConfigPath("$HOME/.config/")
	viper.AddConfigPath("$HOME/")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("overpush")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}