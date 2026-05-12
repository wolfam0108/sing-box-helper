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

async function refreshStatus() {
  try {
    const s = await api('/api/status');
    stSb.textContent = s.sing_box_running
      ? `running, PID ${s.sing_box_pid || '?'} — ${s.sing_box_version || ''}`
      : 'не запущен';
    stTun.textContent = s.tun_up
      ? `UP (${s.tun_name})`
      : `down (${s.tun_name})`;
    stNode.textContent = s.current_node
      ? `${s.current_node.protocol}  ${s.current_node.server}:${s.current_node.port}`
      : 'не задан (нажмите "Применить")';

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
    stSb.textContent = stTun.textContent = stNode.textContent = '—';
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

// initial + auto-refresh
refreshStatus();
setInterval(refreshStatus, 5000);
