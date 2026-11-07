package yandex

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveCookieFileRoundTripsThroughTheLoader pins the contract the exit relies
// on to survive a restart: what SaveCookieFile writes must be exactly what
// LoadCookieFile can read back. Volga rotates session cookies on nearly every
// request, so without this the file on disk is a snapshot the carrier has
// already replaced and a restart comes back with "login required".
func TestSaveCookieFileRoundTripsThroughTheLoader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies-yandex.txt")
	docURL := "https://docs.yandex.ru/edit/d/AbCdEf"
	hosts := []string{"https://yandex.ru", "https://docs.yandex.ru"}
	want := map[string]string{
		"Session_id": "3:1700000000.5.0.1:abcdef",
		"sessionid2": "77:1700000000.5.0.1:xyz",
		"_yasc":      "Qz9example",
	}

	source, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new jar: %v", err)
	}
	for _, host := range hosts {
		h, err := url.Parse(host)
		if err != nil {
			t.Fatalf("parse %s: %v", host, err)
		}
		for name, value := range want {
			source.SetCookies(h, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
		}
	}

	transport := &YandexVolgaTransport{docURL: docURL}
	transport.jarMu.Lock()
	transport.cookieJar = source
	transport.jarMu.Unlock()

	if err := transport.SaveCookieFile(path); err != nil {
		t.Fatalf("SaveCookieFile: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(raw), "Session_id=") {
		t.Fatalf("saved file carries no session cookies: %q", string(raw))
	}

	reloaded, _ := cookiejar.New(nil)
	if err := loadYandexCookies(path, reloaded); err != nil {
		t.Fatalf("loadYandexCookies: %v", err)
	}
	got := make(map[string]string)
	for _, host := range hosts {
		h, _ := url.Parse(host)
		for _, c := range reloaded.Cookies(h) {
			got[c.Name] = c.Value
		}
	}
	for name, value := range want {
		if got[name] != value {
			t.Fatalf("cookie %s = %q after round trip, want %q", name, got[name], value)
		}
	}
}
