package guide

import (
	"net/url"
	"regexp"
	"strings"
)

// Host yang diterima, dicocokkan PERSIS (bukan akhiran). Pencocokan akhiran
// akan menerima "youtube.com.evil.example.com", dan pencocokan substring akan
// menerima "https://evil.example.com/youtube.com/watch?v=x".
var youtubeHosts = map[string]bool{
	"youtube.com":     true,
	"www.youtube.com": true,
	"m.youtube.com":   true,
	"youtu.be":        true,
}

// Identitas video YouTube: 11 karakter dari alfabet URL-safe base64.
var youtubeIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseYouTubeID mengekstrak identitas video dari tautan yang ditempel
// pengelola dan menolak apa pun yang bukan tautan YouTube.
//
// Yang disimpan hanya identitasnya, bukan URL mentah dari input: penyematan
// nanti dibangun dari identitas itu, sehingga tidak ada string dari luar yang
// pernah menjadi bagian dari URL yang dirender.
//
// Bentuk yang diterima: /watch?v=<id>, youtu.be/<id>, dan /shorts/<id>, dengan
// atau tanpa www dan m. Parameter lain (t, list, si) dibuang.
func ParseYouTubeID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidVideoURL
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidVideoURL
	}

	// Hanya http(s). Ini yang menutup javascript: dan data:.
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", ErrInvalidVideoURL
	}

	// u.Host sudah memisahkan userinfo, sehingga "https://youtube.com@evil.com/x"
	// dicocokkan sebagai evil.com — yang memang harus ditolak.
	host := strings.ToLower(u.Hostname())
	host = strings.TrimSuffix(host, ".") // FQDN dengan titik di ujung
	if !youtubeHosts[host] {
		return "", ErrInvalidVideoURL
	}

	var id string
	switch {
	case host == "youtu.be":
		id = firstPathSegment(u.Path)
	case u.Path == "/watch":
		id = u.Query().Get("v")
	case strings.HasPrefix(u.Path, "/shorts/"):
		id = firstPathSegment(strings.TrimPrefix(u.Path, "/shorts"))
	default:
		return "", ErrInvalidVideoURL
	}

	if !youtubeIDRe.MatchString(id) {
		return "", ErrInvalidVideoURL
	}
	return id, nil
}

func firstPathSegment(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return p
}
