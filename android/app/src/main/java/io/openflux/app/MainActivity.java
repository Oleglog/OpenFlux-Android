package io.openflux.app;

import android.app.Activity;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.res.ColorStateList;
import android.content.res.Configuration;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
import android.net.VpnService;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.text.InputType;
import android.text.method.PasswordTransformationMethod;
import android.view.Gravity;
import android.view.View;
import android.view.Window;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ImageButton;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.Switch;
import android.widget.TextView;
import android.widget.Toast;

import java.security.SecureRandom;
import java.util.ArrayList;

import android.util.Base64;

import io.openflux.bridge.mobile.Mobile;

public final class MainActivity extends Activity {
    private static final int VPN_PERMISSION_REQUEST = 42;
    private static final String DEFAULT_DNS = "1.1.1.1";
    private static final int DEFAULT_MTU = 1400;
    private static final int PAGE_HOME = 0;
    private static final int PAGE_LOGS = 1;
    private static final int PAGE_SETTINGS = 2;

    private final Handler handler = new Handler(Looper.getMainLooper());
    private boolean darkMode;
    private boolean urlVisible;
    private boolean autoScroll = true;
    private int currentPage = PAGE_HOME;
    private int background;
    private int surface;
    private int text;
    private int secondary;
    private int border;
    private int accent;
    private int hint;
    private int logColor;

    private LinearLayout root;
    private FrameLayout content;
    private EditText urlInput;
    private EditText dnsInput;
    private EditText mtuInput;
    private ImageButton visibilityButton;
    private TextView statusDot;
    private TextView statusView;
    private TextView statusDetail;
    private LinearLayout pingPanel;
    private TextView pingValue;
    private PingGraphView pingGraph;
    private TextView logView;
    private ScrollView logScroll;
    private LinearLayout vpnButton;
    private TextView vpnButtonText;
    private String documentUrl;
    private String dnsServer;
    private int mtu;
    private String logs = "";
    private String lastShownError = "";
    private SecureSettings secureSettings;
    private final ArrayList<Float> pingHistory = new ArrayList<>();
    private long lastPingRequestAt;
    private long lastPingSequence;
    private boolean pingPanelShown;

    private final Runnable refresh = new Runnable() {
        @Override public void run() {
            updateStatus();
            updatePing();
            String pending = Mobile.readLogs();
            if (pending != null && !pending.isEmpty()) appendLog(pending);
            handler.postDelayed(this, 500);
        }
    };

    @Override protected void onCreate(Bundle state) {
        super.onCreate(state);
        SharedPreferences prefs = getPreferences(MODE_PRIVATE);
        secureSettings = new SecureSettings(this);
        // Older prototype builds used plain preferences. Remove those values:
        // connection credentials now live only in the Keystore-backed store.
        prefs.edit().remove("document_url").remove("connection_document_url").apply();
        documentUrl = secureSettings.getString("document_url", "");
        dnsServer = prefs.getString("dns_server", DEFAULT_DNS);
        mtu = prefs.getInt("mtu", DEFAULT_MTU);
        autoScroll = prefs.getBoolean("auto_scroll", true);
        darkMode = prefs.contains("dark_mode")
                ? prefs.getBoolean("dark_mode", isSystemDark())
                : isSystemDark();
        applyPalette();
        configureSystemBars();
        buildShell();
        showPage(PAGE_HOME);
        appendLog("Готово. При первом запуске Android запросит разрешение на VPN.");
    }

    @Override protected void onStart() {
        super.onStart();
        handler.removeCallbacks(refresh);
        handler.post(refresh);
    }

    @Override protected void onStop() {
        handler.removeCallbacks(refresh);
        readSettingsFromViews();
        persistSettings();
        super.onStop();
    }

    private boolean isSystemDark() {
        return (getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK)
                == Configuration.UI_MODE_NIGHT_YES;
    }

