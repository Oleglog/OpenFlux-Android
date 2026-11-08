package yandex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
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

// wbpost() re-sends the csrf it got from an earlier /api reply, so the guest
// join has to be able to read it off the board info reply.
func TestDecodeAPIResponseCarriesCSRF(t *testing.T) {
	resp, err := decodeAPIResponse("get-whiteboard-info",
		[]byte(`{"csrf":"d6b1a2","presentation":{"properties":{"current_slide":"7a1"}}}`))
	if err != nil {
		t.Fatalf("decodeAPIResponse() = %v, want nil", err)
	}
	if resp.CSRF != "d6b1a2" {
		t.Fatalf("CSRF = %q, want %q", resp.CSRF, "d6b1a2")
	}
}

// TestBoardsGuestLandingIsNotARefusal pins the difference between "you are not
// signed in" and "this board is closed": an anonymous info call is answered with
// code 204 and a redirect to /guest, and the guest session is minted right after
// by request-guest-token. Treating 204 as fatal made every board with link access
// open look closed (measured 2026-11-08). Code 212 with a redirect to /restrict
// stays a refusal.
func TestBoardsGuestLandingIsNotARefusal(t *testing.T) {
	cases := []struct {
		name string
		err  *boardsAPIError
		want bool
	}{
		{"code 204", &boardsAPIError{Code: 204, Redirect: "https://boards.yandex.ru/guest?hash=abc"}, true},
		{"redirect to guest without code", &boardsAPIError{Code: 0, Redirect: "https://boards.yandex.ru/guest?hash=abc"}, true},
		{"guest mode disabled", &boardsAPIError{Code: 212, Description: "Guest mode disabled", Redirect: "https://boards.yandex.ru/restrict?hash=abc"}, false},
		{"hash names no board", &boardsAPIError{Code: 250, Description: "Presentation not found"}, false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := boardsGuestLanding(c.err); got != c.want {
			t.Fatalf("%s: boardsGuestLanding() = %v, want %v", c.name, got, c.want)
		}
	}
	// errors.As must see the refusal through the wrapping the caller does.
	wrapped := fmt.Errorf("/api get-whiteboard-info: %w", &boardsAPIError{Code: 204})
	var apiErr *boardsAPIError
	if !errors.As(wrapped, &apiErr) || !boardsGuestLanding(apiErr) {
		t.Fatal("a wrapped 204 must still be recognised as the guest landing")
	}
}
// The anonymous info reply carries no board, so the dashboard and the shard
// only appear in the reply fetched with the guest token; the merge must take
// them without letting an absent field erase what the first reply knew.
func TestMergeBoardStateKeepsTokenScopedFields(t *testing.T) {
	state := map[string]string{"dashboard": "", "ws_host": "socket33.boards.yandex.ru"}
	mergeBoardState(state, map[string]string{
		"dashboard":     "3f9d53a03819d058cbcb2503d8413d39",
		"current_slide": "3f9d53a03819d058cbcb2503d8413d39",
		"ws_host":       "socket62.boards.yandex.ru",
		"csrf":          "",
	})
	if state["dashboard"] != "3f9d53a03819d058cbcb2503d8413d39" {
		t.Fatalf("dashboard = %q, want the slide from the token reply", state["dashboard"])
	}
	if state["ws_host"] != "socket62.boards.yandex.ru" {
		t.Fatalf("ws_host = %q, want the shard from the token reply", state["ws_host"])
	}
	if _, ok := state["csrf"]; ok {
		t.Fatalf("csrf = %q, want an empty field not to be copied", state["csrf"])
	}

	mergeBoardState(state, map[string]string{"dashboard": ""})
	if state["dashboard"] == "" {
		t.Fatalf("dashboard was erased by a reply that did not name one")
	}
}

// A dashboard room lives on one socket shard: two guests that dial the site
// host are load-balanced onto different front pods and never see each other's
// objects (measured 2026-11-08). So the board's own shard has to be tried
// first, ahead of whatever endpoint happened to work last time.
func TestSocketURLsPreferTheBoardsShard(t *testing.T) {
	tr := &BoardsTransport{}
	tr.goodSocketURL.Store("wss://boards.yandex.ru/socket.io/?EIO=4&transport=websocket")

	urls := tr.socketURLs(boardsInfo{wsHost: "socket62.boards.yandex.ru"})
	if len(urls) != 2 {
		t.Fatalf("socketURLs() = %v, want the shard and the site host", urls)
	}
	if urls[0] != "wss://socket62.boards.yandex.ru/socket.io/?EIO=4&transport=websocket" {
		t.Fatalf("first candidate = %q, want the board shard", urls[0])
	}
	if urls[1] != "wss://boards.yandex.ru/socket.io/?EIO=4&transport=websocket" {
		t.Fatalf("second candidate = %q, want the site host", urls[1])
	}

	// A remembered shard is still tried when the API names another one, and it
	// is not duplicated.
	tr.goodSocketURL.Store("wss://socket61.boards.yandex.ru/socket.io/?EIO=4&transport=websocket")
	urls = tr.socketURLs(boardsInfo{wsHost: "socket62.boards.yandex.ru"})
	if len(urls) != 3 {
		t.Fatalf("socketURLs() = %v, want shard, remembered shard and site host", urls)
	}
	if urls[1] != "wss://socket61.boards.yandex.ru/socket.io/?EIO=4&transport=websocket" {
		t.Fatalf("second candidate = %q, want the remembered shard", urls[1])
	}
}

// One modify-objects event carries the whole batch, and the board refuses a
// value above 48 KiB of payload (PAYLOAD_TOO_LARGE, measured 2026-11-08), so
// the batch hint must stay under that ceiling or a burst loses its tail. The
// linger window is what decides how many objects a second the board sees: at the
// old 20 ms a browsing tunnel wrote ~50 objects a second and a large share of
// them came back RATE_LIMITED.
func TestBoardsBatchFitsOneObject(t *testing.T) {
	if got := (&BoardsTransport{}).PreferredBatchBytes(); got <= 0 || got > boardsMaxObjectBytes {
		t.Fatalf("PreferredBatchBytes() = %d, want it within (0, %d]", got, boardsMaxObjectBytes)
	}
	if got := (&BoardsTransport{}).PreferredBatchBytes(); got != boardsBatchBytes {
		t.Fatalf("PreferredBatchBytes() = %d, want the board's own ceiling of %d", got, boardsBatchBytes)
	}
	if got := (&BoardsTransport{}).PreferredBatchLingerMs(); got < 250 {
		t.Fatalf("PreferredBatchLingerMs() = %d, want at least 250 ms: the deadline is what fills an object, and 100 ms at the old pace left 1.6 KiB objects and 20 KB/s", got)
	}
}

// The carrier is only as fast as objects per second x object size, and the object
// size is roughly the carrier rate times the batch deadline. If events/s x linger
// (in seconds) is near 1, a busy writer drains batches slightly faster than they
// can fill, objects stay tiny and the tunnel collapses - which is exactly what
// v1.0.52's 18/s with a 100 ms deadline did.
func TestBoardsBatchPipelineFillsAtTheEventCeiling(t *testing.T) {
	linger := float64((&BoardsTransport{}).PreferredBatchLingerMs()) / 1000
	if objects := float64(boardsRateMax) * linger; objects < 4 {
		t.Fatalf("event ceiling %v/s x linger %vs = %v objects per window, want at least 4 or batches never fill", boardsRateMax, linger, objects)
	}
	if got := float64(boardsRateMax) * float64(boardsBatchBytes); got < 500_000 {
		t.Fatalf("event ceiling %v/s x %d bytes = %v B/s at full objects, want at least 500 KB/s (the stand measured 641 KB/s)", boardsRateMax, boardsBatchBytes, got)
	}
}

// Deletions must not delete an object the other peer may still be reading off the
// board, and must not spend an event each: both want a window measured in
// seconds, not in milliseconds.
func TestBoardsDropWindowIsMeasuredInSeconds(t *testing.T) {
	if boardsDropWindow < time.Second {
		t.Fatalf("boardsDropWindow = %v, want at least a second", boardsDropWindow)
	}
	if boardsDropQueueMax <= 0 {
		t.Fatalf("boardsDropQueueMax = %d, want a bound on the queued deletions", boardsDropQueueMax)
	}
}

// A drop-objects event echoes the value it deletes, so the budget for one batched
// deletion has to hold one full-size object on its own: the board's ceiling is
// 48 KiB of payload, i.e. 65536 base64 characters in the value.
func TestBoardsDropBudgetHoldsOneFullObject(t *testing.T) {
	if boardsDropEventBytes <= 65536 {
		t.Fatalf("boardsDropEventBytes = %d, want more than the 65536 value characters of a full-size object", boardsDropEventBytes)
	}
	if boardsDropMaxObjects > 256 {
		t.Fatalf("boardsDropMaxObjects = %d, want a bounded batch", boardsDropMaxObjects)
	}
}

// ---- запись: темп, отказы и склейка ----

type fakeBoardsClock struct{ at time.Time }

func (c *fakeBoardsClock) now() time.Time { return c.at }

func newBoardsPacerOn(clk *fakeBoardsClock) *boardsPacer {
	return &boardsPacer{now: clk.now}
}

// The pace starts conservative and climbs one acknowledged object at a time up
// to the ceiling, so the board is probed rather than flooded.
func TestBoardsPacerStartsSlowAndClimbsToTheCeiling(t *testing.T) {
	clk := &fakeBoardsClock{at: time.Unix(1_700_000_000, 0)}
	p := newBoardsPacerOn(clk)

	if got := p.currentRate(); got != boardsRateStart {
		t.Fatalf("currentRate() = %v, want the conservative start %v", got, boardsRateStart)
	}
	// The first write may go at once; the following slots are one interval apart.
	if d := p.reserve(0); d != 0 {
		t.Fatalf("first reserve() = %v, want no wait", d)
	}
	if d := p.reserve(0); d != 200*time.Millisecond {
		t.Fatalf("second reserve() = %v, want one 200 ms interval at %v/s", d, boardsRateStart)
	}
	if d := p.reserve(0); d != 400*time.Millisecond {
		t.Fatalf("third reserve() = %v, want the second interval queued behind the first", d)
	}

	for i := 0; i < 200; i++ {
		p.onSuccess()
	}
	if got := p.currentRate(); got != boardsRateMax {
		t.Fatalf("currentRate() after 200 acknowledged objects = %v, want the ceiling %v", got, boardsRateMax)
	}
}

// A refusal halves the pace and holds the next slot back; the floor keeps the
// tunnel moving instead of stopping it.
func TestBoardsPacerHalvesOnRateLimitAndFloors(t *testing.T) {
	clk := &fakeBoardsClock{at: time.Unix(1_700_000_000, 0)}
	p := newBoardsPacerOn(clk)
	p.reserve(0)

	first := p.onRateLimited()
	if first >= boardsRateStart {
		t.Fatalf("rate after RATE_LIMITED = %v, want it below the start %v", first, boardsRateStart)
	}
	if d := p.reserve(0); d < boardsRateHold {
		t.Fatalf("reserve() after RATE_LIMITED = %v, want at least the %v hold", d, boardsRateHold)
	}

	for i := 0; i < 20; i++ {
		p.onRateLimited()
	}
	if got := p.currentRate(); got != boardsRateMin {
		t.Fatalf("currentRate() after repeated refusals = %v, want the floor %v", got, boardsRateMin)
	}

	// Recovery is additive and slow: one acknowledged object does not undo a cut.
	before := p.currentRate()
	p.onSuccess()
	if after := p.currentRate(); after != before+boardsRateGrow {
		t.Fatalf("rate after one success = %v, want %v", after, before+boardsRateGrow)
	}
}

// A slot costs the object's bytes as well as an event: at the ceiling a full
// 48 KiB object is charged 48 KiB of the 1.5 MB/s byte budget. Filling the object
// is therefore what buys throughput, and a 1.6 KiB object is still bounded by the
// event budget, not by bytes.
func TestBoardsPacerChargesTheObjectBytes(t *testing.T) {
	ceilingPacer := func() *boardsPacer {
		p := newBoardsPacerOn(&fakeBoardsClock{at: time.Unix(1_700_000_000, 0)})
		for i := 0; i < 200; i++ {
			p.onSuccess()
		}
		if got := p.currentRate(); got != boardsRateMax {
			t.Fatalf("currentRate() = %v, want the ceiling %v", got, boardsRateMax)
		}
		return p
	}

	events := ceilingPacer()
	events.reserve(0)
	eventGap := events.reserve(0)
	if eventGap < 20*time.Millisecond || eventGap > 21*time.Millisecond {
		t.Fatalf("object slot = %v, want ~20.8 ms at the %v/s ceiling", eventGap, boardsRateMax)
	}

	bytes := ceilingPacer()
	bytes.reserve(boardsBatchBytes)
	byteGap := bytes.reserve(boardsBatchBytes)
	if byteGap < 31*time.Millisecond || byteGap > 33*time.Millisecond {
		t.Fatalf("full-object slot = %v, want ~32 ms (%d bytes at %v B/s)", byteGap, boardsBatchBytes, boardsRateBytes)
	}
	if byteGap <= eventGap {
		t.Fatalf("full object waited %v, want more than the %v event slot", byteGap, eventGap)
	}
}

// A board whose budget is below the floor cannot be met by halving the rate any
// further, so the hold itself backs off instead of letting a refusal storm run.
func TestBoardsPacerBacksOffTheHoldAtTheFloor(t *testing.T) {
	clk := &fakeBoardsClock{at: time.Unix(1_700_000_000, 0)}
	p := newBoardsPacerOn(clk)
	p.reserve(0)
	for i := 0; i < 6; i++ {
		p.onRateLimited()
	}
	if got := p.currentRate(); got != boardsRateMin {
		t.Fatalf("currentRate() = %v, want the floor %v", got, boardsRateMin)
	}
	if d := p.reserve(0); d < boardsRateHoldMax {
		t.Fatalf("reserve() at the floor = %v, want at least the %v backoff ceiling", d, boardsRateHoldMax)
	}
}

// An ack answers a specific event by id: the id is what ties a refused write back
// to the payload it carried.
func TestParseBoardsAckCarriesTheEventID(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		known bool
		id    int64
		isErr bool
		code  int
		sec   string
	}{
		{"rate limited", `4317[{"error":{"code":200,"securityCode":"RATE_LIMITED"}}]`, true, 17, true, 200, "RATE_LIMITED"},
		{"id zero", `430[{"result":true}]`, true, 0, false, 0, ""},
		{"plain ack", `433[{"result":true}]`, true, 3, false, 0, ""},
		{"bare error", `435[{"error":{"code":0}}]`, true, 5, true, 0, ""},
		{"no id", `43[{"result":true}]`, false, 0, false, 0, ""},
		{"just the prefix", `43`, false, 0, false, 0, ""},
		{"not an ack", `42["dashboard",{"action":"x"}]`, false, 0, false, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ack := parseBoardsAck([]byte(c.raw))
			if ack.Known != c.known {
				t.Fatalf("Known = %v, want %v", ack.Known, c.known)
			}
			if ack.ID != c.id {
				t.Fatalf("ID = %d, want %d", ack.ID, c.id)
			}
			if ack.Error != c.isErr || ack.Code != c.code || ack.Security != c.sec {
				t.Fatalf("ack = %+v, want error=%v code=%d security=%q", ack, c.isErr, c.code, c.sec)
			}
		})
	}
}

