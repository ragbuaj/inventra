// Package geoip resolves a coarse city/country for an IP address, used to give
// a device-session a human-readable location. It is optional: without a
// provisioned MaxMind GeoLite2 database it degrades to a no-op locator that
// returns nothing, so callers must treat an empty result as "unknown" and fall
// back to the raw IP (mirroring the email package's LogSender fallback).
package geoip

import (
	"log/slog"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// Locator resolves an IP to a coarse location. Implementations must be safe for
// concurrent use and must never panic on malformed, private, or loopback input.
type Locator interface {
	// Lookup returns a best-effort city and country for the IP. Either or both
	// may be empty when unresolved (private/loopback IP, missing DB, no match).
	Lookup(ip string) (city, country string)
	// Close releases any underlying resources.
	Close() error
}

// New selects a locator: an mmdb-backed one when dbPath points at a readable
// GeoLite2 database, otherwise a no-op. It never returns an error — a bad path
// logs a warning and yields the no-op locator so startup is never blocked by an
// optional feature.
func New(dbPath string, logger *slog.Logger) Locator {
	if logger == nil {
		logger = slog.Default()
	}
	if dbPath == "" {
		logger.Info("geoip: no GEOIP_DB_PATH set, device-session locations disabled")
		return noopLocator{}
	}
	reader, err := geoip2.Open(dbPath)
	if err != nil {
		logger.Warn("geoip: failed to open database, locations disabled", "path", dbPath, "error", err)
		return noopLocator{}
	}
	logger.Info("geoip: database loaded", "path", dbPath)
	return &mmdbLocator{reader: reader}
}

// noopLocator resolves nothing (dev/CI without a DB).
type noopLocator struct{}

func (noopLocator) Lookup(string) (string, string) { return "", "" }
func (noopLocator) Close() error                   { return nil }

// mmdbLocator resolves via a MaxMind GeoLite2-City database. The underlying
// reader is safe for concurrent reads.
type mmdbLocator struct {
	reader *geoip2.Reader
}

func (l *mmdbLocator) Lookup(ip string) (string, string) {
	parsed := net.ParseIP(ip)
	if parsed == nil || !parsed.IsGlobalUnicast() || parsed.IsPrivate() {
		return "", "" // unparseable, loopback, link-local, or private — nothing to resolve
	}
	rec, err := l.reader.City(parsed)
	if err != nil || rec == nil {
		return "", ""
	}
	city := rec.City.Names["en"]
	country := rec.Country.Names["en"]
	if country == "" {
		country = rec.Country.IsoCode
	}
	return city, country
}

func (l *mmdbLocator) Close() error {
	if l.reader == nil {
		return nil
	}
	return l.reader.Close()
}
