// sing-box-helper UI — vanilla JS, no frameworks.
'use strict';

// --- DOM ---------------------------------------------------------------
const $ = (id) => document.getElementById(id);
const statusPill   = $('status-pill');
const stSb         = $('st-sb');
const stTun        = $('st-tun');
const stNode       = $('st-node');
const uriInput     = $('uri-input');
const btnPreview   = $('btn-preview');
const btnApply     = $('btn-apply');
const btnTest      = $('btn-test');
const btnRefresh   = $('btn-refresh');
const btnToggle    = $('btn-toggle-config');
const busy         = $('busy');

const errorCard    = $('error-card');
const resultError  = $('result-error');

const displayCard  = $('display-card');
const resultDisplay = $('result-display');
const resultNotes  = $('result-notes');
const resultConfig = $('result-config');

const applyCard    = $('apply-card');
const applyMeta    = $('apply-meta');

const testCard     = $('test-card');
const testSteps    = $('test-steps');

const ALL_BUTTONS = [btnPreview, btnApply, btnTest, btnRefresh];

// --- helpers -----------------------------------------------------------

function setBusy(on) {
  busy.classList.toggle('hidden', !on);
  for (const b of ALL_BUTTONS) b.disabled = on;
}

function clearError() {
  errorCard.classList.add('hidden');
  resultError.textContent = '';
}

function showError(msg) {
  resultError.textContent = msg;
  errorCard.classList.remove('hidden');
}

function clearResults() {
  clearError();
  displayCard.classList.add('hidden');
  applyCard.classList.add('hidden');
  testCard.classList.add('hidden');
  resultConfig.classList.add('hidden');
  btnToggle.textContent = 'Показать готовый JSON ▾';
}

async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  if (!res.ok) {
    const msg = (body && body.error) ? body.error : `HTTP ${res.status}: ${text}`;
    throw new Error(msg);
  }
  return body;
}

function row(label, value) {
  const tr = document.createElement('tr');
  const th = document.createElement('th');
  th.textContent = label;
  const td = document.createElement('td');
  if (typeof value === 'string' || typeof value === 'number') {
    td.textContent = String(value);
  } else if (value && value.nodeType) {
    td.appendChild(value);
  } else {
    td.textContent = '—';
  }
  tr.appendChild(th);
  tr.appendChild(td);
  return tr;
}

// --- status (auto-refreshing) -----------------------------------------

function fmtDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function renderCurrentNode(n) {
  if (!n) {
    stNode.textContent = 'не задан — нажмите «Применить»';
    return;
  }
  // Always show what's actually in config.json (the truth).
  const summary = `${n.protocol}  ${n.server}:${n.port}`;

  // Wrap in a small <div> so we can stack secondary lines without
  // restructuring the parent table.
  stNode.innerHTML = '';
  const wrap = document.createElement('div');

  if (n.managed) {
    // Big readable label as the primary identifier, with the
    // protocol/server/port as a smaller subtitle.
    const label = document.createElement('div');
    label.textContent = n.label || '(без метки)';
    label.style.fontWeight = '600';
    wrap.appendChild(label);

    const sub = document.createElement('div');
    sub.className = 'muted';
    sub.textContent = summary + (n.applied_at ? `   ·   применён ${fmtDate(n.applied_at)}` : '');
    sub.style.marginTop = '2px';
    wrap.appendChild(sub);

    if (n.uri) {
      const uriWrap = document.createElement('div');
      uriWrap.style.marginTop = '6px';

      const toggle = document.createElement('button');
      toggle.className = 'btn-link';
      toggle.textContent = 'Показать URI';
      const pre = document.createElement('pre');
      pre.className = 'hidden';
      pre.textContent = n.uri;
      pre.style.marginTop = '4px';
      toggle.addEventListener('click', () => {
        const hidden = pre.classList.toggle('hidden');
        toggle.textContent = hidden ? 'Показать URI' : 'Скрыть URI';
      });
      uriWrap.appendChild(toggle);
      uriWrap.appendChild(pre);
      wrap.appendChild(uriWrap);
    }
  } else {
    // sing-box is running with a config that wasn't applied through
    // this utility (no state.json, or it disagrees with config.json).
    const main = document.createElement('div');
    main.textContent = summary;
    wrap.appendChild(main);

    const tag = document.createElement('div');
    tag.className = 'muted';
    tag.textContent = 'конфиг не управляется через эту утилиту (нет метаданных URI)';
    tag.style.marginTop = '2px';
    wrap.appendChild(tag);
  }

  stNode.appendChild(wrap);
}

