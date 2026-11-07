package yandex

import (
	"errors"
	"strings"
	"testing"
)

// The bodies below are verbatim replies of boards.yandex.ru on 2026-11-07 through
// POST /api with {"action":..., "content": base64(JSON)}, asked as a guest.
const (
	// get-whiteboard-info for a real hash whose owner switched guest access off.
	boardsClosedBoardBody = `{"redirect":"https://boards.yandex.ru/restrict?hash=ea1fa3fd647db29f766e06212a553539","error":{"code":212,"description":"Guest mode disabled"}}`
	// get-whiteboard-info for a well-formed hash that names no board.
	boardsNoSuchBoardBody = `{"error":{"code":250,"description":"Presentation not found: presentationHash=0123456789abcdef0123456789abcdef"}}`
	// get-whiteboard-info for an empty or 64-character hash: the format check.
	boardsBadHashBody = `{"error":{"code":200,"description":"hash"}}`
	// request-guest-token on a closed board: HTTP 200, no "error", no Set-Cookie,
	// just a redirect to the site root. This is why a status-code check alone
	// cannot tell a refusal from a success.
	boardsGuestTokenRefusedBody = `{"redirect":"https://boards.yandex.ru"}`
)

// A 200 carries the refusal in "error", so every 200 body has to be decoded
// before the caller reads the reply as a success.
func TestDecodeAPIResponseNamesTheRefusal(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		code   int
		substr []string
	}{
		{
			name:   "guest mode disabled",
			body:   boardsClosedBoardBody,
			code:   212,
			substr: []string{"212", "Guest mode disabled", "closed to guests", "Доступ по ссылке"},
		},
		{
			name:   "board not found",
			body:   boardsNoSuchBoardBody,
			code:   250,
			substr: []string{"250", "no board"},
		},
		{
			name:   "bad hash",
			body:   boardsBadHashBody,
			code:   200,
			substr: []string{"200", "32-character"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := decodeAPIResponse("get-whiteboard-info", []byte(c.body))
			if err == nil {
				t.Fatalf("decodeAPIResponse() = nil error, want a refusal")
			}
			var apiErr *boardsAPIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("errors.As(err, *boardsAPIError) = false, err = %v", err)
			}
			if apiErr.Code != c.code {
				t.Fatalf("Code = %d, want %d", apiErr.Code, c.code)
			}
			for _, want := range c.substr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Error() = %q, want %q in it", err.Error(), want)
				}
			}
		})
	}
}

// request-guest-token on a closed board answers 200 with a redirect and no
// token cookie: no refusal to decode, so the caller compares the redirect with
// the board it asked about instead of reporting a bare "token not found".
func TestDecodeAPIResponseAcceptsARedirectWithoutError(t *testing.T) {
	resp, err := decodeAPIResponse("request-guest-token", []byte(boardsGuestTokenRefusedBody))
	if err != nil {
		t.Fatalf("decodeAPIResponse() = %v, want nil: a redirect is not an error", err)
	}
	if resp.Redirect != "https://boards.yandex.ru" {
		t.Fatalf("Redirect = %q, want the site root", resp.Redirect)
	}
	if strings.Contains(resp.Redirect, "ea1fa3fd647db29f766e06212a553539") {
		t.Fatalf("Redirect = %q, want it not to name the board", resp.Redirect)
	}
}

func TestDecodeAPIResponseIgnoresNonJSONAndSuccess(t *testing.T) {
	cases := map[string]string{
		"html error page": `<html><body>502</body></html>`,
		"empty body":      "",
		"board info":      `{"presentation":{"properties":{"current_slide":"7a1"},"items":""},"socket_servers":[{"ip":"socket33.boards.yandex.ru"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeAPIResponse("get-whiteboard-info", []byte(body)); err != nil {
				t.Fatalf("decodeAPIResponse() = %v, want nil", err)
			}
		})
	}
}