    private void applyPalette() {
        if (darkMode) {
            background = Color.rgb(18, 18, 18);
            surface = Color.rgb(30, 30, 30);
            text = Color.rgb(241, 243, 244);
            secondary = Color.rgb(189, 193, 198);
            border = Color.rgb(60, 64, 67);
            accent = Color.rgb(138, 180, 248);
            hint = Color.rgb(154, 160, 166);
            logColor = Color.rgb(218, 220, 224);
        } else {
            background = Color.rgb(248, 249, 250);
            surface = Color.WHITE;
            text = Color.rgb(32, 33, 36);
            secondary = Color.rgb(95, 99, 104);
            border = Color.rgb(218, 220, 224);
            accent = Color.rgb(26, 115, 232);
            hint = Color.rgb(128, 134, 139);
            logColor = Color.rgb(60, 64, 67);
        }
    }

    private void configureSystemBars() {
        Window window = getWindow();
        window.setStatusBarColor(background);
        window.setNavigationBarColor(background);
        window.getDecorView().setSystemUiVisibility(darkMode ? 0
                : View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR | View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR);
    }

    private void buildShell() {
        int side = dp(20);
        int top = dp(16);
        int bottom = dp(6);
        root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(side, top, side, bottom);
        root.setBackgroundColor(background);
        root.setOnApplyWindowInsetsListener((view, insets) -> {
            view.setPadding(side, top + insets.getSystemWindowInsetTop(), side,
                    bottom + insets.getSystemWindowInsetBottom());
            return insets;
        });
        root.addView(buildCompactHeader(), new LinearLayout.LayoutParams(-1, dp(54)));

        content = new FrameLayout(this);
        LinearLayout.LayoutParams contentParams = new LinearLayout.LayoutParams(-1, 0, 1f);
        contentParams.topMargin = dp(16);
        root.addView(content, contentParams);
        root.addView(buildBottomNav(), new LinearLayout.LayoutParams(-1, dp(68)));
        setContentView(root);
    }

    private View buildCompactHeader() {
        LinearLayout header = new LinearLayout(this);
        header.setOrientation(LinearLayout.HORIZONTAL);
        header.setGravity(Gravity.CENTER_VERTICAL);

        ImageView logo = new ImageView(this);
        logo.setContentDescription("Логотип OpenFlux");
        logo.setScaleType(ImageView.ScaleType.FIT_CENTER);
        logo.setBackground(rounded(Color.rgb(43, 43, 43), Color.TRANSPARENT, 0, 10));
        logo.setImageResource(R.drawable.ic_openflux_foreground);
        logo.setClipToOutline(true);
        header.addView(logo, new LinearLayout.LayoutParams(dp(44), dp(44)));

        LinearLayout titles = new LinearLayout(this);
        titles.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams titlesParams = new LinearLayout.LayoutParams(0, -2, 1f);
        titlesParams.leftMargin = dp(12);
        TextView title = text("OpenFlux", 21, text, true);
        TextView subtitle = text("VPN через Yandex Docs", 12, secondary, false);
        titles.addView(title);
        titles.addView(subtitle);
        header.addView(titles, titlesParams);

        TextView beta = text("BETA", 10, accent, true);
        beta.setGravity(Gravity.CENTER);
        beta.setPadding(dp(9), dp(5), dp(9), dp(5));
        beta.setBackground(rounded(darkMode ? Color.rgb(38, 50, 68) : Color.rgb(232, 240, 254),
                Color.TRANSPARENT, 0, 12));
        header.addView(beta);
        return header;
    }

    private View buildBottomNav() {
        LinearLayout nav = new LinearLayout(this);
        nav.setOrientation(LinearLayout.HORIZONTAL);
        nav.setGravity(Gravity.CENTER);
        nav.setPadding(dp(4), dp(5), dp(4), dp(3));
        nav.setBackground(rounded(surface, border, 1, 16));
        nav.addView(navItem(R.drawable.ic_home, "Главная", PAGE_HOME), weighted());
        nav.addView(navItem(R.drawable.ic_terminal, "Логи", PAGE_LOGS), weighted());
        nav.addView(navItem(R.drawable.ic_settings, "Настройки", PAGE_SETTINGS), weighted());
        return nav;
    }

    private View navItem(int icon, String label, int page) {
        LinearLayout item = new LinearLayout(this);
        item.setOrientation(LinearLayout.VERTICAL);
        item.setGravity(Gravity.CENTER);
        item.setBackground(ripple(Color.TRANSPARENT, 14));
        ImageView image = new ImageView(this);
        image.setImageResource(icon);
        image.setImageTintList(ColorStateList.valueOf(page == currentPage ? accent : secondary));
        item.addView(image, new LinearLayout.LayoutParams(dp(24), dp(24)));
        TextView title = text(label, 11, page == currentPage ? accent : secondary, page == currentPage);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(-2, -2);
        titleParams.topMargin = dp(2);
        item.addView(title, titleParams);
        item.setOnClickListener(v -> showPage(page));
        return item;
    }