func newTestBoardsSession() *boardsSession {
	participant := "participant-hash"
	creator := "creator-hash"
	sess := &boardsSession{
		Queue:   make(chan []byte, 8),
		pending: make(map[int64]boardsOutgoing),
		retryCh: make(chan struct{}, 1),
		dropCh:  make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}
	sess.participant.Store(&participant)
	sess.creatorHash.Store(&creator)
	return sess
}

// The whole point of the change: a RATE_LIMITED ack returns the payload to the
// writer. Before this the payload was already dropped and `dropped 1 objects`
// was the only trace.
func TestBoardsRateLimitedAckRetriesInsteadOfDroppingThePayload(t *testing.T) {
	tr := &BoardsTransport{}
	sess := newTestBoardsSession()
	payload := []byte("user traffic that must not be lost")
	sess.trackPending(3, boardsOutgoing{pkt: payload, at: time.Now()})

	tr.handleAck(sess, []byte(`433[{"error":{"code":200,"securityCode":"RATE_LIMITED"}}]`))

	if got := tr.statRefused.Load(); got != 1 {
		t.Fatalf("refused = %d, want 1", got)
	}
	if got := tr.statRetried.Load(); got != 1 {
		t.Fatalf("retried = %d, want 1", got)
	}
	if got := tr.statSent.Load(); got != 0 {
		t.Fatalf("sent = %d, want 0: a refusal is not a delivery", got)
	}
	if got := tr.statDroppedDead.Load(); got != 0 {
		t.Fatalf("dropped_dead = %d, want 0: a rate limit is retryable", got)
	}
	if got := sess.pendingCount(); got != 0 {
		t.Fatalf("pending = %d, want the refused write taken out of the in-flight set", got)
	}
	retry, ok := sess.takeRetry()
	if !ok {
		t.Fatal("no retry queued: the payload of a refused write was lost")
	}
	if string(retry.pkt) != string(payload) {
		t.Fatalf("retry carries %q, want the refused payload %q", retry.pkt, payload)
	}
	if retry.tries != 1 {
		t.Fatalf("retry tries = %d, want 1", retry.tries)
	}
	if got := tr.pacer.currentRate(); got >= boardsRateStart {
		t.Fatalf("pace = %v/s, want it cut below the start after a refusal", got)
	}
}

