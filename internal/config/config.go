// Package config loads btcdwatchd configuration from an optional YAML file
// with BTCDWATCH_* environment-variable overrides (env wins). Credentials
// are expected to arrive via the environment in development; the example
// config ships placeholders only.
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Listen string `yaml:"listen"`

	// Public-exposure hardening; every field's zero value disables it,
	// which is right for localhost use. Set all of them when the server
	// faces the internet — see config.example.yaml.
	RateLimitPerMin    int    `yaml:"rate_limit_per_min"`
	RateLimitBurst     int    `yaml:"rate_limit_burst"`
	TrustedProxyHeader string `yaml:"trusted_proxy_header"`
	MaxWSClients       int    `yaml:"max_ws_clients"`
}

type Node struct {
	Network string `yaml:"network"`
	RPCHost string `yaml:"rpc_host"`
	RPCUser string `yaml:"rpc_user"`
	RPCPass string `yaml:"rpc_pass"`
	RPCCert string `yaml:"rpc_cert"`
	// RPCNoTLS matches a btcd running notls=1 (plaintext RPC on
	// loopback); rpc_cert is ignored when set.
	RPCNoTLS bool `yaml:"rpc_notls"`
}

type Price struct {
	Source         string  `yaml:"source"`
	StaticUSD      float64 `yaml:"static_usd"`
	RefreshSeconds int     `yaml:"refresh_seconds"`
}

type Fees struct {
	FloorSlow     float64 `yaml:"floor_slow"`
	FloorStandard float64 `yaml:"floor_standard"`
	FloorUrgent   float64 `yaml:"floor_urgent"`
}

type Address struct {
	MaxScanTxs int `yaml:"max_scan_txs"`
	// MaxConcurrentScans bounds simultaneous address history scans
	// (0 = unlimited) — the most node-expensive request type.
	MaxConcurrentScans int `yaml:"max_concurrent_scans"`
}

type Config struct {
	Server  Server  `yaml:"server"`
	Node    Node    `yaml:"node"`
	Price   Price   `yaml:"price"`
	Fees    Fees    `yaml:"fees"`
	Address Address `yaml:"address"`
}

// Defaults returns the built-in configuration: regtest against a local
// btcd, listening on localhost.
func Defaults() *Config {
	return &Config{
		Server: Server{Listen: "127.0.0.1:8480"},
		Node: Node{
			Network: "regtest",
			RPCHost: "127.0.0.1:18334",
		},
		Price: Price{
			Source:         "coingecko",
			StaticUSD:      98000,
			RefreshSeconds: 60,
		},
		Fees: Fees{
			FloorSlow:     1,
			FloorStandard: 2,
			FloorUrgent:   5,
		},
		Address: Address{MaxScanTxs: 2000},
	}
}

// Load reads the YAML file at path (skipped when path is empty), then
// applies environment overrides, then validates.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	if err := applyEnv(cfg); err != nil {
		return nil, err
	}

	if cfg.Node.RPCUser == "" || cfg.Node.RPCPass == "" {
		return nil, fmt.Errorf("node RPC credentials required (set " +
			"BTCDWATCH_RPC_USER / BTCDWATCH_RPC_PASS or " +
			"node.rpc_user / node.rpc_pass)")
	}

	return cfg, nil
}

func applyEnv(cfg *Config) error {
	str := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok {
			*dst = v
		}
	}

	var err error
	boolean := func(key string, dst *bool) {
		v, ok := os.LookupEnv(key)
		if !ok {
			return
		}
		b, perr := strconv.ParseBool(v)
		if perr != nil && err == nil {
			err = fmt.Errorf("%s: invalid boolean %q", key, v)
			return
		}
		*dst = b
	}
	num := func(key string, set func(float64)) {
		v, ok := os.LookupEnv(key)
		if !ok {
			return
		}
		f, perr := strconv.ParseFloat(v, 64)
		if perr != nil && err == nil {
			err = fmt.Errorf("%s: invalid number %q", key, v)
			return
		}
		set(f)
	}

	str("BTCDWATCH_LISTEN", &cfg.Server.Listen)
	str("BTCDWATCH_TRUSTED_PROXY_HEADER", &cfg.Server.TrustedProxyHeader)
	str("BTCDWATCH_NETWORK", &cfg.Node.Network)
	str("BTCDWATCH_RPC_HOST", &cfg.Node.RPCHost)
	str("BTCDWATCH_RPC_USER", &cfg.Node.RPCUser)
	str("BTCDWATCH_RPC_PASS", &cfg.Node.RPCPass)
	str("BTCDWATCH_RPC_CERT", &cfg.Node.RPCCert)
	boolean("BTCDWATCH_RPC_NOTLS", &cfg.Node.RPCNoTLS)
	str("BTCDWATCH_PRICE_SOURCE", &cfg.Price.Source)

	num("BTCDWATCH_PRICE_STATIC_USD", func(f float64) { cfg.Price.StaticUSD = f })
	num("BTCDWATCH_PRICE_REFRESH_SECONDS", func(f float64) { cfg.Price.RefreshSeconds = int(f) })
	num("BTCDWATCH_FEES_FLOOR_SLOW", func(f float64) { cfg.Fees.FloorSlow = f })
	num("BTCDWATCH_FEES_FLOOR_STANDARD", func(f float64) { cfg.Fees.FloorStandard = f })
	num("BTCDWATCH_FEES_FLOOR_URGENT", func(f float64) { cfg.Fees.FloorUrgent = f })
	num("BTCDWATCH_ADDRESS_MAX_SCAN_TXS", func(f float64) { cfg.Address.MaxScanTxs = int(f) })
	num("BTCDWATCH_RATE_LIMIT_PER_MIN", func(f float64) { cfg.Server.RateLimitPerMin = int(f) })
	num("BTCDWATCH_RATE_LIMIT_BURST", func(f float64) { cfg.Server.RateLimitBurst = int(f) })
	num("BTCDWATCH_MAX_WS_CLIENTS", func(f float64) { cfg.Server.MaxWSClients = int(f) })
	num("BTCDWATCH_ADDRESS_MAX_CONCURRENT_SCANS", func(f float64) { cfg.Address.MaxConcurrentScans = int(f) })

	return err
}