async function refreshStatus() {
  try {
    const s = await api('/api/status');
    stSb.textContent = s.sing_box_running
      ? `running, PID ${s.sing_box_pid || '?'} — ${s.sing_box_version || ''}`
      : 'не запущен';
    stTun.textContent = s.tun_up
      ? `UP (${s.tun_name})`
      : `down (${s.tun_name})`;
    renderCurrentNode(s.current_node);

    if (s.sing_box_running && s.tun_up) {
      statusPill.className = 'pill pill-green';
      statusPill.textContent = '● online';
    } else if (s.sing_box_running) {
      statusPill.className = 'pill pill-yellow';
      statusPill.textContent = '● degraded';
    } else {
      statusPill.className = 'pill pill-red';
      statusPill.textContent = '● offline';
    }
  } catch (e) {
    statusPill.className = 'pill pill-red';
    statusPill.textContent = '● API недоступен';
    stSb.textContent = stTun.textContent = '—';
    stNode.textContent = '—';
  }
}

// --- render Display + notes + config -----------------------------------

function renderDisplay(d, label) {
  resultDisplay.innerHTML = '';
  if (label) resultDisplay.appendChild(row('Метка (#name)', label));
  resultDisplay.appendChild(row('Протокол', d.protocol));
  resultDisplay.appendChild(row('Сервер', d.server));
  resultDisplay.appendChild(row('Порт', d.port));
  if (d.sni) resultDisplay.appendChild(row('SNI', d.sni));
  resultDisplay.appendChild(row('TLS verify', d.tls_verify ? '✓ включён' : '✗ выключен'));
  resultDisplay.appendChild(row('Транспорт', d.transport));

  resultNotes.innerHTML = '';
  if (d.notes && d.notes.length) {
    for (const n of d.notes) {
      const div = document.createElement('div');
      div.className = 'note';
      div.textContent = '⚠ ' + n;
      resultNotes.appendChild(div);
    }
  }
  displayCard.classList.remove('hidden');
}

function setConfigJSON(jsonStr) {
  resultConfig.textContent = jsonStr;
}

btnToggle.addEventListener('click', () => {
  const isHidden = resultConfig.classList.toggle('hidden');
  btnToggle.textContent = isHidden ? 'Показать готовый JSON ▾' : 'Скрыть JSON ▴';
});

// --- render test steps -------------------------------------------------

function renderTestSteps(steps) {
  testSteps.innerHTML = '';
  for (const s of steps) {
    const li = document.createElement('li');
    const icon = document.createElement('span');
    icon.className = 'step-icon ' + s.status;
    icon.textContent = s.status === 'ok' ? '✓' : s.status === 'fail' ? '✗' : '•';
    const name = document.createElement('span');
    name.className = 'step-name';
    name.textContent = s.name;
    const detail = document.createElement('span');
    detail.className = 'step-detail';
    detail.textContent = s.detail || '';
    li.appendChild(icon);
    li.appendChild(name);
    li.appendChild(detail);
    testSteps.appendChild(li);
  }
  testCard.classList.remove('hidden');
}

// --- handlers ---------------------------------------------------------

function readURI() {
  const v = (uriInput.value || '').trim();
  if (!v) throw new Error('Вставьте ссылку на узел в поле выше.');
  return v;
}

async function doPreview() {
  clearResults();
  setBusy(true);
  try {
    const uri = readURI();
    const r = await api('/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ uri }),
    });
    renderDisplay(r.display, r.label);
    setConfigJSON(r.config);
  } catch (e) {
    showError(e.message);
  } finally {
    setBusy(false);
  }
}

