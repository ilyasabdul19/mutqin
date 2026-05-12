// api/cmd/onboard/main.go
//
// onboard creates a proxied Cloudflare A record for a tenant slug.
// Usage:
//   onboard --slug=alfalah
//
// Reads CF_DNS_API_TOKEN, CF_ZONE_ID, CF_RECORD_TARGET_IP, CF_RECORD_BASE_DOMAIN
// from env. Idempotent — re-running is safe.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ilyas/mutqin-api/internal/cloudflare"
	"github.com/ilyas/mutqin-api/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slug := flag.String("slug", "", "tenant slug (lowercase, alphanumeric + dash)")
	flag.Parse()

	if *slug == "" {
		slog.Error("--slug is required")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	if cfg.CFAPIToken == "" || cfg.CFZoneID == "" || cfg.CFRecordTargetIP == "" {
		slog.Error("missing Cloudflare config",
			"have_token", cfg.CFAPIToken != "",
			"have_zone", cfg.CFZoneID != "",
			"have_target_ip", cfg.CFRecordTargetIP != "",
		)
		os.Exit(1)
	}

	name := fmt.Sprintf("%s.%s", *slug, cfg.CFRecordBaseDomain)
	c := cloudflare.New(cfg.CFAPIToken, cfg.CFZoneID, cloudflare.DefaultBaseURL)

	id, err := c.CreateProxiedARecord(context.Background(), name, cfg.CFRecordTargetIP)
	if err != nil {
		slog.Error("create record", "name", name, "error", err)
		os.Exit(1)
	}
	if id == "" {
		slog.Info("record already exists (no-op)", "name", name)
	} else {
		slog.Info("record created", "name", name, "id", id, "target", cfg.CFRecordTargetIP)
	}
}
