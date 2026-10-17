package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	godebug "runtime/debug"
	"strconv"
	"strings"

	_ "github.com/wlynxg/anet"
	"universal-bypass-tool/socks5"
	"universal-bypass-tool/transport"
	"universal-bypass-tool/transport/avito"
	"universal-bypass-tool/transport/bitrix24"
	"universal-bypass-tool/transport/cupsonline"
	"universal-bypass-tool/transport/mailru"
	"universal-bypass-tool/transport/oneme"
	"universal-bypass-tool/transport/yandex"
	"universal-bypass-tool/tunnel"
	"universal-bypass-tool/utils"
)

var (
	globalDocUrl string
	maxToken     string
	maxUid       string
)

func main() {
	//os.Setenv("GODEBUG", "netdns=go")
	fmt.Print("written by p1neappleXpress\n")

	exitNode := flag.Bool("exit-node", false, "Run as exit node (needs root)")
	client := flag.Bool("client", false, "Run as client")
	debug := flag.Bool("debug", false, "Enable verbose debug logging")
	socksAddr := flag.String("socks5", ":1080", "SOCKS5 address")
	transportType := flag.String("transport", "yandex", "Transport type (yandex, vyandex, oneme, mailru)")
	codec := flag.String("codec", "batched", "Codec: batched (default, zstd+coalescing) or legacy (per-packet LZ4)")
	flag.StringVar(&globalDocUrl, "url", "", "Document URL. Required for Yandex/Mailru Docs transport")
	urlFile := flag.String("url-file", "", "Read the document URL from a file")
	encryptionKey := flag.String("encryption-key", "", "Optional: AES-256-GCM transport encryption key")
	encryptionKeyFile := flag.String("encryption-key-file", "", "Optional: read AES-256-GCM transport encryption key from file")
	flag.StringVar(&maxToken, "maxToken", "", "MAX Web token. If u use MAX transport")
	flag.StringVar(&maxUid, "maxUid", "", "MAX call user id. If u use MAX transport")
	localIP := flag.String("local-ip", "", "Exit node egress IP (use a dedicated alias IP so the RST-drop rule can be scoped with -s)")
	mode := flag.String("mode", "l4", "Exit-node mode: l4 (default, user-space forwarder) or l3 (kernel raw socket)")
	warpProxy := flag.String("warp-proxy", "", "WARP/SOCKS5 proxy for client exit traffic in L4 mode (e.g. 127.0.0.1:40000 or user:pass@host:port)")
	socksProxy := flag.String("socks-proxy", "", "SOCKS5/HTTP proxy for transport signaling (e.g. 127.0.0.1:40000 or user:pass@host:port)")
	cookiesFile := flag.String("cookies", "", "Optional: Netscape cookies.txt file (for Yandex Volga or Mail.ru)")
	cookiesFileAlias := flag.String("cookies-file", "", "Optional: alias for --cookies")
	flag.Parse()

	if *mode == "" {
		*mode = "l4"
	}

	exitMode, err := tunnel.ParseExitMode(*mode)
	if err != nil {
		log.Fatalf("--mode: %v", err)
	}

	if *localIP != "" {
		tunnel.SetLocalIP(*localIP)
	}

	warpVal := strings.TrimSpace(*warpProxy)
	if warpVal == "" {
		if env := os.Getenv("OLCRTC_WARP_PROXY"); env != "" {
			warpVal = strings.TrimSpace(env)
		} else if env := os.Getenv("WARP_PROXY"); env != "" {
			warpVal = strings.TrimSpace(env)
		}
	}
	if warpVal != "" {
		if err := tunnel.SetWarpProxy(warpVal); err != nil {
			log.Fatalf("Configure warp-proxy: %v", err)
		}
	}

	socksVal := strings.TrimSpace(*socksProxy)
	if socksVal == "" {
		if env := os.Getenv("OLCRTC_SOCKS_PROXY"); env != "" {
			socksVal = strings.TrimSpace(env)
		} else if env := os.Getenv("SOCKS_PROXY"); env != "" {
			socksVal = strings.TrimSpace(env)
		}
	}
	if socksVal != "" {
		if err := utils.SetSignalingProxy(socksVal); err != nil {
			log.Fatalf("Configure socks-proxy: %v", err)
		}
		pURL := socksVal
		if !strings.Contains(pURL, "://") {
			pURL = "socks5://" + pURL
		}
		os.Setenv("ALL_PROXY", pURL)
		os.Setenv("HTTP_PROXY", pURL)
		os.Setenv("HTTPS_PROXY", pURL)
		os.Setenv("all_proxy", pURL)
		os.Setenv("http_proxy", pURL)
		os.Setenv("https_proxy", pURL)
	}

	if codecEnv := strings.TrimSpace(os.Getenv("OLCRTC_OPENFLUX_CODEC")); codecEnv != "" {
		*codec = codecEnv
	} else if codecEnv := strings.TrimSpace(os.Getenv("OLCRTC_CODEC")); codecEnv != "" {
		*codec = codecEnv
	} else if codecEnv := strings.TrimSpace(os.Getenv("OPENFLUX_CODEC")); codecEnv != "" {
		*codec = codecEnv
	}

	if *exitNode {
		// Aggressive GC to keep heap tight under load on small VPS
		godebug.SetGCPercent(20)
	}

	if !*exitNode && !*client {
		flag.Usage()
		os.Exit(1)
	}

	if *debug {
		utils.EnableDebug()
	}

	var secret string
	if *encryptionKey != "" && *encryptionKeyFile != "" {
		log.Fatalf("use only one of --encryption-key or --encryption-key-file")
	}
	if *encryptionKey != "" {
		secret = strings.TrimSpace(*encryptionKey)
	} else if *encryptionKeyFile != "" {
		data, err := os.ReadFile(*encryptionKeyFile)
		if err != nil {
			log.Fatalf("Read encryption key file: %v", err)
		}
		secret = strings.TrimSpace(string(data))
	} else if envKey := os.Getenv("OPENFLUX_KEY"); envKey != "" {
		secret = strings.TrimSpace(envKey)
	}
	if envCodec := os.Getenv("OPENFLUX_CODEC"); envCodec != "" && *codec == "legacy" {
		*codec = envCodec
	}

	log.Printf("=== Universal Bypass Tool ===")
	if *exitNode {
		log.Printf("Mode: EXIT NODE (%s)", exitMode.String())
	} else {
		log.Printf("Mode: CLIENT")
	}
	log.Printf("Transport: %s (codec: %s)", *transportType, *codec)

	config := transport.DefaultConfig()
	var inner transport.Transport

	switch *transportType {
	case "vyandex", "volga":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "document URL")
		if err != nil {
			log.Fatal(err)
		}
		inner = yandex.NewYandexVolgaTransport(globalDocUrl, config)
	case "yandex":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "document URL")
		if err != nil {
			log.Fatal(err)
		}
		if strings.Contains(globalDocUrl, "mail.ru") {
			log.Printf("[INFO] Detected Mail.ru document URL, switching to mailru transport")
			inner = mailru.NewMailruDocsTransport(globalDocUrl, config)
		} else if strings.Contains(globalDocUrl, "avito.ru") {
			log.Printf("[INFO] Detected Avito URL, switching to avito transport")
			inner = avito.NewAvitoTransport(globalDocUrl, config, "", !*exitNode)
		} else if strings.Contains(globalDocUrl, "cups.online") {
			log.Printf("[INFO] Detected Cups.online URL, switching to cupsonline transport")
			inner = cupsonline.NewCupsonlineTransport(globalDocUrl, config, !*exitNode)
		} else if strings.Contains(globalDocUrl, "telemost.yandex.ru") {
			log.Printf("[INFO] Detected Telemost conference URL, switching to telemost transport")
			inner = yandex.NewTelemostTransport("", globalDocUrl, *exitNode, config)
		} else if strings.Contains(globalDocUrl, "bitrix24") {
			log.Printf("[INFO] Detected Bitrix24 URL, switching to bitrix24 transport")
			inner = bitrix24.NewBitrix24Transport(globalDocUrl, config, os.Getenv("B24_COOKIES"), os.Getenv("B24_SESSID"), os.Getenv("B24_CHAT_ID"))
		} else if isVolgaDoc(globalDocUrl) {
			log.Printf("[INFO] Detected Volga editor document, switching to vyandex transport")
			inner = yandex.NewYandexVolgaTransport(globalDocUrl, config)
		} else {
			inner = yandex.NewYandexDocsTransport(globalDocUrl, config)
		}
	case "mailru":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "document URL")
		if err != nil {
			log.Fatal(err)
		}
		inner = mailru.NewMailruDocsTransport(globalDocUrl, config)
	case "avito":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "room/code URL")
		if err != nil {
			log.Fatal(err)
		}
		inner = avito.NewAvitoTransport(globalDocUrl, config, "", !*exitNode)
	case "cupsonline", "cups":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "room/interview URL")
		if err != nil {
			log.Fatal(err)
		}
		inner = cupsonline.NewCupsonlineTransport(globalDocUrl, config, !*exitNode)
	case "telemost":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "Telemost room URL")
		if err != nil {
			log.Fatal(err)
		}
		inner = yandex.NewTelemostTransport("", globalDocUrl, *exitNode, config)
	case "bitrix24", "b24":
		var err error
		globalDocUrl, err = readRequiredOption(globalDocUrl, *urlFile, "Bitrix24 portal URL")
		if err != nil {
			log.Fatal(err)
		}
		inner = bitrix24.NewBitrix24Transport(globalDocUrl, config, os.Getenv("B24_COOKIES"), os.Getenv("B24_SESSID"), os.Getenv("B24_CHAT_ID"))
	case "oneme":
		uidint, _ := strconv.ParseInt(maxUid, 10, 64)
		inner = oneme.NewOneMeTransport(*exitNode, maxToken, uidint, config)
	default:
		log.Fatalf("Unknown transport type: %s", *transportType)
	}

	cookiePath := strings.TrimSpace(*cookiesFile)
	if cookiePath == "" {
		cookiePath = strings.TrimSpace(*cookiesFileAlias)
	}
	if cookiePath == "" {
		if env := os.Getenv("OLCRTC_COOKIES_FILE"); env != "" {
			cookiePath = strings.TrimSpace(env)
		} else if env := os.Getenv("OLCRTC_COOKIES"); env != "" {
			cookiePath = strings.TrimSpace(env)
		} else if env := os.Getenv("OPENFLUX_COOKIES"); env != "" {
			cookiePath = strings.TrimSpace(env)
		}
	}
	if cookiePath != "" {
		type cookieLoader interface {
			LoadCookieFile(string) error
		}
		if cl, ok := inner.(cookieLoader); ok {
			log.Printf("[INFO] Loading cookies from %s...", cookiePath)
			if err := cl.LoadCookieFile(cookiePath); err != nil {
				log.Printf("[WARN] Cookie file %s: %v (continuing without cookies)", cookiePath, err)
			} else {
				log.Printf("[INFO] Cookies loaded successfully from %s", cookiePath)
			}
		} else {
			log.Printf("[WARN] Transport %s does not support loading cookie files", *transportType)
		}
	}

	if secret != "" {
		context := *transportType
		if globalDocUrl != "" {
			context = globalDocUrl
		}
		encrypted, err := transport.NewEncryptedTransport(inner, secret, context, *exitNode)
		if err != nil {
			log.Fatalf("Configure encrypted transport: %v", err)
		}
		inner = encrypted
		log.Printf("Transport encryption: AES-256-GCM enabled")
	}

	var trans transport.Transport
	switch *codec {
	case "legacy":
		log.Printf("Codec: legacy (per-packet LZ4, no batching)")
		trans = transport.NewCompressedTransport(inner)
	case "batched":
		log.Printf("Codec: batched (zstd + coalescing)")
		trans = transport.NewBatchedTransport(inner)
	default:
		log.Fatalf("Unknown codec: %s (want batched|legacy)", *codec)
	}

	if err := trans.Start(); err != nil {
		log.Fatalf("Failed to start transport: %v", err)
	}

	if *exitNode {
		runExit(trans, exitMode, *localIP)
	} else {
		runClient(trans, *socksAddr)
	}
}