async function doApply() {
  clearResults();
  setBusy(true);
  try {
    const uri = readURI();
    const r = await api('/api/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ uri }),
    });
    renderDisplay(r.display, r.label);
    // setConfigJSON not called — apply doesn't return config, but display+meta is enough

    applyMeta.innerHTML = '';
    applyMeta.appendChild(row('Бэкап',
      r.backup_path ? r.backup_path : '— (старого файла не было)'));
    applyMeta.appendChild(row('Размер записанного файла', r.config_size + ' байт'));
    if (r.reach) {
      applyMeta.appendChild(row('Доступность узла (pre-check)',
        r.reach.ok
          ? `✓ ${r.reach.network.toUpperCase()} OK`
          : `✗ ${r.reach.network.toUpperCase()} — ${r.reach.error || 'failed'}`));
    }
    applyMeta.appendChild(row('Restart sing-box',
      r.restarted ? '✓ выполнен' : ('✗ ошибка: ' + (r.restart_error || 'unknown'))));
    applyCard.classList.remove('hidden');

    // refresh status + auto-run /api/test after apply
    await refreshStatus();
    await runTest(/*alreadyBusy=*/true);
  } catch (e) {
    showError(e.message);
  } finally {
    setBusy(false);
  }
}

async function runTest(alreadyBusy) {
  if (!alreadyBusy) {
    clearResults();
    setBusy(true);
  }
  try {
    const r = await api('/api/test');
    renderTestSteps(r.steps);
  } catch (e) {
    showError(e.message);
  } finally {
    if (!alreadyBusy) setBusy(false);
  }
}

btnPreview.addEventListener('click', doPreview);
btnApply.addEventListener('click', doApply);
btnTest.addEventListener('click', () => runTest(false));
btnRefresh.addEventListener('click', refreshStatus);

// --- tabs --------------------------------------------------------------

const tabs = Array.from(document.querySelectorAll('nav.tabs .tab'));
const panes = {
  home:     $('tab-home'),
  settings: $('tab-settings'),
  logs:     $('tab-logs'),
  backups:  $('tab-backups'),
};

function activateTab(name) {
  for (const t of tabs) t.classList.toggle('active', t.dataset.tab === name);
  for (const k of Object.keys(panes)) panes[k].classList.toggle('hidden', k !== name);
  if (name === 'settings') loadSettings();
  if (name === 'logs')     loadLogs();
  else                     setLogsAutoRefresh(false);
  if (name === 'backups')  loadBackups();
}

for (const t of tabs) {
  t.addEventListener('click', () => activateTab(t.dataset.tab));
}

// --- settings ----------------------------------------------------------

const $settings = {
  mixedModeRadios:  () => Array.from(document.querySelectorAll('input[name="mixed_mode"]')),
  mixedCustom:      () => $('settings-mixed-listen-custom'),
  mixedPort:        () => $('settings-mixed-port'),
  enableMixed:      () => $('settings-enable-mixed'),
  tunIface:         () => $('settings-tun-iface'),
  tunAddr:          () => $('settings-tun-addr'),
  tunMtu:           () => $('settings-tun-mtu'),
  tunStack:         () => $('settings-tun-stack'),
  dnsUpstream:      () => $('settings-dns-upstream'),
  dnsStrategy:      () => $('settings-dns-strategy'),
  enableClash:      () => $('settings-enable-clash'),
  clashListen:      () => $('settings-clash-listen'),
  clashUiDir:       () => $('settings-clash-uidir'),
  logLevel:         () => $('settings-log-level'),
  logTs:            () => $('settings-log-ts'),
  status:           () => $('settings-status'),
  busy:             () => $('settings-busy'),
  autoResolved:     () => $('settings-auto-resolved'),
};

function mixedListenFromForm() {
  const r = $settings.mixedModeRadios().find(x => x.checked);
  if (!r) return 'auto';
  if (r.value === 'custom') return ($settings.mixedCustom().value || '').trim();
  return r.value;
}

function setMixedListenInForm(value) {
  const known = ['auto', '127.0.0.1', '0.0.0.0'];
  const radios = $settings.mixedModeRadios();
  if (known.includes(value)) {
    for (const r of radios) r.checked = (r.value === value);
    $settings.mixedCustom().value = '';
  } else {
    for (const r of radios) r.checked = (r.value === 'custom');
    $settings.mixedCustom().value = value || '';
  }
}

