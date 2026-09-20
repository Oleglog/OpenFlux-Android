package tunnel

import (
	"fmt"
	"runtime"

	"universal-bypass-tool/transport"
	"universal-bypass-tool/tunnel/l3"
	"universal-bypass-tool/utils"
)

type ExitNode interface {
	Start() error
	Stop() error
	Mode() string
}

func NewExitNode(trans transport.Transport, mode string) (ExitNode, error) {
	switch mode {
	case "l3":
		node, err := l3.New(trans)
		if err != nil {
			return nil, fmt.Errorf("l3: %w", err)
		}
		utils.Debugf("[EXIT] using L3 (platform=%s)", runtime.GOOS)
		return node, nil
	case "l4", "proxy", "":
		// "proxy" is an alias kept for compatibility.
		return newProxyExit(trans), nil
	default:
		return nil, fmt.Errorf("unknown exit mode %q (want l3|l4)", mode)
	}
}
