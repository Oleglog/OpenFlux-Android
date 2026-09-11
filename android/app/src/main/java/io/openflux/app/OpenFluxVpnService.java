package io.openflux.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.VpnService;
import android.os.ParcelFileDescriptor;

import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Arrays;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;

import io.openflux.bridge.mobile.Mobile;

public final class OpenFluxVpnService extends VpnService {
    public static final String ACTION_START = "io.openflux.app.START";
    public static final String ACTION_STOP = "io.openflux.app.STOP";
    public static final String EXTRA_DOCUMENT_URL = "document_url";
    public static final String EXTRA_DNS_SERVER = "dns_server";
    public static final String EXTRA_MTU = "mtu";

    private static final String CHANNEL_ID = "openflux_vpn";
    private static final int NOTIFICATION_ID = 7;
    private static volatile boolean running;
    private static volatile String status = "Остановлено";
    private static volatile String lastError = "";

    private final ExecutorService workers = Executors.newCachedThreadPool();
    private final Object outputLock = new Object();
    private final AtomicInteger generation = new AtomicInteger();
    private volatile boolean active;
    private ParcelFileDescriptor tunnel;
    private FileInputStream tunnelInput;
    private FileOutputStream tunnelOutput;

    public static boolean isRunning() { return running; }
    public static String getStatus() { return status; }
    public static String getLastError() { return lastError; }

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && ACTION_STOP.equals(intent.getAction())) {
            stopVpn();
            return START_NOT_STICKY;
        }
        if (running) return START_STICKY;

        String url = intent == null ? null : intent.getStringExtra(EXTRA_DOCUMENT_URL);
        if (url == null || !url.startsWith("https://")) {
            lastError = "Некорректная ссылка на документ";
            status = "Ошибка";
            running = false;
            stopSelf();
            return START_NOT_STICKY;
        }
        String dnsServer = intent.getStringExtra(EXTRA_DNS_SERVER);
        if (dnsServer == null || dnsServer.trim().isEmpty()) dnsServer = "1.1.1.1";
        int mtu = Math.max(576, Math.min(1500, intent.getIntExtra(EXTRA_MTU, 1400)));

        createNotificationChannel();
        startForeground(NOTIFICATION_ID, notification("Подключение…"));
        active = true;
        running = true;
        status = "Подключение…";
        lastError = "";
        int session = generation.incrementAndGet();
        String selectedDns = dnsServer;
        int selectedMtu = mtu;
        workers.execute(() -> startTunnel(url, selectedDns, selectedMtu, session));
        return START_STICKY;
    }

    private void startTunnel(String url, String dnsServer, int mtu, int session) {
        if (!isCurrent(session)) return;
        String error = Mobile.start(url);
        if (error != null && !error.isEmpty()) {
            fail(session, error);
            return;
        }

        for (int attempt = 0; isCurrent(session) && !Mobile.isConnected() && attempt < 120; attempt++) {
            try { Thread.sleep(250); }
            catch (InterruptedException interrupted) {
                Thread.currentThread().interrupt();
                return;
            }
        }
        if (!isCurrent(session)) return;
        if (!Mobile.isConnected()) {
            fail(session, "Yandex-транспорт не подключился за 30 секунд");
            return;
        }

        try {
            Builder builder = new Builder()
                    .setSession("OpenFlux")
                    .setMtu(mtu)
                    .addAddress("10.10.10.2", 24)
                    .addRoute("0.0.0.0", 0)
                    .addDnsServer(dnsServer);
            // Go opens the Yandex connection inside this process. Excluding our
            // own package prevents that control connection from entering TUN.
            builder.addDisallowedApplication(getPackageName());
            ParcelFileDescriptor established = builder.establish();
            if (established == null) throw new IOException("Android не создал TUN-интерфейс");
            synchronized (outputLock) {
                if (!isCurrent(session)) {
                    established.close();
                    return;
                }
                tunnel = established;
                tunnelInput = new FileInputStream(established.getFileDescriptor());
                tunnelOutput = new FileOutputStream(established.getFileDescriptor());
            }
        } catch (PackageManager.NameNotFoundException | IOException | IllegalArgumentException exception) {
            fail(session, exception.getMessage());
            return;
        }

        if (!isCurrent(session)) return;
        status = "Подключено";
        updateNotification("VPN подключён");
        FileInputStream input = tunnelInput;
        FileOutputStream output = tunnelOutput;
        workers.execute(() -> readOutgoingPackets(session, input, dnsServer));
        workers.execute(() -> writeIncomingPackets(session, output));
    }

    private void readOutgoingPackets(int session, FileInputStream input, String dnsServer) {
        byte[] buffer = new byte[32767];
        try {
            while (isCurrent(session)) {
                int length = input.read(buffer);
                if (length <= 0) continue;
                byte[] packet = Arrays.copyOf(buffer, length);
                if (isIpv4UdpDns(packet)) {
                    workers.execute(() -> forwardDns(session, outputFor(session), packet, dnsServer));
                } else if (isIpv4Tcp(packet)) {
                    String error = Mobile.send(packet);
                    if (error != null && !error.isEmpty() && isCurrent(session)) {
                        lastError = "Отправка пакета: " + error;
                    }
                }
            }
        } catch (IOException exception) {
            if (isCurrent(session)) fail(session, "Чтение TUN: " + exception.getMessage());
        }
    }

    private void writeIncomingPackets(int session, FileOutputStream output) {
        try {
            while (isCurrent(session)) {
                byte[] packet = Mobile.read();
                if (packet == null || packet.length == 0) {
                    Thread.sleep(2);
                    continue;
                }
                inject(session, output, packet);
            }
        } catch (IOException exception) {
            if (isCurrent(session)) fail(session, "Запись TUN: " + exception.getMessage());
        } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
        }
    }

    private void forwardDns(int session, FileOutputStream output, byte[] request, String dnsServer) {
        int ipHeader = (request[0] & 0x0f) * 4;
        int dnsOffset = ipHeader + 8;
        int udpLength = unsignedShort(request, ipHeader + 4);
        if (dnsOffset > request.length || udpLength < 8 || ipHeader + udpLength > request.length) return;

        byte[] query = Arrays.copyOfRange(request, dnsOffset, ipHeader + udpLength);
        try {
            byte[] answer = queryDnsOverHttps(query, dnsServer);
            inject(session, output, buildDnsResponse(request, answer));
        } catch (Exception exception) {
            if (isCurrent(session)) lastError = "DNS: " + exception.getMessage();
        }
    }

    private byte[] queryDnsOverHttps(byte[] query, String dnsServer) throws IOException {
        String endpoint;
        switch (dnsServer) {
            case "8.8.8.8":
            case "8.8.4.4":
                endpoint = "https://dns.google/dns-query";
                break;
            case "9.9.9.9":
            case "149.112.112.112":
                endpoint = "https://dns.9.9.9.9/dns-query";
                break;
            case "1.1.1.1":
            case "1.0.0.1":
                endpoint = "https://cloudflare-dns.com/dns-query";
                break;
            default:
                throw new IOException("поддерживаются DNS 1.1.1.1, 8.8.8.8 и 9.9.9.9");
        }

        HttpURLConnection connection = (HttpURLConnection) new URL(endpoint).openConnection();
        connection.setConnectTimeout(5000);
        connection.setReadTimeout(5000);
        connection.setRequestMethod("POST");
        connection.setRequestProperty("Accept", "application/dns-message");
        connection.setRequestProperty("Content-Type", "application/dns-message");
        connection.setDoOutput(true);
        connection.setFixedLengthStreamingMode(query.length);
        try {
            connection.getOutputStream().write(query);
            int statusCode = connection.getResponseCode();
            if (statusCode != HttpURLConnection.HTTP_OK) {
                throw new IOException("DoH вернул HTTP " + statusCode);
            }
            try (java.io.InputStream input = connection.getInputStream();
                 java.io.ByteArrayOutputStream output = new java.io.ByteArrayOutputStream()) {
                byte[] buffer = new byte[2048];
                int read;
                while ((read = input.read(buffer)) != -1) {
                    output.write(buffer, 0, read);
                    if (output.size() > 65535) throw new IOException("слишком большой DNS-ответ");
                }
                return output.toByteArray();
            }
        } finally {
            connection.disconnect();
        }
    }

    private void inject(int session, FileOutputStream output, byte[] packet) throws IOException {
        if (!isCurrent(session) || output == null || packet == null) return;
        synchronized (outputLock) {
            if (isCurrent(session)) output.write(packet);
        }
    }

    private FileOutputStream outputFor(int session) {
        return isCurrent(session) ? tunnelOutput : null;
    }

    private boolean isCurrent(int session) {
        return active && generation.get() == session;
    }

    private static boolean isIpv4Tcp(byte[] packet) {
        return packet.length >= 20 && (packet[0] >>> 4) == 4 && (packet[9] & 0xff) == 6;
    }

    private static boolean isIpv4UdpDns(byte[] packet) {
        if (packet.length < 28 || (packet[0] >>> 4) != 4 || (packet[9] & 0xff) != 17) return false;
        int header = (packet[0] & 0x0f) * 4;
        return header >= 20 && packet.length >= header + 8 && unsignedShort(packet, header + 2) == 53;
    }

    private static byte[] buildDnsResponse(byte[] request, byte[] dns) {
        int requestHeader = (request[0] & 0x0f) * 4;
        byte[] response = new byte[20 + 8 + dns.length];
        response[0] = 0x45;
        response[1] = request[1];
        putShort(response, 2, response.length);
        response[4] = request[4];
        response[5] = request[5];
        response[8] = 64;
        response[9] = 17;
        System.arraycopy(request, 16, response, 12, 4);
        System.arraycopy(request, 12, response, 16, 4);
        putShort(response, 10, checksum(response, 0, 20));

        putShort(response, 20, 53);
        putShort(response, 22, unsignedShort(request, requestHeader));
        putShort(response, 24, 8 + dns.length);
        // A zero UDP checksum is valid for IPv4.
        putShort(response, 26, 0);
        System.arraycopy(dns, 0, response, 28, dns.length);
        return response;
    }

    private static int checksum(byte[] bytes, int offset, int length) {
        long sum = 0;
        for (int i = offset; i < offset + length; i += 2) {
            int high = bytes[i] & 0xff;
            int low = i + 1 < offset + length ? bytes[i + 1] & 0xff : 0;
            sum += (high << 8) | low;
            while ((sum & 0xffff0000L) != 0) sum = (sum & 0xffffL) + (sum >>> 16);
        }
        return (int) (~sum) & 0xffff;
    }

    private static int unsignedShort(byte[] bytes, int offset) {
        return ((bytes[offset] & 0xff) << 8) | (bytes[offset + 1] & 0xff);
    }

    private static void putShort(byte[] bytes, int offset, int value) {
        bytes[offset] = (byte) (value >>> 8);
        bytes[offset + 1] = (byte) value;
    }

    private synchronized void fail(int session, String message) {
        if (!isCurrent(session)) return;
        lastError = message == null ? "Неизвестная ошибка" : message;
        status = "Ошибка";
        generation.incrementAndGet();
        active = false;
        closeTunnel();
        Mobile.stop();
        running = false;
        stopForeground(STOP_FOREGROUND_REMOVE);
        stopSelf();
    }

    private synchronized void stopVpn() {
        status = "Останавливается…";
        generation.incrementAndGet();
        active = false;
        closeTunnel();
        Mobile.stop();
        running = false;
        status = "Остановлено";
        lastError = "";
        stopForeground(STOP_FOREGROUND_REMOVE);
        stopSelf();
    }

    @Override public void onDestroy() {
        generation.incrementAndGet();
        active = false;
        closeTunnel();
        Mobile.stop();
        running = false;
        if (!"Ошибка".equals(status)) status = "Остановлено";
        workers.shutdownNow();
        super.onDestroy();
    }

    private void closeTunnel() {
        synchronized (outputLock) {
            if (tunnel != null) {
                try { tunnel.close(); } catch (IOException ignored) { }
                tunnel = null;
            }
            tunnelInput = null;
            tunnelOutput = null;
        }
    }

    private void createNotificationChannel() {
        NotificationManager manager = getSystemService(NotificationManager.class);
        manager.createNotificationChannel(new NotificationChannel(
                CHANNEL_ID, "OpenFlux VPN", NotificationManager.IMPORTANCE_LOW));
    }

    private Notification notification(String text) {
        Intent open = new Intent(this, MainActivity.class);
        PendingIntent content = PendingIntent.getActivity(
                this, 0, open, PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        return new Notification.Builder(this, CHANNEL_ID)
                .setContentTitle("OpenFlux")
                .setContentText(text)
                .setSmallIcon(R.drawable.ic_openflux_notification)
                .setOngoing(true)
                .setContentIntent(content)
                .build();
    }

    private void updateNotification(String text) {
        getSystemService(NotificationManager.class).notify(NOTIFICATION_ID, notification(text));
    }
}