function settingsFromForm() {
  return {
    mixed_listen:        mixedListenFromForm(),
    mixed_listen_port:   Number($settings.mixedPort().value) || 0,
    enable_mixed:        $settings.enableMixed().checked,
    tun_interface_name:  $settings.tunIface().value.trim(),
    tun_address:         $settings.tunAddr().value.trim(),
    tun_mtu:             Number($settings.tunMtu().value) || 0,
    tun_stack:           $settings.tunStack().value,
    upstream_dns:        $settings.dnsUpstream().value.trim(),
    dns_strategy:        $settings.dnsStrategy().value,
    enable_clash_api:    $settings.enableClash().checked,
    clash_api_listen:    $settings.clashListen().value.trim(),
    clash_api_ui_dir:    $settings.clashUiDir().value.trim(),
    log_level:           $settings.logLevel().value,
    log_timestamp:       $settings.logTs().checked,
  };
}

function settingsToForm(s, effective, isAuto) {
  setMixedListenInForm(s.mixed_listen);
  $settings.mixedPort().value     = s.mixed_listen_port;
  $settings.enableMixed().checked = !!s.enable_mixed;
  $settings.tunIface().value      = s.tun_interface_name || '';
  $settings.tunAddr().value       = s.tun_address || '';
  $settings.tunMtu().value        = s.tun_mtu;
  $settings.tunStack().value      = s.tun_stack || 'gvisor';
  $settings.dnsUpstream().value   = s.upstream_dns || '';
  $settings.dnsStrategy().value   = s.dns_strategy || 'ipv4_only';
  $settings.enableClash().checked = !!s.enable_clash_api;
  $settings.clashListen().value   = s.clash_api_listen || '';
  $settings.clashUiDir().value    = s.clash_api_ui_dir || '';
  $settings.logLevel().value      = s.log_level || 'info';
  $settings.logTs().checked       = !!s.log_timestamp;
  $settings.autoResolved().textContent =
    isAuto ? `(сейчас → ${effective})` : '';
}

async function loadSettings() {
  $settings.status().textContent = '';
  try {
    const r = await api('/api/settings');
    settingsToForm(r.settings, r.mixed_listen_effective, r.mixed_listen_auto);
  } catch (e) {
    $settings.status().textContent = 'не удалось загрузить: ' + e.message;
    $settings.status().style.color = 'var(--err)';
  }
}

async function saveSettings() {
  const s = settingsFromForm();
  $settings.busy().classList.remove('hidden');
  $settings.status().textContent = '';
  try {
    const r = await api('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(s),
    });
    settingsToForm(r.settings, r.mixed_listen_effective, r.mixed_listen_auto);
    const msg = [];
    msg.push('сохранено');
    if (r.re_rendered) msg.push('config.json пересобран');
    if (r.restarted)   msg.push('sing-box перезапущен');
    if (r.restart_error) msg.push('предупреждение: ' + r.restart_error);
    $settings.status().textContent = msg.join('; ');
    $settings.status().style.color = r.restart_error ? 'var(--warn)' : 'var(--ok)';
    // status pill may flip after restart
    await refreshStatus();
  } catch (e) {
    $settings.status().textContent = 'ошибка: ' + e.message;
    $settings.status().style.color = 'var(--err)';
  } finally {
    $settings.busy().classList.add('hidden');
  }
}

$('btn-settings-save').addEventListener('click', saveSettings);
$('btn-settings-reload').addEventListener('click', loadSettings);

// --- logs --------------------------------------------------------------

let logsTimer = null;

async function loadLogs() {
  const source = $('logs-source').value;
  const lines  = $('logs-lines').value;
  const out    = $('logs-out');
  const note   = $('logs-note');
  try {
    const r = await api(`/api/logs?source=${encodeURIComponent(source)}&lines=${encodeURIComponent(lines)}`);
    if (r.note) {
      note.textContent = '⚠ ' + r.note;
      note.classList.remove('hidden');
    } else {
      note.classList.add('hidden');
    }
    if (!r.lines || r.lines.length === 0) {
      out.textContent = '— нет строк —';
    } else {
      out.textContent = r.lines.join('\n');
      // autoscroll to bottom for fresh tails
      out.scrollTop = out.scrollHeight;
    }
  } catch (e) {
    note.textContent = '⚠ ошибка: ' + e.message;
    note.classList.remove('hidden');
  }
}

