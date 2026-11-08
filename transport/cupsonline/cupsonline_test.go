package cupsonline

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A room the relay closed is swapped for a fresh one, and the list the phone
// must follow changes with it: RoomList and the observer both see the swap.
func TestSwapRoomPublishesNewList(t *testing.T) {
	t.Parallel()
	tr := &CupsonlineTransport{}
	tr.roomIDs = []string{"aaaaaaaa-1", "bbbbbbbb-2"}
	var published []string
	tr.OnRoomList(func(packed string) {
		ids, err := unpackRooms(packed)
		if err != nil {
			t.Fatalf("unpackRooms(%q): %v", packed, err)
		}
		published = ids
	})
	tr.setRoomList(tr.roomIDs)

	tr.swapRoomID("aaaaaaaa-1", "cccccccc-3")

	want := []string{"cccccccc-3", "bbbbbbbb-2"}
	if got := strings.Join(tr.roomIDs, ","); got != strings.Join(want, ",") {
		t.Fatalf("roomIDs = %s, want %s", got, strings.Join(want, ","))
	}
	if got := strings.Join(published, ","); got != strings.Join(want, ",") {
		t.Fatalf("published = %s, want %s", got, strings.Join(want, ","))
	}
	if got := tr.RoomList(); got != packRooms(want) {
		t.Fatalf("RoomList() = %q, want %q", got, packRooms(want))
	}
}

// Only the exit creates rooms: a client's closed room cannot be replaced, and
// that refusal is not an error to log.
func TestReplaceDeadRoomIsExitOnly(t *testing.T) {
	t.Parallel()
	tr := &CupsonlineTransport{isClient: true}
	if _, err := tr.replaceDeadRoom("room"); !errors.Is(err, errNoReplace) {
		t.Fatalf("client replace err = %v, want errNoReplace", err)
	}
}

// Swaps are paced: one per RoomReplaceMinInterval, so a burst of closed rooms
// does not look like room-creation abuse to the relay.
func TestReplaceDeadRoomIsPaced(t *testing.T) {
	t.Parallel()
	tr := &CupsonlineTransport{config: DefaultCupsonlineConfig()}
	tr.lastReplace = time.Now()
	if _, err := tr.replaceDeadRoom("room"); !errors.Is(err, errNoReplace) {
		t.Fatalf("paced replace err = %v, want errNoReplace", err)
	}
}

// A closed room with the whole pool down retries within minutes, not in half
// an hour: the relay's refusals are usually a passing rate limit.
func TestRetryDelayCapsWhenWholePoolIsDown(t *testing.T) {
	t.Parallel()
	w := &cupsWS{config: DefaultCupsonlineConfig()}
	w.config.RoomGoneRetryMin = time.Minute
	w.config.RoomGoneRetryMax = 30 * time.Minute
	w.dead.Store(true)
	w.goneDelay = 8 * time.Minute
	w.pool = func() (int, int) { return 0, 8 }
	if d := w.retryDelay(0); d > allPoolDeadRetryMax {
		t.Fatalf("dead pool retry = %v, want <= %v", d, allPoolDeadRetryMax)
	}

	w.pool = func() (int, int) { return 7, 8 }
	w.goneDelay = 8 * time.Minute
	if d := w.retryDelay(0); d != 8*time.Minute {
		t.Fatalf("one live room retry = %v, want 8m", d)
	}
}

// Up is not alive: a peer that stopped answering inside live rooms has to show
// as disconnected, or the tunnel sits there carrying nothing forever.
func TestSilentPeer(t *testing.T) {
	t.Parallel()
	tr := &CupsonlineTransport{}
	if tr.silentPeer() {
		t.Fatal("silentPeer with no traffic sent")
	}

	tr.lastSend.Store(time.Now().Add(-2 * peerSilentAfter).UnixNano())
	if !tr.silentPeer() {
		t.Fatal("silentPeer did not notice a minute of unanswered sending")
	}

	tr.lastRecv.Store(time.Now().UnixNano())
	if tr.silentPeer() {
		t.Fatal("silentPeer said silent while the peer is answering")
	}
}

// liveRooms is what the retry cap and the operator line count: a room marked
// closed is not part of the pool anymore.
func TestLiveRooms(t *testing.T) {
	t.Parallel()
	dead, live := &cupsWS{}, &cupsWS{}
	dead.dead.Store(true)
	tr := &CupsonlineTransport{wss: []*cupsWS{dead, live, live}}
	alive, total := tr.liveRooms()
	if alive != 2 || total != 3 {
		t.Fatalf("liveRooms() = %d/%d, want 2/3", alive, total)
	}
}
