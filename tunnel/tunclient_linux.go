//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"universal-bypass-tool/transport"
	"universal-bypass-tool/utils"
)

// Linux TUN client mode.
//
// The Android VPN client hands raw IP packets to the transport and takes raw IP
// packets back, which is exactly what an l3 exit node expects on the other side.
// The SOCKS5 client mode above it speaks an l4 TCP tunnel instead, so until this
// existed the phone's framing could only be tested with a phone. With `-tun` the
// same path runs on a Linux box: the kernel stack does TCP, the packets travel
// through the carrier, and the exit node forwards them.

const (
	ifNameSize = 16
	tunSetIFF  = 0x400454ca
	iffTun     = 0x0001
	iffNoPI    = 0x1000
)

type ifReq struct {
	name  [ifNameSize]byte
	flags uint16
	_     [22]byte
}

// RunTUNClient relays raw IP packets between the named TUN device and the
// transport until the transport stops. The device must already exist and have an
// address; the function only opens it and clears the packet-information header
// so the payload is a bare IP packet.
func RunTUNClient(trans transport.Transport, name string) error {
	if name == "" {
		return fmt.Errorf("tun client: empty device name")
	}
	fd, err := syscall.Open("/dev/net/tun", syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("tun client: open /dev/net/tun: %w", err)
	}
	var req ifReq
	copy(req.name[:], name)
	req.flags = iffTun | iffNoPI
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tunSetIFF, uintptr(unsafe.Pointer(&req))); errno != 0 {
		syscall.Close(fd)
		return fmt.Errorf("tun client: attach %s: %w", name, errno)
	}
	file := os.NewFile(uintptr(fd), "/dev/net/tun")
	defer file.Close()

	utils.Infof("[TUN-CLIENT] attached %s, relaying raw IP packets through the carrier", name)

	var once sync.Once
	stop := func() {
		once.Do(func() { _ = file.Close() })
	}
	defer stop()

	// Uplink: whatever the kernel routes into the device goes to the carrier.
	go func() {
		buf := make([]byte, 65535)
		for {
			n, err := file.Read(buf)
			if err != nil || n <= 0 {
				utils.Debugf("[TUN-CLIENT] device read stopped: %v", err)
				stop()
				return
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			if err := trans.Send(pkt); err != nil {
				utils.Debugf("[TUN-CLIENT] send failed: %v", err)
			}
		}
	}()

	// Downlink: whatever the carrier delivers is written back into the device.
	trans.Receive(func(pkt []byte) {
		if len(pkt) == 0 {
			return
		}
		if _, err := file.Write(pkt); err != nil {
			utils.Debugf("[TUN-CLIENT] device write failed: %v", err)
		}
	})

	// Stay up while the carrier is up: a client that returned here would drop the
	// device and take the tunnel down at the first reconnect.
	waitConnected(trans, 30*time.Second)
	for trans.IsConnected() {
		time.Sleep(500 * time.Millisecond)
	}
	utils.Infof("[TUN-CLIENT] carrier disconnected, leaving")
	return nil
}

// waitConnected waits until the transport reports a live carrier, or the timeout
// expires, so a brief dial delay does not look like a disconnect.
func waitConnected(trans transport.Transport, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if trans.IsConnected() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
