package config

import (
	"time"

	"github.com/spf13/viper"
)

// setDefaults sets the default configuration values.
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 9000)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 0) // 0 means unlimited
	v.SetDefault("server.idle_timeout", 120*time.Second)
	v.SetDefault("server.read_header_timeout", 10*time.Second)
	v.SetDefault("server.max_header_bytes", 1<<20) // 1MB

	v.SetDefault("server.tls.enabled", false)
	v.SetDefault("server.tls.auto_redirect", false)
	v.SetDefault("server.tls.http_port", 8080)

	v.SetDefault("storage.data_dir", "./data/objects")
	v.SetDefault("storage.temp_dir", "./data/tmp")
	v.SetDefault("storage.meta_db", "./data/meta.db")
	v.SetDefault("storage.max_object_size", 5*1024*1024*1024*1024) // 5TB
	v.SetDefault("storage.multipart_cleanup_days", 7)

	v.SetDefault("auth.region", "us-east-1")
	v.SetDefault("auth.token_expiry_minutes", 15)

	v.SetDefault("cors.global_allowed_origins", []string{"*"})
	v.SetDefault("cors.global_allowed_methods", []string{"GET", "PUT", "POST", "DELETE", "HEAD"})

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")

	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.requests_per_second", 1000.0)
	v.SetDefault("rate_limit.burst", 200)

	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/_metrics")

	v.SetDefault("admin.api_enabled", true)
	v.SetDefault("admin.path_prefix", "/_admin")
}