// A plain ack is a delivery: counted, and nothing is requeued.
func TestBoardsPlainAckCountsTheObjectAsSent(t *testing.T) {
	tr := &BoardsTransport{}
	sess := newTestBoardsSession()
	sess.trackPending(4, boardsOutgoing{pkt: []byte("payload"), at: time.Now()})

	tr.handleAck(sess, []byte(`434[{"result":true}]`))

	if got := tr.statSent.Load(); got != 1 {
		t.Fatalf("sent = %d, want 1", got)
	}
	if _, ok := sess.takeRetry(); ok {
		t.Fatal("a delivered object was requeued")
	}
	if got := sess.pendingCount(); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
}

// PAYLOAD_TOO_LARGE is our own frame-size bug and NETWORK_TRANSPORT is a key
// mismatch between the peers: both are permanent for the same bytes, and merging
// them into one counter would make the two indistinguishable in the log.
func TestBoardsPermanentRefusalsStaySeparateAndAreNotRetried(t *testing.T) {
	tr := &BoardsTransport{}
	sess := newTestBoardsSession()
	sess.trackPending(10, boardsOutgoing{pkt: []byte("a"), at: time.Now()})
	tr.handleAck(sess, []byte(`4310[{"error":{"code":200,"securityCode":"PAYLOAD_TOO_LARGE"}}]`))

	sess.trackPending(11, boardsOutgoing{pkt: []byte("b"), at: time.Now()})
	tr.handleAck(sess, []byte(`4311[{"error":{"code":200,"securityCode":"NETWORK_TRANSPORT"}}]`))

	if got := tr.statTooLarge.Load(); got != 1 {
		t.Fatalf("too_large = %d, want 1", got)
	}
	if got := tr.statKeyMismatch.Load(); got != 1 {
		t.Fatalf("key_mismatch = %d, want 1", got)
	}
	if got := tr.statRetried.Load(); got != 0 {
		t.Fatalf("retried = %d, want 0: the same bytes meet the same refusal", got)
	}
	if got := tr.statDroppedDead.Load(); got != 2 {
		t.Fatalf("dropped_dead = %d, want 2", got)
	}
}

