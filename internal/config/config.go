package config

import (
	"flag"
	"os"
)

type Config struct {
	ListenAddr string
	Namespace  string

	// TLS file paths (wired in PR4)
	TLSCertFile string
	TLSKeyFile  string
	CACertFile  string

	// Whether to run with TLS enabled (PR4 will enforce mTLS when true)
	TLSEnabled bool
}

func Load() Config {
	cfg := Config{
		ListenAddr: ":8080",
		Namespace:  "default",
	}

	// Defaults can be overridden by env, then flags override env.
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
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
	if v := os.Getenv("CA_CERT_FILE"); v != "" {
		cfg.CACertFile = v
	}
	if v := os.Getenv("TLS_ENABLED"); v == "1" || v == "true" || v == "TRUE" {
		cfg.TLSEnabled = true
	}

	flag.StringVar(&cfg.ListenAddr, "listen-addr", cfg.ListenAddr, "address to listen on (env: LISTEN_ADDR)")
	flag.StringVar(&cfg.Namespace, "namespace", cfg.Namespace, "kubernetes namespace to target (env: NAMESPACE)")
	flag.StringVar(&cfg.TLSCertFile, "tls-cert-file", cfg.TLSCertFile, "path to server TLS cert (env: TLS_CERT_FILE)")
	flag.StringVar(&cfg.TLSKeyFile, "tls-key-file", cfg.TLSKeyFile, "path to server TLS key (env: TLS_KEY_FILE)")
	flag.StringVar(&cfg.CACertFile, "ca-cert-file", cfg.CACertFile, "path to CA cert bundle (env: CA_CERT_FILE)")
	flag.BoolVar(&cfg.TLSEnabled, "tls-enabled", cfg.TLSEnabled, "enable TLS listener (env: TLS_ENABLED)")

	flag.Parse()
	return cfg
}
