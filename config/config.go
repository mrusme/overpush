package config

import (
	"errors"
	"strings"

	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/target"
	"github.com/mrusme/overpush/models/user"
	"github.com/spf13/viper"
)

type Config struct {
	Debug   bool
	Testing bool

	Redis struct {
		Connection  string
		Connections []string
		Username    string
		Password    string
		Cluster     bool
		Failover    bool
		MasterName  string
		Concurrency int
	}

	Server struct {
		Enable bool
		BindIP string
		Port   string

		BodyLimit          int
		Concurrency        int
		ProxyHeader        string
		EnableIPValidation bool
		TrustProxy         bool
		TrustLoopback      bool
		TrustProxies       []string
		ReduceMemoryUsage  bool
		ServerHeader       string

		Limiter struct {
			MaxReqests           int
			PerDurationInSeconds int
			IgnoreFailedRequests bool
			UseRedis             bool
		}
	}

	Worker struct {
		Enable bool
	}

	Database struct {
		Enable     bool
		Connection string
	}

	Users []user.User

	Targets []target.Target
}

func Cfg() (Config, error) {
	viper.SetDefault("Debug", "false")
	viper.SetDefault("Testing", "false")

	viper.SetDefault("Redis.Connection", "localhost:6380")
	viper.SetDefault("Redis.Username", "default")
	viper.SetDefault("Redis.Password", "")
	viper.SetDefault("Redis.Cluster", "false")
	viper.SetDefault("Redis.Failover", "false")
	viper.SetDefault("Redis.MasterName", "")
	viper.SetDefault("Redis.Concurrency", "1")

	viper.SetDefault("Server.Enable", "true")
	viper.SetDefault("Server.BindIP", "127.0.0.1")
	viper.SetDefault("Server.Port", "8080")

	viper.SetDefault("Server.BodyLimit", "4194304")
	viper.SetDefault("Server.Concurrency", "262144")
	viper.SetDefault("Server.ProxyHeader", "")
	viper.SetDefault("Server.EnableIPValidation", "false")
	viper.SetDefault("Server.TrustProxy", "false")
	viper.SetDefault("Server.TrustLoopback", "true")
	viper.SetDefault("Server.TrustProxies", "")
	viper.SetDefault("Server.ReduceMemoryUsage", "false")
	viper.SetDefault("Server.ServerHeader", "AmazonS3")

	viper.SetDefault("Server.Limiter.MaxReqests", "15")
	viper.SetDefault("Server.Limiter.PerDurationInSeconds", "30")
	viper.SetDefault("Server.Limiter.IgnoreFailedRequests", "true")
	viper.SetDefault("Server.Limiter.UseRedis", "false")

	viper.SetDefault("Worker.Enable", "true")

	viper.SetDefault("Database.Enable", "false")
	viper.SetDefault("Database.Connection",
		"postgres://postgres:postgres@localhost/overpush?sslmode=disable")

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

func (cfg *Config) GetUserFromToken(token string) (user.User, error) {
	for _, user := range cfg.Users {
		for _, app := range user.Applications {
			if app.Token == token {
				return user, nil
			}
		}
	}

	return user.User{}, errors.New("No user key for token found")
}

func (cfg *Config) GetApplication(userKey string, token string) (application.Application, error) {
	for _, user := range cfg.Users {
		if user.Key == userKey {
			for _, app := range user.Applications {
				if app.Token == token {
					return app, nil
				}
			}
		}
	}

	return application.Application{}, errors.New("No application for user/token found")
}

func (cfg *Config) GetTargetByID(targetID string) (target.Target, error) {
	for _, target := range cfg.Targets {
		if targetID == target.ID {
			return target, nil
		}
	}

	return target.Target{}, errors.New("No target for targetID found")
}