// A retry is bounded: an object the board keeps refusing is counted once and
// let go, so it cannot pin the writer forever.
func TestBoardsRetriesAreBounded(t *testing.T) {
	tr := &BoardsTransport{}
	sess := newTestBoardsSession()
	sess.trackPending(12, boardsOutgoing{pkt: []byte("payload"), tries: boardsMaxTries, at: time.Now()})

	tr.handleAck(sess, []byte(`4312[{"error":{"code":200,"securityCode":"RATE_LIMITED"}}]`))

	if got := tr.statDroppedDead.Load(); got != 1 {
		t.Fatalf("dropped_dead = %d, want 1", got)
	}
	if got := tr.statRetried.Load(); got != 0 {
		t.Fatalf("retried = %d, want 0 after the retry cap", got)
	}
	if _, ok := sess.takeRetry(); ok {
		t.Fatal("an object over the retry cap was requeued")
	}
}

// A write nobody acknowledged is retried after the timeout, but a drop is not: it
// costs a full-size event, and an object left on the board is clutter rather than
// lost payload.
func TestBoardsUnacknowledgedDropIsNotResent(t *testing.T) {
	tr := &BoardsTransport{}
	sess := newTestBoardsSession()
	old := time.Now().Add(-2 * boardsAckTimeout)
	event := buildDropObjects([]map[string]interface{}{boardDropObject("obj", 32)}, "participant-hash")
	sess.trackPending(20, boardsOutgoing{drop: event, at: old})
	sess.trackPending(21, boardsOutgoing{pkt: []byte("payload"), at: old})

	tr.requeueExpired(sess)

	if retry, ok := sess.takeRetry(); !ok {
		t.Fatal("a payload with no ack was not retried")
	} else if string(retry.pkt) != "payload" {
		t.Fatalf("retried %q, want the payload write", retry.pkt)
	}
	if _, ok := sess.takeRetry(); ok {
		t.Fatal("an unacknowledged drop was resent")
	}
	if got := sess.pendingCount(); got != 0 {
		t.Fatalf("pending = %d, want both expired writes forgotten", got)
	}
}

