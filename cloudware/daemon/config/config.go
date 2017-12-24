package config

type Config struct {
	Pidfile string `json:"pidfile,omitempty"`
	Root   string  `json:"data-root,omitempty"`
	Debug   bool   `json:"debug,omitempty"`
	LogLevel  string   `json:"log-level,omitempty"`
}

// New returns a new fully initialized Config struct
func New() *Config {
	config := Config{}
	return &config
}
