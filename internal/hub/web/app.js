(() => {
  'use strict';

  const $ = id => document.getElementById(id);
  const strings = {
    zh: {
      documentTitle:'Local Clipboard 手机收发', mobileHub:'NAS Hub · 手机收发', connectHub:'连接 NAS Hub',
      installTitle:'安装 Local Clipboard', installHelp:'添加到主屏幕，之后可像 App 一样直接打开。',
      installIOSHelp:'点击浏览器的分享按钮，然后选择“添加到主屏幕”。',
      installManualHelp:'打开浏览器菜单，选择“添加到主屏幕”或“安装应用”。', installAction:'安装', installGuide:'查看方法', installed:'已安装到设备',
      tokenHelp:'首次输入共享令牌；信任后 Hub 重启也无需再次输入。令牌不会写入 URL 或浏览器存储。', sharedToken:'共享令牌',
      tokenPlaceholder:'输入 LOCALCLIP_TOKEN', trustDevice:'信任此设备', connect:'连接', thisDevice:'此手机的设备名称', disconnect:'取消信任',
      connected:'已连接', onlineDevices:n => `${n} 台桌面设备在线`, send:'发送', receive:'接收', sendText:'发送文本',
      sendTextHelp:'粘贴内容并明确发送到目标设备的剪贴板。', targetDevice:'目标设备', allDevices:'所有在线设备',
      textContent:'文本内容', textPlaceholder:'在这里长按粘贴，或使用下方按钮', paste:'从剪贴板粘贴',
      sendTextAction:'发送文本', sendFiles:'发送文件或照片', sendFilesHelp:'文件上传到 Hub 后，接收设备会看到待接收项目。',
      chooseFiles:'选择照片或文件', fileLimit:'单个文件最大 1 GiB，Hub 保留 24 小时', uploadAndSend:'上传并发送',
      inbox:'接收箱', inboxHelp:'显示发给此设备或广播的最近内容。', refresh:'刷新', noInbox:'暂无可接收内容',
      copy:'复制', view:'查看', download:'下载', saveImage:'查看并保存', copied:'已复制到系统剪贴板',
      manualCopy:'浏览器未允许自动复制，请在文本框中长按复制。', pasteUnavailable:'请在文本框中长按并选择“粘贴”。',
      connecting:'正在连接…', authFailed:'令牌不正确或 Hub 无法访问', requestFailed:'请求失败', textEmpty:'请先粘贴文本',
      sending:'正在发送…', sent:n => n > 0 ? `已发送到 ${n} 台设备` : '已存入 Hub，目标设备当前不在线',
      fileSelected:n => `已选择 ${n} 个文件`, uploading:(i,n,name) => `正在上传 ${i}/${n}：${name}`,
      filesSent:(n,d) => `已发送 ${n} 个文件，共投递到 ${d} 台设备`, loading:'正在加载…',
      textItem:'文本内容', imageItem:'图片', fileItem:'文件', from:(source,time) => `${source} · ${time}`,
      mobileDefault:'我的手机', forgetting:'正在取消信任…', forgetFailed:'取消信任失败，请稍后重试',
      sessionEnded:'此设备已退出并取消信任', unknownError:'发生未知错误'
    },
    en: {
      documentTitle:'Local Clipboard Mobile', mobileHub:'NAS Hub · Mobile transfer', connectHub:'Connect to NAS Hub',
      installTitle:'Install Local Clipboard', installHelp:'Add it to your home screen and open it like an app.',
      installIOSHelp:'Use the browser Share menu, then choose Add to Home Screen.',
      installManualHelp:'Open the browser menu and choose Add to Home screen or Install app.', installAction:'Install', installGuide:'View steps', installed:'Installed on this device',
      tokenHelp:'Enter the shared token once. A trusted device stays signed in after Hub restarts; the token is never stored in the URL or browser.', sharedToken:'Shared token',
      tokenPlaceholder:'Enter LOCALCLIP_TOKEN', trustDevice:'Trust this device', connect:'Connect', thisDevice:'This phone’s device name', disconnect:'Forget device',
      connected:'Connected', onlineDevices:n => `${n} desktop device${n === 1 ? '' : 's'} online`, send:'Send', receive:'Receive',
      sendText:'Send text', sendTextHelp:'Paste content and explicitly send it to a target device’s clipboard.',
      targetDevice:'Target device', allDevices:'All online devices', textContent:'Text',
      textPlaceholder:'Long-press to paste here, or use the button below', paste:'Paste from clipboard', sendTextAction:'Send text',
      sendFiles:'Send files or photos', sendFilesHelp:'Files are uploaded to the Hub and appear as pending items on receiving devices.',
      chooseFiles:'Choose photos or files', fileLimit:'Up to 1 GiB per file; retained by the Hub for 24 hours', uploadAndSend:'Upload and send',
      inbox:'Inbox', inboxHelp:'Recent items sent to this device or broadcast by another device.', refresh:'Refresh', noInbox:'Nothing to receive yet',
      copy:'Copy', view:'View', download:'Download', saveImage:'View and save', copied:'Copied to the system clipboard',
      manualCopy:'Clipboard access was not granted. Long-press the text field to copy manually.',
      pasteUnavailable:'Long-press the text field and choose Paste.', connecting:'Connecting…', authFailed:'Incorrect token or Hub unavailable',
      requestFailed:'Request failed', textEmpty:'Paste some text first', sending:'Sending…',
      sent:n => n > 0 ? `Sent to ${n} device${n === 1 ? '' : 's'}` : 'Saved to the Hub; the target is currently offline',
      fileSelected:n => `${n} file${n === 1 ? '' : 's'} selected`, uploading:(i,n,name) => `Uploading ${i}/${n}: ${name}`,
      filesSent:(n,d) => `Sent ${n} file${n === 1 ? '' : 's'}; delivered to ${d} device${d === 1 ? '' : 's'}`,
      loading:'Loading…', textItem:'Text', imageItem:'Image', fileItem:'File', from:(source,time) => `${source} · ${time}`,
      mobileDefault:'My phone', forgetting:'Forgetting device…', forgetFailed:'Could not forget this device. Try again.',
      sessionEnded:'This device was signed out and forgotten', unknownError:'An unknown error occurred'
    }
  };

  const state = {
    locale: preferredLocale(), authenticated:false, online:[], files:[], items:[], poller:null, dialogURL:'', installPrompt:null
  };

  function preferredLocale() {
    const saved = localStorage.getItem('localclip-mobile-locale');
    if (saved === 'zh' || saved === 'en') return saved;
    return (navigator.languages || [navigator.language || 'en']).some(x => String(x).toLowerCase().startsWith('zh')) ? 'zh' : 'en';
  }

  function t(key, ...args) {
    const value = strings[state.locale][key];
    return typeof value === 'function' ? value(...args) : (value || key);
  }

  function applyLocale() {
    document.documentElement.lang = state.locale === 'zh' ? 'zh-CN' : 'en';
    document.title = t('documentTitle');
    document.querySelectorAll('[data-i18n]').forEach(el => { el.textContent = t(el.dataset.i18n); });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => { el.placeholder = t(el.dataset.i18nPlaceholder); });
    document.querySelectorAll('[data-locale]').forEach(el => el.classList.toggle('active', el.dataset.locale === state.locale));
    if (!localStorage.getItem('localclip-mobile-device')) $('deviceName').value = t('mobileDefault');
    renderTargets();
    renderConnection();
    renderFiles();
    renderInbox();
    renderInstall();
  }

  function standaloneMode() {
    return window.matchMedia('(display-mode: standalone)').matches || navigator.standalone === true;
  }

  function isiOS() {
    return /iphone|ipad|ipod/i.test(navigator.userAgent);
  }

  function isMobileBrowser() {
    return /android|iphone|ipad|ipod/i.test(navigator.userAgent);
  }

  function renderInstall() {
    const available = !standaloneMode() && (state.installPrompt || isMobileBrowser());
    $('installBanner').hidden = !available;
    if (!available) return;
    $('installHelp').textContent = state.installPrompt ? t('installHelp') : (isiOS() ? t('installIOSHelp') : t('installManualHelp'));
    $('installBtn').textContent = state.installPrompt ? t('installAction') : t('installGuide');
  }

  async function installApp() {
    if (!state.installPrompt) {
      toast(isiOS() ? t('installIOSHelp') : t('installManualHelp'));
      return;
    }
    const prompt = state.installPrompt;
    state.installPrompt = null;
    let accepted = false;
    try {
      await prompt.prompt();
      const choice = await prompt.userChoice;
      accepted = choice.outcome === 'accepted';
    } catch (_) {
      toast(t('installManualHelp'));
    }
    if (accepted) {
      $('installBanner').hidden = true;
    } else {
      renderInstall();
    }
  }

  async function api(url, options = {}) {
    const response = await fetch(url, {credentials:'same-origin', ...options});
    if (response.status === 401) {
      showAuth();
      throw new Error(t('authFailed'));
    }
    if (response.status === 204) return null;
    const type = response.headers.get('content-type') || '';
    const payload = type.includes('application/json') ? await response.json() : await response.text();
    if (!response.ok) {
      const message = typeof payload === 'string' ? payload.trim() : payload.error;
      throw new Error(message || `${t('requestFailed')} (${response.status})`);
    }
    return payload;
  }

  function jsonOptions(body, method = 'POST') {
    return {method, headers:{'Content-Type':'application/json'}, body:JSON.stringify(body)};
  }

  function currentDevice() {
    return $('deviceName').value.trim() || t('mobileDefault');
  }

  function showAuth() {
    state.authenticated = false;
    $('authCard').hidden = false;
    $('app').hidden = true;
    if (state.poller) clearInterval(state.poller);
    state.poller = null;
  }

  function showApp() {
    state.authenticated = true;
    $('authCard').hidden = true;
    $('app').hidden = false;
    if (!state.poller) state.poller = setInterval(() => { if (!document.hidden) refreshAll(false); }, 5000);
  }

  async function refreshStatus() {
    const status = await api('/api/status');
    state.online = Array.isArray(status.online) ? status.online.slice().sort((a,b) => a.localeCompare(b)) : [];
    showApp();
    renderTargets();
    renderConnection();
  }

  async function refreshInbox(report = false) {
    if (!state.authenticated) return;
    if (report) setStatus('inboxStatus', t('loading'));
    try {
      state.items = await api(`/api/inbox?device=${encodeURIComponent(currentDevice())}`);
      renderInbox();
      setStatus('inboxStatus', '');
    } catch (error) {
      if (state.authenticated) setStatus('inboxStatus', error.message, 'error');
    }
  }

  async function refreshAll(report = false) {
    try {
      await refreshStatus();
      await refreshInbox(report);
    } catch (error) {
      if (state.authenticated && report) toast(error.message);
    }
  }

  function renderConnection() {
    if (!$('hubState')) return;
    $('hubDot').className = `dot${state.authenticated ? ' up' : ''}`;
    $('hubState').textContent = state.authenticated ? t('connected') : t('connecting');
    $('onlineSummary').textContent = t('onlineDevices', state.online.length);
  }

  function renderTargets() {
    for (const id of ['textTarget','fileTarget']) {
      const select = $(id);
      if (!select) continue;
      const previous = select.value;
      select.replaceChildren();
      select.add(new Option(t('allDevices'), ''));
      for (const name of state.online) {
        if (name !== currentDevice()) select.add(new Option(name, name));
      }
      if ([...select.options].some(option => option.value === previous)) select.value = previous;
    }
  }

  function setStatus(id, message, kind = '') {
    const el = $(id);
    el.textContent = message;
    el.className = `form-status${kind ? ` ${kind}` : ''}`;
  }

  function deliveryStatus(result) {
    return t('sent', Number(result.delivered || 0));
  }

  async function sendText() {
    const text = $('textInput').value;
    if (!text.trim()) {
      setStatus('textStatus', t('textEmpty'), 'error');
      return;
    }
    $('sendTextBtn').disabled = true;
    setStatus('textStatus', t('sending'));
    try {
      const result = await api('/api/send-text', jsonOptions({text, source:currentDevice(), target:$('textTarget').value}));
      $('textInput').value = '';
      setStatus('textStatus', deliveryStatus(result), 'success');
      await refreshInbox(false);
    } catch (error) {
      setStatus('textStatus', error.message, 'error');
    } finally {
      $('sendTextBtn').disabled = false;
    }
  }

  async function pasteText() {
    try {
      if (!navigator.clipboard?.readText) throw new Error('unavailable');
      $('textInput').value = await navigator.clipboard.readText();
      $('textInput').focus();
      setStatus('textStatus', '');
    } catch (_) {
      $('textInput').focus();
      setStatus('textStatus', t('pasteUnavailable'));
    }
  }

  function setFiles(files) {
    state.files = Array.from(files || []);
    renderFiles();
  }

  function renderFiles() {
    const list = $('selectedFiles');
    if (!list) return;
    list.replaceChildren();
    for (const file of state.files) {
      const row = document.createElement('div');
      row.className = 'selected-file';
      const name = document.createElement('span');
      name.textContent = file.name;
      const size = document.createElement('small');
      size.textContent = formatSize(file.size);
      row.append(name, size);
      list.append(row);
    }
    $('sendFilesBtn').disabled = state.files.length === 0;
    setStatus('fileStatus', state.files.length ? t('fileSelected', state.files.length) : '');
  }

  async function sendFiles() {
    if (!state.files.length) return;
    const files = state.files.slice();
    $('sendFilesBtn').disabled = true;
    $('uploadProgress').hidden = false;
    let delivered = 0;
    try {
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        setStatus('fileStatus', t('uploading', i + 1, files.length, file.name));
        $('uploadProgress').firstElementChild.style.width = `${Math.round(i * 100 / files.length)}%`;
        const upload = await api(`/upload?name=${encodeURIComponent(file.name)}`, {
          method:'POST', headers:{'Content-Type':file.type || 'application/octet-stream'}, body:file
        });
        const result = await api('/api/send-file', jsonOptions({
          fid:upload.id, name:file.name, source:currentDevice(), target:$('fileTarget').value
        }));
        delivered += Number(result.delivered || 0);
        $('uploadProgress').firstElementChild.style.width = `${Math.round((i + 1) * 100 / files.length)}%`;
      }
      setFiles([]);
      $('fileInput').value = '';
      setStatus('fileStatus', t('filesSent', files.length, delivered), 'success');
      await refreshInbox(false);
    } catch (error) {
      setStatus('fileStatus', error.message, 'error');
    } finally {
      $('sendFilesBtn').disabled = state.files.length === 0;
      setTimeout(() => { $('uploadProgress').hidden = true; }, 500);
    }
  }

  function renderInbox() {
    const list = $('inbox');
    if (!list) return;
    $('inboxCount').textContent = String(state.items.length);
    list.replaceChildren();
    if (!state.items.length) {
      const empty = document.createElement('div');
      empty.className = 'empty';
      empty.textContent = t('noInbox');
      list.append(empty);
      return;
    }
    for (const item of state.items) list.append(inboxRow(item));
  }

  function inboxRow(item) {
    const row = document.createElement('article');
    row.className = 'inbox-item';
    const icon = document.createElement('span');
    icon.className = 'kind-icon';
    icon.textContent = item.type === 'text' ? 'T' : item.type === 'image' ? '▧' : '↓';
    const main = document.createElement('div');
    main.className = 'item-main';
    const title = document.createElement('div');
    title.className = 'item-title';
    title.textContent = item.name || t(item.type === 'text' ? 'textItem' : item.type === 'image' ? 'imageItem' : 'fileItem');
    const meta = document.createElement('div');
    meta.className = 'item-meta';
    meta.textContent = `${t('from', item.source || '?', formatTime(item.ts))}${item.size ? ` · ${formatSize(item.size)}` : ''}`;
    main.append(title, meta);
    if (item.preview) {
      const preview = document.createElement('div');
      preview.className = 'item-preview';
      preview.textContent = item.preview;
      main.append(preview);
    }
    row.append(icon, main, itemAction(item));
    return row;
  }

  function itemAction(item) {
    if (item.type === 'file') {
      const link = document.createElement('a');
      link.className = 'button item-action';
      link.textContent = t('download');
      link.href = `/download?id=${encodeURIComponent(item.fid)}`;
      link.download = item.name || '';
      return link;
    }
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'item-action';
    button.textContent = t(item.type === 'text' ? 'copy' : 'saveImage');
    button.addEventListener('click', () => openItem(item));
    return button;
  }

  async function loadItem(id) {
    return api(`/api/item?id=${encodeURIComponent(id)}&device=${encodeURIComponent(currentDevice())}`);
  }

  async function openItem(item) {
    try {
      const full = await loadItem(item.id);
      if (full.type === 'text') {
        try {
          if (!navigator.clipboard?.writeText) throw new Error('unavailable');
          await navigator.clipboard.writeText(full.content);
          toast(t('copied'));
        } catch (_) {
          showTextDialog(full.content);
        }
        return;
      }
      if (full.type === 'image') showImageDialog(full);
    } catch (error) {
      toast(error.message);
    }
  }

  function resetDialog() {
    if (state.dialogURL) URL.revokeObjectURL(state.dialogURL);
    state.dialogURL = '';
    $('dialogText').hidden = true;
    $('dialogImage').hidden = true;
    $('dialogCopy').hidden = true;
    $('dialogDownload').hidden = true;
    $('dialogDownload').removeAttribute('href');
  }

  function showTextDialog(text) {
    resetDialog();
    $('dialogTitle').textContent = t('textItem');
    $('dialogText').value = text;
    $('dialogText').hidden = false;
    $('dialogCopy').hidden = false;
    openDialog();
    $('dialogText').focus();
    $('dialogText').select();
    toast(t('manualCopy'));
  }

  function showImageDialog(item) {
    resetDialog();
    $('dialogTitle').textContent = item.name || t('imageItem');
    const binary = atob(item.content);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    state.dialogURL = URL.createObjectURL(new Blob([bytes], {type:'image/png'}));
    $('dialogImage').src = state.dialogURL;
    $('dialogImage').hidden = false;
    $('dialogDownload').href = state.dialogURL;
    $('dialogDownload').download = item.name || 'clipboard-image.png';
    $('dialogDownload').hidden = false;
    openDialog();
  }

  function openDialog() {
    if (typeof $('itemDialog').showModal === 'function') $('itemDialog').showModal();
    else $('itemDialog').setAttribute('open', '');
  }

  async function copyDialogText() {
    try {
      await navigator.clipboard.writeText($('dialogText').value);
      toast(t('copied'));
      $('itemDialog').close();
    } catch (_) {
      $('dialogText').focus();
      $('dialogText').select();
      toast(t('manualCopy'));
    }
  }

  function formatSize(bytes) {
    if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`;
    if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MB`;
    if (bytes >= 1 << 10) return `${Math.round(bytes / (1 << 10))} KB`;
    return `${bytes} B`;
  }

  function formatTime(value) {
    return new Date(value).toLocaleString(state.locale === 'zh' ? 'zh-CN' : 'en-US', {month:'short',day:'numeric',hour:'2-digit',minute:'2-digit'});
  }

  let toastTimer = 0;
  function toast(message) {
    clearTimeout(toastTimer);
    $('toast').textContent = message;
    $('toast').hidden = false;
    toastTimer = setTimeout(() => { $('toast').hidden = true; }, 2800);
  }

  function activateTab(name) {
    document.querySelectorAll('[data-tab]').forEach(button => button.classList.toggle('active', button.dataset.tab === name));
    $('sendPanel').hidden = name !== 'send';
    $('receivePanel').hidden = name !== 'receive';
    if (name === 'receive') refreshInbox(true);
  }

  function bindEvents() {
    document.querySelectorAll('[data-locale]').forEach(button => button.addEventListener('click', () => {
      state.locale = button.dataset.locale;
      localStorage.setItem('localclip-mobile-locale', state.locale);
      applyLocale();
    }));
    document.querySelectorAll('[data-tab]').forEach(button => button.addEventListener('click', () => activateTab(button.dataset.tab)));
    $('authForm').addEventListener('submit', async event => {
      event.preventDefault();
      setStatus('authStatus', t('connecting'));
      try {
        const response = await fetch('/api/session', jsonOptions({
          token:$('tokenInput').value,
          trust:$('trustDevice').checked,
          device:currentDevice()
        }));
        if (!response.ok) throw new Error(t('authFailed'));
        $('tokenInput').value = '';
        setStatus('authStatus', '');
        await refreshAll(true);
      } catch (error) { setStatus('authStatus', error.message, 'error'); }
    });
    $('installBtn').addEventListener('click', installApp);
    $('logoutBtn').addEventListener('click', async () => {
      const button = $('logoutBtn');
      button.disabled = true;
      button.textContent = t('forgetting');
      try {
        const response = await fetch('/api/session', {method:'DELETE', credentials:'same-origin'});
        if (!response.ok) throw new Error(t('forgetFailed'));
        showAuth();
        setStatus('authStatus', t('sessionEnded'));
      } catch (error) {
        toast(error.message || t('forgetFailed'));
      } finally {
        button.disabled = false;
        button.textContent = t('disconnect');
      }
    });
    $('deviceName').addEventListener('change', () => {
      $('deviceName').value = currentDevice();
      localStorage.setItem('localclip-mobile-device', currentDevice());
      renderTargets();
      refreshInbox(false);
    });
    $('pasteBtn').addEventListener('click', pasteText);
    $('sendTextBtn').addEventListener('click', sendText);
    $('fileInput').addEventListener('change', event => setFiles(event.target.files));
    $('sendFilesBtn').addEventListener('click', sendFiles);
    $('refreshBtn').addEventListener('click', () => refreshAll(true));
    $('closeDialog').addEventListener('click', () => $('itemDialog').close());
    $('dialogCopy').addEventListener('click', copyDialogText);
    $('itemDialog').addEventListener('close', resetDialog);
    for (const eventName of ['dragenter','dragover']) $('dropZone').addEventListener(eventName, event => {
      event.preventDefault(); $('dropZone').classList.add('drag');
    });
    for (const eventName of ['dragleave','drop']) $('dropZone').addEventListener(eventName, event => {
      event.preventDefault(); $('dropZone').classList.remove('drag');
    });
    $('dropZone').addEventListener('drop', event => setFiles(event.dataTransfer.files));
    document.addEventListener('visibilitychange', () => { if (!document.hidden && state.authenticated) refreshAll(false); });
  }

  async function start() {
    $('deviceName').value = localStorage.getItem('localclip-mobile-device') || t('mobileDefault');
    bindEvents();
    applyLocale();
    try {
      await refreshAll(false);
    } catch (_) {
      showAuth();
    }
  }

  window.addEventListener('beforeinstallprompt', event => {
    event.preventDefault();
    state.installPrompt = event;
    renderInstall();
  });
  window.addEventListener('appinstalled', () => {
    state.installPrompt = null;
    $('installBanner').hidden = true;
    toast(t('installed'));
  });

  start();
})();