// The in-flight set is bounded by bytes as well as by count: this code also runs
// inside the phone, where 200 unacknowledged 48 KiB writes would be an OOM.
func TestBoardsPendingSetIsBoundedAndCountsEvictions(t *testing.T) {
	sess := newTestBoardsSession()
	big := boardsOutgoing{pkt: make([]byte, 48000), at: time.Now()}
	evicted := 0
	for i := 0; i < boardsMaxPendingBytes/48000+5; i++ {
		evicted += len(sess.trackPending(int64(i), big))
	}
	if evicted == 0 {
		t.Fatal("no write was evicted: the byte cap does not bind")
	}
	if got := sess.pendingByteCount(); got > boardsMaxPendingBytes {
		t.Fatalf("pending bytes = %d, want at most %d", got, boardsMaxPendingBytes)
	}
	if got := sess.pendingCount(); got > boardsMaxPending {
		t.Fatalf("pending count = %d, want at most %d", got, boardsMaxPending)
	}
}

// Many received objects become one drop-objects event per window: at one event
// per received object a busy tunnel spent half its budget on deletions.
func TestBoardsDeletionsAreCoalesced(t *testing.T) {
	sess := newTestBoardsSession()
	for i := 0; i < 5; i++ {
		sess.queueDrops([]map[string]interface{}{boardDropObject("obj", 64)})
	}
	if got := sess.dropsPending(); got != 5 {
		t.Fatalf("queued = %d, want 5", got)
	}
	event, ok := sess.takeDrops("participant-hash")
	if !ok {
		t.Fatal("takeDrops() returned nothing")
	}
	objects := dropEventObjects(t, event)
	if len(objects) != 5 {
		t.Fatalf("one drop-objects event carries %d objects, want all 5", len(objects))
	}
	if got := sess.dropsPending(); got != 0 {
		t.Fatalf("still queued after the batch = %d, want 0", got)
	}
	if body, err := json.Marshal(event); err != nil || len(body) > boardsDropEventBytes {
		t.Fatalf("drop event is %d bytes, want at most %d", len(body), boardsDropEventBytes)
	}
	if got := event["action"]; got != "drop-objects" {
		t.Fatalf("action = %v, want drop-objects", got)
	}
}

