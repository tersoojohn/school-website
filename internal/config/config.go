// Package config loads runtime settings from environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr          string
	DBPath        string
	SchoolName    string
	SecureCookies bool
	TrustProxy    bool
	DataKey       []byte
	SessionIdle   time.Duration
	SessionMax    time.Duration

	BootstrapName     string
	BootstrapUser     string
	BootstrapPassword string

	// BaseURL is the public address used in setup links, e.g. https://portal.school.ng.
	// When empty it is derived from each request.
	BaseURL string
	Images  Images
}

// Images are the photographs used on the sign-in page and staff/student headers.
// Each is either an https URL or a path under /static/. They are placeholders:
// point the SCHOOL_IMG_* variables at your own photos, or drop files into web/static/img.
type Images struct {
	Login   string
	Student string
	Staff   string
}

func Load() (*Config, error) {
	c := &Config{
		Addr:              env("SCHOOL_ADDR", ":8080"),
		DBPath:            env("SCHOOL_DB", "data/school.db"),
		SchoolName:        env("SCHOOL_NAME", "School Portal"),
		SecureCookies:     envBool("SCHOOL_SECURE_COOKIES", true),
		TrustProxy:        envBool("SCHOOL_TRUST_PROXY", false),
		SessionIdle:       30 * time.Minute,
		SessionMax:        12 * time.Hour,
		BootstrapName:     env("SCHOOL_BOOTSTRAP_NAME", "Principal"),
		BootstrapUser:     env("SCHOOL_BOOTSTRAP_USER", "principal"),
		BootstrapPassword: os.Getenv("SCHOOL_BOOTSTRAP_PASSWORD"),
		BaseURL:           strings.TrimRight(os.Getenv("SCHOOL_BASE_URL"), "/"),
		Images: Images{
			Login:   imageEnv("SCHOOL_IMG_LOGIN", "https://images.unsplash.com/photo-1473649085228-583485e6e4d7?auto=format&fit=crop&w=1600&q=70"),
			Student: imageEnv("SCHOOL_IMG_STUDENT", "https://images.unsplash.com/photo-1762512346988-045f4d5ad2b3?auto=format&fit=crop&w=1800&q=70"),
			Staff:   imageEnv("SCHOOL_IMG_STAFF", "https://images.unsplash.com/photo-1694180847438-261238ab1710?auto=format&fit=crop&w=1800&q=70"),
		},
	}

	raw := os.Getenv("SCHOOL_DATA_KEY")
	if raw == "" {
		return nil, errors.New("SCHOOL_DATA_KEY is required (generate one with: openssl rand -base64 32)")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("SCHOOL_DATA_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("SCHOOL_DATA_KEY must decode to exactly 32 bytes, got %d", len(key))
	}
	c.DataKey = key
	return c, nil
}

// imageEnv accepts only https URLs or /static/ paths, so a bad value can never inject another scheme.
func imageEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	if strings.HasPrefix(v, "/static/") {
		return v
	}
	if u, err := url.Parse(v); err == nil && u.Scheme == "https" && u.Host != "" {
		return v
	}
	return fallback
}

// ImageHosts lists the external hosts images are loaded from, for the Content-Security-Policy.
func (c *Config) ImageHosts() []string {
	seen := map[string]bool{}
	var hosts []string
	for _, raw := range []string{c.Images.Login, c.Images.Student, c.Images.Staff} {
		if u, err := url.Parse(raw); err == nil && u.Scheme == "https" && !seen[u.Host] {
			seen[u.Host] = true
			hosts = append(hosts, "https://"+u.Host)
		}
	}
	return hosts
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