    private void showPage(int page) {
        captureSettings();
        currentPage = page;
        content.removeAllViews();
        View pageView = page == PAGE_HOME ? buildHomePage()
                : page == PAGE_LOGS ? buildLogsPage() : buildSettingsPage();
        content.addView(pageView, new FrameLayout.LayoutParams(-1, -1));
        LinearLayout oldNav = (LinearLayout) root.getChildAt(root.getChildCount() - 1);
        root.removeView(oldNav);
        root.addView(buildBottomNav(), new LinearLayout.LayoutParams(-1, dp(68)));
        updateStatus();
    }

    private View buildHomePage() {
        LinearLayout page = page();
        TextView heading = text("Подключение", 25, text, true);
        page.addView(heading);
        TextView intro = text("Защищённый системный VPN-туннель через документ-транспорт.", 13, secondary, false);
        LinearLayout.LayoutParams introParams = matchWrap();
        introParams.topMargin = dp(4);
        page.addView(intro, introParams);

        boolean documentConfigured = isValidDocumentUrl(documentUrl);
        String transportTitle = documentConfigured ? "Yandex Docs" : "Документ не указан";
        String transportDetail = !documentConfigured
                ? "Укажите HTTPS-ссылку во вкладке «Настройки»"
                : "Документ настроен";
        LinearLayout transport = cardRow(R.drawable.ic_link, transportTitle, transportDetail);
        transport.setClickable(true);
        transport.setFocusable(true);
        transport.setOnClickListener(v -> showPage(PAGE_SETTINGS));
        LinearLayout.LayoutParams transportParams = matchWrap();
        transportParams.topMargin = dp(28);
        page.addView(transport, transportParams);

        LinearLayout status = new LinearLayout(this);
        status.setOrientation(LinearLayout.HORIZONTAL);
        status.setGravity(Gravity.CENTER_VERTICAL);
        status.setPadding(dp(18), dp(18), dp(18), dp(18));
        status.setBackground(rounded(surface, border, 1, 12));
        status.setElevation(dp(1));
        statusDot = new TextView(this);
        LinearLayout.LayoutParams dot = new LinearLayout.LayoutParams(dp(13), dp(13));
        dot.rightMargin = dp(15);
        status.addView(statusDot, dot);
        LinearLayout statusCopy = new LinearLayout(this);
        statusCopy.setOrientation(LinearLayout.VERTICAL);
        pingPanelShown = false;
        pingPanel = new LinearLayout(this);
        pingPanel.setOrientation(LinearLayout.VERTICAL);
        pingPanel.setVisibility(View.GONE);
        pingPanel.setAlpha(0f);
        pingValue = text("Пинг до VDS: —", 13, accent, true);
        pingPanel.addView(pingValue);
        pingGraph = new PingGraphView(this, accent, border);
        pingGraph.setHistory(pingHistory);
        LinearLayout.LayoutParams graphParams = new LinearLayout.LayoutParams(-1, dp(48));
        graphParams.topMargin = dp(5);
        graphParams.bottomMargin = dp(9);
        pingPanel.addView(pingGraph, graphParams);
        statusCopy.addView(pingPanel, new LinearLayout.LayoutParams(-1, -2));
        statusView = text("Остановлено", 17, text, true);
        statusDetail = text("VPN сейчас не используется", 13, secondary, false);
        statusCopy.addView(statusView);
        statusCopy.addView(statusDetail);
        status.addView(statusCopy, new LinearLayout.LayoutParams(0, -2, 1f));
        LinearLayout.LayoutParams statusParams = matchWrap();
        statusParams.topMargin = dp(14);
        page.addView(status, statusParams);

        vpnButton = new LinearLayout(this);
        vpnButton.setOrientation(LinearLayout.HORIZONTAL);
        vpnButton.setGravity(Gravity.CENTER);
        vpnButton.setClickable(true);
        vpnButton.setFocusable(true);
        vpnButton.setElevation(dp(2));
        ImageView powerIcon = icon(R.drawable.ic_power, Color.WHITE);
        LinearLayout.LayoutParams powerParams = new LinearLayout.LayoutParams(dp(24), dp(24));
        powerParams.rightMargin = dp(10);
        vpnButton.addView(powerIcon, powerParams);
        vpnButtonText = text("Запустить VPN", 16, Color.WHITE, true);
        vpnButton.addView(vpnButtonText, new LinearLayout.LayoutParams(-2, -2));
        vpnButton.setOnClickListener(v -> toggleVpn());
        LinearLayout.LayoutParams buttonParams = new LinearLayout.LayoutParams(-1, dp(58));
        buttonParams.topMargin = dp(16);
        page.addView(vpnButton, buttonParams);

        TextView summaryTitle = label("АКТИВНЫЕ ПАРАМЕТРЫ");
        LinearLayout.LayoutParams summaryTitleParams = matchWrap();
        summaryTitleParams.topMargin = dp(30);
        summaryTitleParams.bottomMargin = dp(8);
        page.addView(summaryTitle, summaryTitleParams);
        page.addView(infoCard("DNS-сервер", dnsServer, "MTU пакета", String.valueOf(mtu)));
        return page;
    }