// A batch never exceeds the measured event budget, and a full-size object goes
// alone instead of taking the event over the board's ceiling.
func TestBoardsDropBatchIsCutByItsMarshalledSize(t *testing.T) {
	sess := newTestBoardsSession()
	full := strings.Repeat("A", 65536) // the longest value the board accepts
	sess.queueDrops([]map[string]interface{}{
		boardDropObjectWithValue("big-1", full),
		boardDropObjectWithValue("big-2", full),
	})

	event, ok := sess.takeDrops("participant-hash")
	if !ok {
		t.Fatal("takeDrops() returned nothing")
	}
	if got := len(dropEventObjects(t, event)); got != 1 {
		t.Fatalf("batch carries %d full-size objects, want 1", got)
	}
	if body, err := json.Marshal(event); err != nil || len(body) > boardsDropEventBytes {
		t.Fatalf("drop event is %d bytes, want at most %d", len(body), boardsDropEventBytes)
	}
	if got := sess.dropsPending(); got != 1 {
		t.Fatalf("remaining = %d, want the second object left for the next window", got)
	}
}

func boardDropObject(id string, valueChars int) map[string]interface{} {
	return boardDropObjectWithValue(id, strings.Repeat("A", valueChars))
}

func boardDropObjectWithValue(id, value string) map[string]interface{} {
	return map[string]interface{}{
		"_attributes_": map[string]interface{}{"id": id, "value": value},
		"hash":         id,
	}
}