func runExit(trans transport.Transport, exitMode tunnel.ExitMode, localIP string) {
	ex, err := tunnel.NewExitNode(trans, exitMode.String())
	if err != nil {
		log.Fatalf("exit node: %v", err)
	}
	log.Printf("Running as EXIT NODE (mode=%s)", ex.Mode())
	if err := ex.Start(); err != nil {
		log.Fatalf("exit start: %v", err)
	}

	if exitMode == tunnel.ExitModeL3 {
		if localIP != "" {
			log.Printf("! Run: sudo iptables -A OUTPUT -s %s -p tcp --tcp-flags RST RST -j DROP", localIP)
		} else {
			log.Printf("! Run: sudo iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP")
		}
	}
	select {}
}

func runClient(trans transport.Transport, socksAddr string) {
	log.Printf("Running as CLIENT (SOCKS5 on %s)", socksAddr)
	tun := tunnel.NewTCPTunnel(trans, false)
	socks5Server := socks5.NewSOCKS5Server(socksAddr, tun)
	log.Fatal(socks5Server.Start())
}

func readRequiredOption(value, filename, label string) (string, error) {
	if value != "" && filename != "" {
		return "", fmt.Errorf("use only one of the inline or file options for %s", label)
	}
	if filename != "" {
		data, err := os.ReadFile(filename)
		if err != nil {
			return "", fmt.Errorf("read %s file: %w", label, err)
		}
		value = string(data)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

func isVolgaDoc(docURL string) bool {
	// Volga editor is strictly used for /edit/d/ documents
	return strings.Contains(docURL, "/edit/d/")
}
