package lib

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"text/template"

	"github.com/Jeffail/gabs/v2"
	"github.com/spf13/viper"
)

type CFormat struct {
	Attachment       string
	AttachmentBase64 string
	AttachmentType   string
	Device           string
	HTML             string
	Message          string
	Priority         string
	TTL              string
	Timestamp        string
	Title            string
	URL              string
	URLTitle         string
}

func (cf *CFormat) GetLocationAndPath(str string) (string, string) {
	loc, path, found := strings.Cut(str, ".")
	if !found {
		return "body", str
	}
	return loc, path
}

func (cf *CFormat) GetValue(
	locations map[string]*gabs.Container,
	tmplstr string,
) (string, bool) {
	if tmplstr == "" {
		return "", false
	}

	funcs := template.FuncMap{
		"webhook": func(fullpath string) any {
			loc, path := cf.GetLocationAndPath(fullpath)

			location, ok := locations[loc]
			if !ok {
				return ""
			}

			locctr := location.Path(path)
			if locctr == nil {
				return ""
			}

			locctrData := locctr.Data()
			if locctrData == nil {
				return ""
			}
			locctrType := reflect.TypeOf(locctrData).Kind()
			switch locctrType {
			case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
				if reflect.ValueOf(locctrData).IsNil() {
					return ""
				}
			}

			if locctrType == reflect.String {
				return locctrData.(string)
			}

			return locctr.String()
		},
	}

	tmpl, err := template.New("field").Funcs(funcs).Parse(tmplstr)
	if err != nil {
		return "", false
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		return "", false
	}

	return buf.String(), true
}

type Application struct {
	Token        string
	Name         string
	IconPath     string
	Format       string
	CustomFormat CFormat

	Target string
}

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
		BindIP string
		Port   string
	}

	Users []struct {
		Key          string
		Applications []Application
	}

	Targets []struct {
		ID   string
		Type string
		Args map[string]string
	}
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

func (cfg *Config) GetUserKeyFromToken(token string) (string, error) {
	for _, user := range cfg.Users {
		for _, app := range user.Applications {
			if app.Token == token {
				return user.Key, nil
			}
		}
	}

	return "", errors.New("No user key for token found")
}

func (cfg *Config) GetApplication(userKey string, token string) (Application, error) {
	for _, user := range cfg.Users {
		if user.Key == userKey {
			for _, app := range user.Applications {
				if app.Token == token {
					return app, nil
				}
			}
		}
	}

	return Application{}, errors.New("No application for user/token found")
}