    private View buildLogsPage() {
        LinearLayout page = page();
        LinearLayout header = new LinearLayout(this);
        header.setGravity(Gravity.CENTER_VERTICAL);
        TextView heading = text("Журнал событий", 25, text, true);
        header.addView(heading, new LinearLayout.LayoutParams(0, -2, 1f));
        ImageButton clear = iconButton(R.drawable.ic_delete, "Очистить журнал");
        clear.setOnClickListener(v -> {
            logs = "";
            logView.setText("");
        });
        header.addView(clear, new LinearLayout.LayoutParams(dp(48), dp(48)));
        page.addView(header);

        logView = text(logs, 12, logColor, false);
        logView.setTypeface(Typeface.MONOSPACE);
        logView.setTextIsSelectable(true);
        logView.setPadding(dp(14), dp(12), dp(14), dp(12));
        logScroll = new ScrollView(this);
        logScroll.setFillViewport(true);
        logScroll.setBackground(rounded(surface, border, 1, 10));
        logScroll.addView(logView, new ScrollView.LayoutParams(-1, -2));
        LinearLayout.LayoutParams logParams = new LinearLayout.LayoutParams(-1, 0, 1f);
        logParams.topMargin = dp(12);
        logParams.bottomMargin = dp(10);
        page.addView(logScroll, logParams);
        TextView note = text("Логи хранятся только до закрытия приложения.", 11, secondary, false);
        note.setGravity(Gravity.CENTER);
        page.addView(note);
        return page;
    }

