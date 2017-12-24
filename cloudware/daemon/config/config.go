package config

type Config struct {
	Pidfile string `json:"pidfile,omitempty"`
	Debug   bool   `json:"debug,omitempty"`
}