function setLogsAutoRefresh(on) {
  if (logsTimer) { clearInterval(logsTimer); logsTimer = null; }
  if (on) logsTimer = setInterval(loadLogs, 3000);
}

$('btn-logs-refresh').addEventListener('click', loadLogs);
$('logs-source').addEventListener('change', loadLogs);
$('logs-lines').addEventListener('change', loadLogs);
$('logs-autorefresh').addEventListener('change', (e) => setLogsAutoRefresh(e.target.checked));

// --- backups -----------------------------------------------------------

function fmtBackupTime(iso) {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function setBackupsStatus(msg, kind) {
  const el = $('backups-status');
  el.textContent = msg || '';
  el.style.color = kind === 'err' ? 'var(--err)'
                 : kind === 'ok'  ? 'var(--ok)'
                 :                  'var(--muted)';
}

async function loadBackups() {
  const ul = $('backups-list');
  ul.innerHTML = '<li class="muted">— загрузка —</li>';
  try {
    const r = await api('/api/backups');
    $('backups-keep').textContent = r.keep || 10;
    ul.innerHTML = '';
    if (!r.backups || r.backups.length === 0) {
      ul.innerHTML = '<li class="muted">— бэкапов нет —</li>';
      return;
    }
    for (const b of r.backups) ul.appendChild(renderBackupRow(b));
  } catch (e) {
    ul.innerHTML = '';
    setBackupsStatus('ошибка загрузки: ' + e.message, 'err');
  }
}

function renderBackupRow(b) {
  const li = document.createElement('li');
  li.className = 'backup-row';

  const info = document.createElement('div');
  info.className = 'info';
  const when = document.createElement('div');
  when.className = 'when';
  when.textContent = fmtBackupTime(b.created_at);
  info.appendChild(when);
  const meta = document.createElement('div');
  meta.className = 'meta';
  meta.textContent = `${b.summary || '(не удалось распознать)'} · ${b.size} байт · ${b.name}`;
  info.appendChild(meta);
  li.appendChild(info);

  const actions = document.createElement('div');
  actions.className = 'actions';

  const btnRestore = document.createElement('button');
  btnRestore.textContent = 'Откатиться';
  btnRestore.className = 'primary';
  btnRestore.addEventListener('click', () => restoreBackup(b.file));
  actions.appendChild(btnRestore);

  const btnDelete = document.createElement('button');
  btnDelete.textContent = '×';
  btnDelete.title = 'Удалить';
  btnDelete.addEventListener('click', () => deleteBackup(b.file));
  actions.appendChild(btnDelete);

  li.appendChild(actions);
  return li;
}

async function restoreBackup(file) {
  if (!confirm('Откатить config.json к этому бэкапу? Текущий конфиг будет сохранён как новый бэкап.')) return;
  setBackupsStatus('откатываем…', '');
  try {
    const r = await api('/api/backups/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ file }),
    });
    const parts = ['восстановлено'];
    if (r.backup_of_previous) parts.push('предыдущий сохранён как ' + r.backup_of_previous.split(/[\\\/]/).pop());
    if (r.restarted)          parts.push('sing-box перезапущен');
    if (r.restart_error)      parts.push('предупреждение: ' + r.restart_error);
    setBackupsStatus(parts.join('; '), r.restart_error ? 'err' : 'ok');
    await loadBackups();
    await refreshStatus();
  } catch (e) {
    setBackupsStatus('ошибка: ' + e.message, 'err');
  }
}

async function deleteBackup(file) {
  if (!confirm('Удалить этот бэкап навсегда?')) return;
  try {
    await api('/api/backups?file=' + encodeURIComponent(file), { method: 'DELETE' });
    setBackupsStatus('удалено', 'ok');
    await loadBackups();
  } catch (e) {
    setBackupsStatus('ошибка: ' + e.message, 'err');
  }
}

$('btn-backups-refresh').addEventListener('click', loadBackups);

// initial + auto-refresh
refreshStatus();
setInterval(refreshStatus, 5000);