    private View buildSettingsPage() {
        ScrollView scroll = new ScrollView(this);
        scroll.setFillViewport(true);
        LinearLayout page = page();
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        page.addView(text("Настройки", 25, text, true));
        TextView restartHint = text("Параметры сети применяются при следующем подключении.", 12, secondary, false);
        LinearLayout.LayoutParams hintParams = matchWrap();
        hintParams.topMargin = dp(4);
        page.addView(restartHint, hintParams);

        TextView transportLabel = label("ТРАНСПОРТ");
        LinearLayout.LayoutParams transportLabelParams = matchWrap();
        transportLabelParams.topMargin = dp(24);
        transportLabelParams.bottomMargin = dp(8);
        page.addView(transportLabel, transportLabelParams);
        page.addView(buildUrlField(), new LinearLayout.LayoutParams(-1, dp(56)));

        TextView networkLabel = label("СЕТЬ");
        LinearLayout.LayoutParams networkLabelParams = matchWrap();
        networkLabelParams.topMargin = dp(22);
        networkLabelParams.bottomMargin = dp(8);
        page.addView(networkLabel, networkLabelParams);
        dnsInput = settingInput("DNS-сервер", dnsServer, InputType.TYPE_CLASS_PHONE);
        page.addView(settingRow(R.drawable.ic_public, "DNS-сервер", dnsInput));
        mtuInput = settingInput("MTU", String.valueOf(mtu), InputType.TYPE_CLASS_NUMBER);
        LinearLayout.LayoutParams mtuParams = matchWrap();
        mtuParams.topMargin = dp(8);
        page.addView(settingRow(R.drawable.ic_settings, "MTU пакета", mtuInput), mtuParams);

        TextView appearanceLabel = label("ИНТЕРФЕЙС");
        LinearLayout.LayoutParams appearanceParams = matchWrap();
        appearanceParams.topMargin = dp(22);
        appearanceParams.bottomMargin = dp(8);
        page.addView(appearanceLabel, appearanceParams);
        Switch themeSwitch = settingSwitch(R.drawable.ic_dark_mode, "Тёмная тема",
                "До первого выбора используется тема телефона", darkMode);
        themeSwitch.setOnCheckedChangeListener((button, checked) -> switchTheme(checked));
        page.addView((View) themeSwitch.getTag());
        Switch scrollSwitch = settingSwitch(R.drawable.ic_terminal, "Автопрокрутка логов",
                "Показывать последние события", autoScroll);
        scrollSwitch.setOnCheckedChangeListener((button, checked) -> {
            autoScroll = checked;
            getPreferences(MODE_PRIVATE).edit().putBoolean("auto_scroll", checked).apply();
        });
        LinearLayout.LayoutParams scrollSettingParams = matchWrap();
        scrollSettingParams.topMargin = dp(8);
        page.addView((View) scrollSwitch.getTag(), scrollSettingParams);

        Button save = new Button(this);
        save.setText("Сохранить настройки");
        save.setAllCaps(false);
        save.setTextColor(Color.WHITE);
        save.setTextSize(15);
        save.setTypeface(Typeface.DEFAULT_BOLD);
        save.setStateListAnimator(null);
        save.setBackground(buttonBackground(Color.rgb(26, 115, 232), Color.rgb(23, 78, 166)));
        save.setOnClickListener(v -> {
            readSettingsFromViews();
            persistSettings();
            Toast.makeText(this, "Настройки сохранены", Toast.LENGTH_SHORT).show();
        });
        LinearLayout.LayoutParams saveParams = new LinearLayout.LayoutParams(-1, dp(52));
        saveParams.topMargin = dp(20);
        saveParams.bottomMargin = dp(12);
        page.addView(save, saveParams);
        return scroll;
    }

