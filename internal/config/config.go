package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime configuration for the service.
type Config struct {
	ListenAddr      string
	ProbeListenAddr string
	Namespace       string

	// TLS file paths (only required when TLSEnabled is true).
	TLSCertFile     string
	TLSKeyFile      string
	TLSClientCAFile string

	// TLSEnabled enables HTTPS with mutual TLS on the API listener.
	TLSEnabled bool
}

// Load builds a Config from defaults, environment variables, and flags.
func Load() (Config, error) {
	cfg := Config{
		ListenAddr:      ":8080",
		ProbeListenAddr: ":8081",
		Namespace:       "default",
	}

	// env overrides
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("PROBE_LISTEN_ADDR"); v != "" {
		cfg.ProbeListenAddr = v
	}
	if v := os.Getenv("NAMESPACE"); v != "" {
		cfg.Namespace = v
	}
	if v := os.Getenv("TLS_CERT_FILE"); v != "" {
		cfg.TLSCertFile = v
	}
	if v := os.Getenv("TLS_KEY_FILE"); v != "" {
		cfg.TLSKeyFile = v
	}
	if v := os.Getenv("TLS_CLIENT_CA_FILE"); v != "" {
		cfg.TLSClientCAFile = v
	}
	if v := os.Getenv("TLS_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.TLSEnabled = b
		} else {
			return Config{}, err
		}
	}

	// flags override env
	flag.StringVar(&cfg.ListenAddr, "listen-addr", cfg.ListenAddr, "address to listen on (env: LISTEN_ADDR)")
	flag.StringVar(&cfg.ProbeListenAddr, "probe-listen-addr", cfg.ProbeListenAddr, "address for health probes (env: PROBE_LISTEN_ADDR)")
	flag.StringVar(&cfg.Namespace, "namespace", cfg.Namespace, "kubernetes namespace to target (env: NAMESPACE)")
	flag.StringVar(&cfg.TLSCertFile, "tls-cert-file", cfg.TLSCertFile, "path to server TLS cert (env: TLS_CERT_FILE)")
	flag.StringVar(&cfg.TLSKeyFile, "tls-key-file", cfg.TLSKeyFile, "path to server TLS key (env: TLS_KEY_FILE)")
	flag.StringVar(&cfg.TLSClientCAFile, "tls-client-ca-file", cfg.TLSClientCAFile, "path to client CA bundle (env: TLS_CLIENT_CA_FILE)")
	flag.BoolVar(&cfg.TLSEnabled, "tls-enabled", cfg.TLSEnabled, "enable TLS listener (env: TLS_ENABLED)")
	flag.Parse()

	if cfg.TLSEnabled {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" || cfg.TLSClientCAFile == "" {
			return Config{}, fmt.Errorf("tls enabled but TLS_CERT_FILE, TLS_KEY_FILE, or TLS_CLIENT_CA_FILE is missing")
		}
	}

	return cfg, nil
}