func dropEventObjects(t *testing.T, event map[string]interface{}) []map[string]interface{} {
	t.Helper()
	data, ok := event["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("drop event has no data: %v", event)
	}
	objects, ok := data["objects"].([]map[string]interface{})
	if !ok {
		t.Fatalf("drop event has no objects: %v", data)
	}
	return objects
}

// The board decodes an object's value and refuses one that carries visible
// IPv4/TCP traffic: measured 2026-11-08, a real SYN inside the batch frame got
// {"error":{"code":200,"securityCode":"NETWORK_TRANSPORT"}} while the same
// frame with a zero-filled packet was accepted. That is the refusal a peer
// without -encryption-key hits, so it has to reach the log by name.
func TestAckSecurityCodeIsReadFromTheBoardAck(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"network transport", `433[{"error":{"code":200,"securityCode":"NETWORK_TRANSPORT"}}]`, "NETWORK_TRANSPORT"},
		{"payload too large", `437[{"error":{"code":200,"securityCode":"PAYLOAD_TOO_LARGE"}}]`, "PAYLOAD_TOO_LARGE"},
		{"plain ack", `433[{"result":true}]`, ""},
		{"plain error", `435[{"error":{"code":0}}]`, ""},
		{"not json", `43`, ""},
		{"empty", ``, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ackSecurityCode([]byte(c.raw)); got != c.want {
				t.Fatalf("ackSecurityCode(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}