    private View buildUrlField() {
        FrameLayout field = new FrameLayout(this);
        field.setBackground(rounded(surface, border, 1, 10));
        urlInput = settingInput("HTTPS-ссылка на документ", documentUrl,
                InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        urlInput.setTransformationMethod(urlVisible ? null : PasswordTransformationMethod.getInstance());
        urlInput.setPadding(dp(16), 0, dp(56), 0);
        field.addView(urlInput, new FrameLayout.LayoutParams(-1, -1));
        visibilityButton = iconButton(urlVisible ? R.drawable.ic_visibility_off : R.drawable.ic_visibility,
                urlVisible ? "Скрыть ссылку" : "Показать ссылку");
        visibilityButton.setOnClickListener(v -> toggleUrlVisibility());
        FrameLayout.LayoutParams eye = new FrameLayout.LayoutParams(dp(48), dp(48), Gravity.END | Gravity.CENTER_VERTICAL);
        eye.rightMargin = dp(4);
        field.addView(visibilityButton, eye);
        return field;
    }

    private LinearLayout cardRow(int iconRes, String titleValue, String detailValue) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(16), dp(14), dp(16), dp(14));
        row.setBackground(rounded(surface, border, 1, 11));
        ImageView icon = icon(iconRes, accent);
        row.addView(icon, new LinearLayout.LayoutParams(dp(26), dp(26)));
        LinearLayout copy = new LinearLayout(this);
        copy.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams copyParams = new LinearLayout.LayoutParams(0, -2, 1f);
        copyParams.leftMargin = dp(14);
        copy.addView(text(titleValue, 15, text, true));
        copy.addView(text(detailValue, 12, secondary, false));
        row.addView(copy, copyParams);
        return row;
    }

    private View infoCard(String leftTitle, String leftValue, String rightTitle, String rightValue) {
        LinearLayout card = new LinearLayout(this);
        card.setPadding(dp(16), dp(14), dp(16), dp(14));
        card.setBackground(rounded(surface, border, 1, 11));
        card.addView(infoColumn(leftTitle, leftValue), weighted());
        card.addView(infoColumn(rightTitle, rightValue), weighted());
        return card;
    }

    private View infoColumn(String titleValue, String value) {
        LinearLayout column = new LinearLayout(this);
        column.setOrientation(LinearLayout.VERTICAL);
        column.addView(text(titleValue, 11, secondary, false));
        TextView valueView = text(value, 16, text, true);
        LinearLayout.LayoutParams valueParams = matchWrap();
        valueParams.topMargin = dp(3);
        column.addView(valueView, valueParams);
        return column;
    }

    private View settingRow(int iconRes, String labelValue, EditText input) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(14), dp(7), dp(10), dp(7));
        row.setBackground(rounded(surface, border, 1, 10));
        row.addView(icon(iconRes, secondary), new LinearLayout.LayoutParams(dp(24), dp(24)));
        TextView title = text(labelValue, 14, text, false);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(0, -2, 1f);
        titleParams.leftMargin = dp(12);
        row.addView(title, titleParams);
        row.addView(input, new LinearLayout.LayoutParams(dp(120), dp(46)));
        return row;
    }

    private Switch settingSwitch(int iconRes, String titleValue, String detailValue, boolean checked) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(14), dp(10), dp(10), dp(10));
        row.setBackground(rounded(surface, border, 1, 10));
        row.addView(icon(iconRes, secondary), new LinearLayout.LayoutParams(dp(24), dp(24)));
        LinearLayout copy = new LinearLayout(this);
        copy.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams copyParams = new LinearLayout.LayoutParams(0, -2, 1f);
        copyParams.leftMargin = dp(12);
        copy.addView(text(titleValue, 14, text, false));
        copy.addView(text(detailValue, 11, secondary, false));
        row.addView(copy, copyParams);
        Switch toggle = new Switch(this);
        toggle.setChecked(checked);
        toggle.setContentDescription(titleValue);
        row.addView(toggle, new LinearLayout.LayoutParams(-2, dp(42)));
        toggle.setTag(row);
        return toggle;
    }

    private EditText settingInput(String fieldHint, String value, int inputType) {
        EditText input = new EditText(this);
        input.setHint(fieldHint);
        input.setHintTextColor(hint);
        input.setText(value);
        input.setSingleLine(true);
        input.setTextSize(14);
        input.setTextColor(text);
        input.setInputType(inputType);
        input.setBackgroundColor(Color.TRANSPARENT);
        input.setPadding(dp(8), 0, dp(8), 0);
        return input;
    }

    private void switchTheme(boolean checked) {
        if (darkMode == checked) return;
        captureSettings();
        darkMode = checked;
        getPreferences(MODE_PRIVATE).edit().putBoolean("dark_mode", darkMode).apply();
        applyPalette();
        configureSystemBars();
        buildShell();
        showPage(currentPage);
    }

    private void toggleUrlVisibility() {
        int position = urlInput.getSelectionStart();
        urlVisible = !urlVisible;
        urlInput.setTransformationMethod(urlVisible ? null : PasswordTransformationMethod.getInstance());
        urlInput.setTypeface(Typeface.DEFAULT);
        visibilityButton.setImageResource(urlVisible ? R.drawable.ic_visibility_off : R.drawable.ic_visibility);
        visibilityButton.setContentDescription(urlVisible ? "Скрыть ссылку" : "Показать ссылку");
        urlInput.setSelection(Math.max(0, Math.min(position, urlInput.length())));
    }

    private void captureSettings() {
        readSettingsFromViews();
        persistSettings();
        urlInput = null;
        dnsInput = null;
        mtuInput = null;
        logView = null;
        logScroll = null;
    }

    private void readSettingsFromViews() {
        if (urlInput != null) documentUrl = urlInput.getText().toString().trim();
        if (dnsInput != null) dnsServer = dnsInput.getText().toString().trim();
        if (mtuInput != null) {
            try { mtu = Integer.parseInt(mtuInput.getText().toString()); }
            catch (NumberFormatException ignored) { mtu = DEFAULT_MTU; }
            mtu = Math.max(576, Math.min(1500, mtu));
        }
        if (logView != null) logs = logView.getText().toString();
    }

    private void persistSettings() {
        if (dnsServer.isEmpty()) dnsServer = DEFAULT_DNS;
        secureSettings.putString("document_url", documentUrl);
        getPreferences(MODE_PRIVATE).edit()
                .remove("connection_document_url")
                .putString("dns_server", dnsServer)
                .putInt("mtu", mtu)
                .putBoolean("auto_scroll", autoScroll)
                .putBoolean("dark_mode", darkMode)
                .commit();
    }

    private void toggleVpn() {
        if (OpenFluxVpnService.isRunning()) {
            Intent stop = new Intent(this, OpenFluxVpnService.class);
            stop.setAction(OpenFluxVpnService.ACTION_STOP);
            startService(stop);
            appendLog("Запрошена остановка VPN");
            return;
        }
        if (!isValidDocumentUrl(documentUrl)) {
            Toast.makeText(this, "Укажите корректную HTTPS-ссылку в настройках", Toast.LENGTH_LONG).show();
            showPage(PAGE_SETTINGS);
            return;
        }
        persistSettings();
        Intent permission = VpnService.prepare(this);
        if (permission != null) startActivityForResult(permission, VPN_PERMISSION_REQUEST);
        else startVpn();
    }

    @Override protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == VPN_PERMISSION_REQUEST && resultCode == RESULT_OK) startVpn();
        else if (requestCode == VPN_PERMISSION_REQUEST) appendLog("Разрешение на создание VPN не выдано");
    }

    private void startVpn() {
        Intent intent = new Intent(this, OpenFluxVpnService.class);
        intent.setAction(OpenFluxVpnService.ACTION_START);
        intent.putExtra(OpenFluxVpnService.EXTRA_DOCUMENT_URL, documentUrl);
        intent.putExtra(OpenFluxVpnService.EXTRA_DNS_SERVER, dnsServer);
        intent.putExtra(OpenFluxVpnService.EXTRA_MTU, mtu);
        startForegroundService(intent);
        appendLog("Запуск VPN…");
    }

    private void updateStatus() {
        if (statusView == null || vpnButton == null) return;
        String state = OpenFluxVpnService.getStatus();
        boolean running = OpenFluxVpnService.isRunning();
        statusView.setText(state);
        vpnButtonText.setText(running ? "Остановить VPN" : "Запустить VPN");
        int stateColor;
        if ("Подключено".equals(state)) {
            stateColor = darkMode ? Color.rgb(129, 201, 149) : Color.rgb(24, 128, 56);
            statusDetail.setText("Трафик направляется через OpenFlux");
        } else if ("Ошибка".equals(state)) {
            stateColor = darkMode ? Color.rgb(242, 139, 130) : Color.rgb(217, 48, 37);
            statusDetail.setText("Откройте вкладку «Логи»");
        } else if (state != null && (state.contains("Подключ") || state.contains("Останав"))) {
            stateColor = darkMode ? Color.rgb(253, 214, 99) : Color.rgb(249, 171, 0);
            statusDetail.setText("Подождите несколько секунд…");
        } else {
            stateColor = Color.rgb(154, 160, 166);
            statusDetail.setText("VPN сейчас не используется");
        }
        statusDot.setBackground(rounded(stateColor, Color.TRANSPARENT, 0, 8));
        vpnButton.setBackground(buttonBackground(running ? Color.rgb(217, 48, 37) : Color.rgb(26, 115, 232),
                running ? Color.rgb(183, 28, 28) : Color.rgb(23, 78, 166)));
        String error = OpenFluxVpnService.getLastError();
        if (error != null && !error.isEmpty() && !error.equals(lastShownError)) {
            lastShownError = error;
            appendLog("Ошибка: " + error);
        }
    }

    private void updatePing() {
        boolean connected = OpenFluxVpnService.isRunning()
                && "Подключено".equals(OpenFluxVpnService.getStatus());
        if (!connected) {
            lastPingRequestAt = 0;
            lastPingSequence = 0;
            setPingPanelVisible(false);
            return;
        }

        long sequence = Mobile.pingSequence();
        if (sequence == 0) {
            setPingPanelVisible(false);
            return;
        }
        setPingPanelVisible(true);
        if (sequence == lastPingSequence) return;
        lastPingSequence = sequence;
        long milliseconds = Mobile.pingMillis();
        if (milliseconds < 0) return;
        if (pingHistory.size() >= 32) pingHistory.remove(0);
        pingHistory.add((float) milliseconds);
        if (pingValue != null) pingValue.setText("Пинг до VDS: " + milliseconds + " мс");
        if (pingGraph != null) pingGraph.addSample(milliseconds);
    }

    private void setPingPanelVisible(boolean visible) {
        if (pingPanel == null || pingPanelShown == visible) return;
        pingPanelShown = visible;
        pingPanel.animate().cancel();
        if (visible) {
            pingPanel.setVisibility(View.VISIBLE);
            pingPanel.setAlpha(0f);
            pingPanel.setTranslationY(dp(8));
            pingPanel.animate().alpha(1f).translationY(0f).setDuration(450).start();
        } else {
            pingPanel.animate().alpha(0f).translationY(dp(8)).setDuration(220)
                    .withEndAction(() -> {
                        if (!pingPanelShown && pingPanel != null) pingPanel.setVisibility(View.GONE);
                    }).start();
        }
    }

    private boolean isValidDocumentUrl(String value) {
        if (value == null || !value.startsWith("https://")) return false;
        try {
            android.net.Uri uri = android.net.Uri.parse(value);
            return uri.getHost() != null && !uri.getHost().isEmpty();
        } catch (Exception ignored) {
            return false;
        }
    }

    private void appendLog(String value) {
        if (!logs.isEmpty()) logs += "\n";
        logs += value;
        if (logs.length() > 60000) logs = logs.substring(logs.length() - 40000);
        if (logView != null) {
            logView.setText(logs);
            if (autoScroll && logScroll != null) logScroll.post(() -> logScroll.fullScroll(View.FOCUS_DOWN));
        }
    }

    private LinearLayout page() {
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        return page;
    }

    private TextView text(String value, int size, int color, boolean bold) {
        TextView view = new TextView(this);
        view.setText(value);
        view.setTextSize(size);
        view.setTextColor(color);
        if (bold) view.setTypeface(Typeface.DEFAULT_BOLD);
        return view;
    }

    private TextView label(String value) {
        TextView view = text(value, 11, secondary, true);
        view.setLetterSpacing(0.08f);
        return view;
    }

    private ImageView icon(int resource, int color) {
        ImageView view = new ImageView(this);
        view.setImageResource(resource);
        view.setImageTintList(ColorStateList.valueOf(color));
        return view;
    }

    private ImageButton iconButton(int resource, String description) {
        ImageButton button = new ImageButton(this);
        button.setImageResource(resource);
        button.setImageTintList(ColorStateList.valueOf(secondary));
        button.setContentDescription(description);
        button.setPadding(dp(11), dp(11), dp(11), dp(11));
        button.setBackground(ripple(Color.TRANSPARENT, 24));
        return button;
    }

    private GradientDrawable rounded(int fill, int stroke, int strokeWidth, int radius) {
        GradientDrawable drawable = new GradientDrawable();
        drawable.setColor(fill);
        drawable.setCornerRadius(dp(radius));
        if (strokeWidth > 0) drawable.setStroke(dp(strokeWidth), stroke);
        return drawable;
    }

    private RippleDrawable ripple(int fill, int radius) {
        return new RippleDrawable(ColorStateList.valueOf(darkMode ? 0x2FFFFFFF : 0x1F1A73E8),
                rounded(fill, Color.TRANSPARENT, 0, radius), rounded(Color.WHITE, Color.TRANSPARENT, 0, radius));
    }

    private RippleDrawable buttonBackground(int fill, int pressed) {
        return new RippleDrawable(ColorStateList.valueOf(pressed), rounded(fill, Color.TRANSPARENT, 0, 9),
                rounded(Color.WHITE, Color.TRANSPARENT, 0, 9));
    }

    private LinearLayout.LayoutParams matchWrap() { return new LinearLayout.LayoutParams(-1, -2); }
    private LinearLayout.LayoutParams weighted() { return new LinearLayout.LayoutParams(0, -1, 1f); }
    private int dp(int value) { return Math.round(value * getResources().getDisplayMetrics().density); }
}
