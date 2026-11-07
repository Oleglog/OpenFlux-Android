//go:build !linux

package tunnel

import (
	"fmt"

	"universal-bypass-tool/transport"
)

// RunTUNClient is available on Linux only: it opens /dev/net/tun and speaks raw
// IP to an l3 exit node, the same framing the Android client uses.
func RunTUNClient(trans transport.Transport, name string) error {
	return fmt.Errorf("tun client: only supported on linux, use the SOCKS5 client mode here")
}
