package transport

import (
	"bytes"
	"testing"
)

// An exit node started without -encryption-key used to swallow a peer's sealed
// frames in the "legacy unbatched" fallback and hand the ciphertext upward: on
// an l3 exit that surfaced as "drop: bad IPv4 slice (85 bytes)" — 33 bytes of
// crypto overhead plus a 52-byte batch — which says nothing about the key. The
// ciphertext is now recognised and dropped, with one line that names the cause.
func TestBatchedTransportDropsEncryptedFrameWithoutKey(t *testing.T) {
	wire := &testTransport{}
	b := NewBatchedTransport(wire)
	var got [][]byte
	b.Receive(func(p []byte) { got = append(got, append([]byte(nil), p...)) })

	// What EncryptedTransport puts on the wire: OFX, version 1, direction, nonce.
	sealed := append([]byte{'O', 'F', 'X', 1, 0}, bytes.Repeat([]byte{7}, 12+16)...)
	sealed = append(sealed, []byte("batch")...)
	wire.deliver(sealed)
	if len(got) != 0 {
		t.Fatalf("delivered %d frame(s) of ciphertext upward: %q", len(got), got[0])
	}

	// A legacy unbatched peer still gets through: a raw IPv4 packet has to
	// arrive intact, otherwise the guard would break the old codec path.
	legacy := []byte{0x45, 0x00, 0x00, 0x1c, 0, 0, 0, 0, 64, 6}
	wire.deliver(legacy)
	if len(got) != 1 || !bytes.Equal(got[0], legacy) {
		t.Fatalf("legacy packet = %v, want it delivered as-is", got)
	}
}

func TestLooksLikeEncryptedFrame(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"sealed", []byte{'O', 'F', 'X', 1, 0, 0}, true},
		{"short", []byte{'O', 'F'}, false},
		{"ipv4", []byte{0x45, 0, 0, 20, 0}, false},
		{"ipv6", []byte{0x60, 0, 0, 0, 0}, false},
		{"batch", []byte{batchFormatVersion, 0, 0, 40}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksLikeEncryptedFrame(c.data); got != c.want {
				t.Fatalf("looksLikeEncryptedFrame(% x) = %v, want %v", c.data, got, c.want)
			}
		})
	}
}

// The mirror case: this side has a key, the peer does not. The frame is a plain
// batch frame, so it is rejected by the header check and never reaches the
// callback — the point of the test is that a plaintext batch frame is not
// mistaken for a sealed one by the new warning path.
func TestEncryptedTransportRejectsPlainBatchFrame(t *testing.T) {
	wire := &testTransport{}
	enc, err := NewEncryptedTransport(wire, "a sufficiently long shared secret", "document", true)
	if err != nil {
		t.Fatal(err)
	}
	var got [][]byte
	enc.Receive(func(p []byte) { got = append(got, append([]byte(nil), p...)) })

	plain := encodeBatch([][]byte{{0x45, 0, 0, 20}})
	wire.deliver(plain)
	if len(got) != 0 {
		t.Fatalf("plaintext batch frame was accepted as ciphertext: %q", got[0])
	}
}
