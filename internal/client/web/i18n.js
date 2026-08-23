(() => {
  'use strict';

  const storageKey = 'local-clipboard.locale';
  const supportedLocales = new Set(['zh', 'en']);
  const messages = Object.freeze({
    zh: Object.freeze({
      documentTitle: 'Local Clipboard 控制台',
      languageSwitcher: '界面语言',
      sendToDevice: '发送到设备',
      sendClipboard: '发送剪贴板',
      sendFiles: '发送文件',
      targetPlaceholder: '目标设备名（留空则发送给所有设备）',
      hotkeyHint: '使用全局快捷键复制并发送',
      pendingFiles: '待接收文件',
      receiveAll: '全部接收',
      transferHistory: '传输记录',
      connected: '已连接',
      reconnecting: '离线，正在重连',
      consoleOffline: '控制台离线',
      clipboard: '（剪贴板）',
      clipboardContent: '（剪贴板内容）',
      fromPeer: '来自 {peer}',
      receive: '接收',
      retry: '重试',
      noActiveTransfers: '没有进行中的传输',
      noHistory: '暂无记录',
      directionIn: '收',
      directionOut: '发',
      kindText: '文本',
      kindImage: '图片',
      kindFile: '文件',
      stateDone: '完成',
      stateReceived: '已保存',
      stateFailed: '失败',
      stateSending: '发送中',
      stateOffered: '待接收',
      stateReceiving: '接收中'
    }),
    en: Object.freeze({
      documentTitle: 'Local Clipboard Console',
      languageSwitcher: 'Interface language',
      sendToDevice: 'Send to device',
      sendClipboard: 'Send clipboard',
      sendFiles: 'Send files',
      targetPlaceholder: 'Target device (leave empty for all devices)',
      hotkeyHint: 'Use the global hotkey to copy and send',
      pendingFiles: 'Pending files',
      receiveAll: 'Receive all',
      transferHistory: 'Transfer history',
      connected: 'Connected',
      reconnecting: 'Offline, reconnecting',
      consoleOffline: 'Console offline',
      clipboard: '(clipboard)',
      clipboardContent: '(clipboard content)',
      fromPeer: 'From {peer}',
      receive: 'Receive',
      retry: 'Retry',
      noActiveTransfers: 'No active transfers',
      noHistory: 'No transfer history',
      directionIn: 'In',
      directionOut: 'Out',
      kindText: 'Text',
      kindImage: 'Image',
      kindFile: 'File',
      stateDone: 'Done',
      stateReceived: 'Saved',
      stateFailed: 'Failed',
      stateSending: 'Sending',
      stateOffered: 'Pending',
      stateReceiving: 'Receiving'
    })
  });

  const normalizeLocale = value => String(value || '').toLowerCase().startsWith('zh') ? 'zh' : 'en';

  function storedLocale() {
    try {
      const value = localStorage.getItem(storageKey);
      return supportedLocales.has(value) ? value : '';
    } catch (_) {
      return '';
    }
  }

  function systemLocale() {
    const languages = Array.isArray(navigator.languages) && navigator.languages.length
      ? navigator.languages
      : [navigator.language];
    return normalizeLocale(languages[0]);
  }

  let locale = storedLocale() || systemLocale();
  const listeners = new Set();

  function t(key, values = {}) {
    let text = messages[locale][key] ?? messages.en[key] ?? key;
    return text.replace(/\{(\w+)\}/g, (match, name) => values[name] ?? match);
  }

  function apply(root = document) {
    document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en';
    document.title = t('documentTitle');
    root.querySelectorAll('[data-i18n]').forEach(element => {
      element.textContent = t(element.dataset.i18n);
    });
    root.querySelectorAll('[data-i18n-placeholder]').forEach(element => {
      element.setAttribute('placeholder', t(element.dataset.i18nPlaceholder));
    });
    root.querySelectorAll('[data-i18n-aria-label]').forEach(element => {
      element.setAttribute('aria-label', t(element.dataset.i18nAriaLabel));
    });
    root.querySelectorAll('[data-locale]').forEach(element => {
      const active = element.dataset.locale === locale;
      element.classList.toggle('active', active);
      element.setAttribute('aria-pressed', String(active));
    });
  }

  function setLocale(value) {
    const next = normalizeLocale(value);
    if (next === locale) return;
    locale = next;
    try {
      localStorage.setItem(storageKey, locale);
    } catch (_) {
      // The language still changes for this page when storage is unavailable.
    }
    apply();
    listeners.forEach(listener => listener(locale));
  }

  function onChange(listener) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }

  globalThis.LocalClipboardI18n = Object.freeze({
    apply,
    locale: () => locale,
    localeTag: () => locale === 'zh' ? 'zh-CN' : 'en',
    onChange,
    setLocale,
    t
  });
})();
