// Package mobile exposes the OpenFlux packet transport to Android through
// gomobile. Android owns the TUN file descriptor; this package only transports
// complete IPv4 packets through the configured Yandex document.
package mobile

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"universal-bypass-tool/transport"
	"universal-bypass-tool/transport/mailru"
	"universal-bypass-tool/transport/yandex"
	"universal-bypass-tool/utils"
)

var (
	client        = packetClient{}
	encKeyMu      sync.Mutex
	encryptionKey string
)

// SetEncryptionKey configures an optional AES-256-GCM transport key.
func SetEncryptionKey(key string) {
	encKeyMu.Lock()
	defer encKeyMu.Unlock()
	encryptionKey = strings.TrimSpace(key)
}

type packetClient struct {
	mu        sync.Mutex
	running   bool
	transport transport.Transport
	packets   [][]byte
	logs      []string
}

func appendLog(message string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.logs = append(client.logs, message)
	if len(client.logs) > 500 {
		client.logs = append([]string(nil), client.logs[len(client.logs)-500:]...)
	}
}

func detectTransport(docURL string) string {
	if strings.Contains(docURL, "mail.ru") {
		return "mailru"
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", docURL, nil)
	if err != nil {
		return "yandex"
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "yandex"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "yandex"
	}

	content := string(body)
	if strings.Contains(content, `"officeType":"volga"`) || (strings.Contains(content, "volga") && !strings.Contains(content, "balancer_url")) {
		return "vyandex"
	}
	return "yandex"
}

// Start connects the packet transport. It returns an empty string on success
// and a user-readable error on failure.
func Start(documentURL string) string {
	if documentURL == "" {
		return "Ссылка на документ не указана"
	}

	client.mu.Lock()
	if client.running {
		client.mu.Unlock()
		return ""
	}
	client.running = true
	client.packets = nil
	client.logs = nil
	client.mu.Unlock()

	utils.EnableDebug()
	utils.SetLogSink(appendLog)

	config := transport.DefaultConfig()
	detected := detectTransport(documentURL)

	var innerTrans transport.Transport
	switch detected {
	case "mailru":
		appendLog("[ANDROID] Запуск транспорта Mail.ru Docs")
		innerTrans = mailru.NewMailruDocsTransport(documentURL, config)
	case "vyandex":
		appendLog("[ANDROID] Обнаружен редактор Volga. Запуск транспорта vyandex")
		innerTrans = yandex.NewYandexVolgaTransport(documentURL, config)
	default:
		appendLog("[ANDROID] Запуск классического транспорта yandex")
		innerTrans = yandex.NewYandexDocsTransport(documentURL, config)
	}

	encKeyMu.Lock()
	key := encryptionKey
	encKeyMu.Unlock()
	if envKey := os.Getenv("OPENFLUX_ENCRYPTION_KEY"); envKey != "" {
		key = strings.TrimSpace(envKey)
	} else if envKey := os.Getenv("OPENFLUX_KEY"); envKey != "" {
		key = strings.TrimSpace(envKey)
	}
	if key != "" {
		appendLog("[ANDROID] Включение сквозного шифрования AES-256-GCM")
		encTrans, err := transport.NewEncryptedTransport(innerTrans, key, documentURL, false)
		if err != nil {
			appendLog(fmt.Sprintf("[ANDROID] Ошибка настройки шифрования: %v", err))
			client.mu.Lock()
			client.running = false
			client.mu.Unlock()
			return err.Error()
		}
		innerTrans = encTrans
	}

	var trans transport.Transport
	if os.Getenv("OPENFLUX_CODEC") == "legacy" {
		appendLog("[ANDROID] Использование legacy LZ4 кодека")
		trans = transport.NewCompressedTransport(innerTrans)
	} else {
		appendLog("[ANDROID] Использование batched+zstd кодека")
		trans = transport.NewBatchedTransport(innerTrans)
	}
	trans.Receive(func(data []byte) {
		packet := append([]byte(nil), data...)
		client.mu.Lock()
		if !client.running {
			client.mu.Unlock()
			return
		}
		if len(client.packets) >= config.MaxQueueSize {
			client.packets = client.packets[1:]
		}
		client.packets = append(client.packets, packet)
		client.mu.Unlock()
	})

	if err := trans.Start(); err != nil {
		appendLog(fmt.Sprintf("[ANDROID] Ошибка запуска: %v", err))
		client.mu.Lock()
		client.running = false
		client.mu.Unlock()
		return err.Error()
	}

	client.mu.Lock()
	client.transport = trans
	client.mu.Unlock()
	return ""
}

func Stop() {
	client.mu.Lock()
	trans := client.transport
	client.running = false
	client.transport = nil
	client.packets = nil
	client.mu.Unlock()
	appendLog("[ANDROID] Остановка транспорта")
	if trans != nil {
		_ = trans.Stop()
	}
}

func Ping() string {
	return ""
}

func PingMillis() int64 {
	return -1
}

func PingSequence() int64 {
	return 0
}

func IsConnected() bool {
	client.mu.Lock()
	trans := client.transport
	client.mu.Unlock()
	return trans != nil && trans.IsConnected()
}

func Send(packet []byte) string {
	client.mu.Lock()
	trans := client.transport
	running := client.running
	client.mu.Unlock()
	if !running || trans == nil {
		return "Транспорт не запущен"
	}
	if err := trans.Send(packet); err != nil {
		return err.Error()
	}
	return ""
}

// Read returns one received packet, or nil when the queue is empty.
func Read() []byte {
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.packets) == 0 {
		return nil
	}
	packet := client.packets[0]
	client.packets = client.packets[1:]
	return packet
}

// ReadLogs returns and clears the pending log lines.
func ReadLogs() string {
	client.mu.Lock()
	defer client.mu.Unlock()
	logs := strings.Join(client.logs, "\n")
	client.logs = nil
	return logs
}
