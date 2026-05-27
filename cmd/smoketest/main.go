// Package main — main.go is the entry point for the Smobilpay Go
// smoke-test runner. It resolves the config file (CLI arg →
// SMOBILPAY_SMOKE_CONFIG env → ./smoke-test.json), validates it, builds
// a smobilpay.Client, and dispatches the 15 scenarios in the same order
// as the Java and Node runners so their outputs diff cleanly.
//
// Exit codes:
//
//	0  all non-skipped scenarios passed
//	1  at least one scenario failed
//	2  configuration error before the client could start
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	smob "github.com/maviance/smobilpay-go"
)

const banner = "======================================================================"

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(2)
	}

	clientCfg, err := buildClientConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(2)
	}
	client, err := smob.New(clientCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(2)
	}

	fmt.Println()
	fmt.Println(banner)
	fmt.Printf("Smobilpay smoke test  —  baseUrl=%s, apiVersion=%s, publicKey=%s\n",
		clientCfg.BaseURL, clientCfg.APIVersion, redactKey(clientCfg.PublicKey))
	fmt.Println(banner)

	ctx := context.Background()
	h := &Harness{client: client, cfg: cfg}

	h.run("Ping (auth probe)", func() error { return h.scenarioPing(ctx) })
	h.run("OAuth 2.0 token refresh", func() error { return h.scenarioTokenRefresh(ctx) })
	h.run("Account profile", func() error { return h.scenarioAccount(ctx) })
	h.run("Merchant catalog", func() error { return h.scenarioMerchants(ctx) })
	h.run("Service catalog", func() error { return h.scenarioServices(ctx) })
	h.run("Collection — cash-out (discover + quote)", func() error { return h.scenarioCashout(ctx) })
	h.run("Collection — bill payment (discover + quote)", func() error { return h.scenarioBill(ctx) })
	h.run("Collection — airtime top-up (discover + quote)", func() error { return h.scenarioTopup(ctx) })
	h.run("Collection — voucher purchase (discover + quote)", func() error { return h.scenarioVoucher(ctx) })
	h.run("Collection — product purchase (discover + quote)", func() error { return h.scenarioProduct(ctx) })
	h.run("Collection — subscription top-up (discover + quote)", func() error { return h.scenarioSubscription(ctx) })
	h.run("Disbursement — cash-in (discover + quote)", func() error { return h.scenarioCashin(ctx) })
	h.run("Account validation — verify serviceNumber", func() error { return h.scenarioVerifyServiceNumber(ctx) })
	h.run("Account validation — validate destination", func() error { return h.scenarioValidateAccount(ctx) })
	h.run("History - last 7 days", func() error { return h.scenarioHistoryLast7Days(ctx) })

	fmt.Println(sep)
	fmt.Printf("Summary: %d passed, %d skipped, %d failed\n", h.passed, h.skipped, h.failed)
	fmt.Println(sep)

	if h.failed > 0 {
		os.Exit(1)
	}
}

// loadConfig resolves the config path, reads and unmarshals the JSON,
// then runs Validate(). All failures are wrapped with the path so the
// stderr message tells the operator exactly which file is at fault.
func loadConfig(args []string) (SmokeConfig, error) {
	path := resolveConfigPath(args)
	if _, err := os.Stat(path); err != nil {
		return SmokeConfig{}, fmt.Errorf(
			"config file not found at %s. Pass a path as the first argument, "+
				"set SMOBILPAY_SMOKE_CONFIG, or create ./smoke-test.json "+
				"(see smoke-test.example.json)",
			path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SmokeConfig{}, fmt.Errorf("could not read %s: %w", path, err)
	}
	var cfg SmokeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return SmokeConfig{}, fmt.Errorf("could not parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return SmokeConfig{}, err
	}
	return cfg, nil
}

// resolveConfigPath applies the precedence: CLI arg → env var →
// ./smoke-test.json. An absolute path is returned for the default so
// error messages tell the operator exactly where the runner looked.
func resolveConfigPath(args []string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return args[0]
	}
	if env := strings.TrimSpace(os.Getenv("SMOBILPAY_SMOKE_CONFIG")); env != "" {
		return env
	}
	abs, _ := filepath.Abs("smoke-test.json")
	return abs
}

// buildClientConfig translates a SmokeConfig into a smobilpay.Config.
// APIVersion is only forwarded when set, so the client's default
// (DefaultAPIVersion) applies when the JSON omits it.
func buildClientConfig(c SmokeConfig) (smob.Config, error) {
	opts := []smob.Option{
		smob.WithBaseURL(c.BaseURL),
		smob.WithCredentials(c.PublicKey, c.SecretKey),
	}
	if c.APIVersion != "" {
		opts = append(opts, smob.WithAPIVersion(c.APIVersion))
	}
	return smob.NewConfig(opts...)
}

// redactKey produces a banner-safe rendering of the public key:
// the first 4 + "..." + last 2 characters, or "****" for short keys.
func redactKey(k string) string {
	if len(k) <= 4 {
		return "****"
	}
	return k[:4] + "..." + k[len(k)-2:]
}
