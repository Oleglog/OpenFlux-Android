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
	"universal-bypass-tool/transport/avito"
	"universal-bypass-tool/transport/bitrix24"
	"universal-bypass-tool/transport/cupsonline"
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
	if strings.Contains(docURL, "avito.ru") {
		return "avito"
	}
	if strings.Contains(docURL, "cups.online") {
		return "cupsonline"
	}
	if strings.Contains(docURL, "telemost.yandex.ru") {
		return "telemost"
	}
	if strings.Contains(docURL, "bitrix24") {
		return "bitrix24"
	}

	// Volga is strictly used on /edit/d/ documents
	if strings.Contains(docURL, "/edit/d/") {
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
		prev := client.transport
		client.running = false
		client.transport = nil
		client.packets = nil
		client.mu.Unlock()
		if prev != nil {
			_ = prev.Stop()
		}
		client.mu.Lock()
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
	case "avito":
		appendLog("[ANDROID] Обнаружен сервис Avito. Запуск транспорта avito")
		innerTrans = avito.NewAvitoTransport(documentURL, config, "", true)
	case "cupsonline":
		appendLog("[ANDROID] Обнаружен сервис Cups.online. Запуск транспорта cupsonline")
		innerTrans = cupsonline.NewCupsonlineTransport(documentURL, config, true)
	case "telemost":
		appendLog("[ANDROID] Обнаружена конференция Telemost. Запуск транспорта telemost")
		innerTrans = yandex.NewTelemostTransport("", documentURL, false, config)
	case "bitrix24":
		appendLog("[ANDROID] Обнаружен сервис Bitrix24. Запуск транспорта bitrix24")
		innerTrans = bitrix24.NewBitrix24Transport(documentURL, config, os.Getenv("B24_COOKIES"), os.Getenv("B24_SESSID"), os.Getenv("B24_CHAT_ID"))
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
