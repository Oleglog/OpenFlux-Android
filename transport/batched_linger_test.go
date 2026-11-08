package transport

import (
	"sync"
	"testing"
	"time"
)

// carrierHintStub publishes batch defaults of its own, which the coalescing layer
// uses unless an env var overrides them. The embedded Transport is never called:
// these tests only construct the layer, they do not run it.
type carrierHintStub struct {
	Transport
	bytes  int
	linger int
}

func (s carrierHintStub) PreferredBatchBytes() int    { return s.bytes }
func (s carrierHintStub) PreferredBatchLingerMs() int { return s.linger }

// The env var has to beat the carrier hint: the hint is the carrier's default
// operating point, the env var is what an experiment on a live stand sets without
// a rebuild. v1.0.53 shipped the opposite reading of the code and the stand could
// not move the deadline at all.
func TestBatchLingerEnvBeatsTheCarrierHint(t *testing.T) {
	t.Setenv("OPENFLUX_BATCH_LINGER_MS", "250")
	t.Setenv("OPENFLUX_BATCH_BYTES", "12345")

	inner := carrierHintStub{bytes: 48000, linger: 300}
	if got := batchLingerMs(inner); got != 250 {
		t.Fatalf("batchLingerMs() = %d, want the env value 250 over the carrier hint 300", got)
	}
	if got := batchBytes(inner); got != 12345 {
		t.Fatalf("batchBytes() = %d, want the env value 12345 over the carrier hint 48000", got)
	}

	b := NewBatchedTransport(inner)
	if b.lingerMs != 250 || b.maxBatchBytes != 12345 {
		t.Fatalf("NewBatchedTransport: linger=%d bytes=%d, want 250/12345", b.lingerMs, b.maxBatchBytes)
	}
}

// With no env var the carrier's hint is the operating point.
func TestBatchHintsApplyWhenTheEnvIsUnset(t *testing.T) {
	t.Setenv("OPENFLUX_BATCH_LINGER_MS", "")
	t.Setenv("OPENFLUX_BATCH_BYTES", "")

	b := NewBatchedTransport(carrierHintStub{bytes: 48000, linger: 300})
	if b.lingerMs != 300 {
		t.Fatalf("lingerMs = %d, want the carrier hint 300", b.lingerMs)
	}
	if b.maxBatchBytes != 48000 {
		t.Fatalf("maxBatchBytes = %d, want the carrier hint 48000", b.maxBatchBytes)
	}
}

// With neither env nor hint the global defaults stand.
func TestBatchDefaultsWithoutEnvOrHint(t *testing.T) {
	t.Setenv("OPENFLUX_BATCH_LINGER_MS", "")
	t.Setenv("OPENFLUX_BATCH_BYTES", "")

	b := NewBatchedTransport(nil)
	if b.lingerMs != defaultLingerMs || b.maxBatchBytes != defaultMaxBatchBytes {
		t.Fatalf("linger=%d bytes=%d, want the defaults %d/%d",
			b.lingerMs, b.maxBatchBytes, defaultLingerMs, defaultMaxBatchBytes)
	}
}

// A batch of small packets gets the quick deadline, bulk data waits the carrier's
// window: holding an ACK for the bulk window pays that window on every round trip
// through the tunnel.
func TestBatchDeadlineFollowsThePacketSize(t *testing.T) {
	small := make([]byte, 64)
	bulk := make([]byte, 1400)

	b := NewBatchedTransport(carrierHintStub{bytes: 48000, linger: 300})
	if got := b.lingerFor([][]byte{small, small}); got != quickLingerMs*time.Millisecond {
		t.Fatalf("small batch deadline = %v, want the quick %v", got, time.Duration(quickLingerMs)*time.Millisecond)
	}
	if got := b.lingerFor([][]byte{small, bulk}); got != 300*time.Millisecond {
		t.Fatalf("bulk batch deadline = %v, want the carrier window 300 ms", got)
	}

	// A carrier that asks for less than the quick deadline keeps its own number.
	short := NewBatchedTransport(carrierHintStub{bytes: 4096, linger: 20})
	if got := short.lingerFor([][]byte{small}); got != 20*time.Millisecond {
		t.Fatalf("quick deadline over a 20 ms window = %v, want 20 ms", got)
	}
}

// captureTransport records the messages the coalescing layer hands to the wire.
type captureTransport struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (c *captureTransport) Start() error { return nil }
func (c *captureTransport) Stop() error  { return nil }

func (c *captureTransport) Send(data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	c.mu.Lock()
	c.msgs = append(c.msgs, cp)
	c.mu.Unlock()
	return nil
}

func (c *captureTransport) Receive(func([]byte))  {}
func (c *captureTransport) IsConnected() bool     { return true }
func (c *captureTransport) Stats() TransportStats { return TransportStats{} }

func (c *captureTransport) messages() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.msgs))
	copy(out, c.msgs)
	return out
}

// A batch is cut at the byte ceiling, not at the timer: when the tunnel has data
// queued, one event must carry up to 48 KiB of payload. The other regime is what
// cost v1.0.52 its throughput - 1.6 KiB objects, one event each, 20 KB/s.
func TestBatchFillsToTheByteCeilingNotTheTimer(t *testing.T) {
	t.Setenv("OPENFLUX_BATCH_BYTES", "48000")
	t.Setenv("OPENFLUX_BATCH_LINGER_MS", "300")

	carrier := &captureTransport{}
	b := NewBatchedTransport(carrier)
	// Queue the traffic before the flush loop starts, so the first batch sees a
	// full queue instead of racing the sender.
	for i := 0; i < 200; i++ {
		pkt := make([]byte, 1024)
		pkt[0] = byte(i)
		if err := b.Send(pkt); err != nil {
			t.Fatalf("Send() = %v", err)
		}
	}
	if err := b.Start(); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	defer b.Stop()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(carrier.messages()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	msgs := carrier.messages()
	if len(msgs) == 0 {
		t.Fatal("no batch reached the carrier")
	}
	pkts, err := decodeBatch(msgs[0])
	if err != nil {
		t.Fatalf("decodeBatch() = %v", err)
	}
	// 48000 / 1026 = 46 packets fit; anything near the timer would be ~1-2.
	if len(pkts) < 40 {
		t.Fatalf("first batch carries %d packets, want a batch filled to the 48000-byte ceiling", len(pkts))
	}
}
