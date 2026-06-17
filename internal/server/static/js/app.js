const state = { schemes: [], settings: [], activeScheme: 'active', platform: null, diffTarget: null, diffMap: {}, dirty: new Set() };

const $ = (id) => document.getElementById(id);

async function api(path, options = {}) {
  const res = await fetch(path, { headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options });
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try { const err = await res.json(); msg = err.error || msg; } catch {}
    throw new Error(msg);
  }
  const type = res.headers.get('content-type') || '';
  return type.includes('application/json') ? res.json() : res.text();
}

/* Toast */
function showMessage(text, type = 'ok') {
  const el = $('toast');
  el.textContent = text;
  el.className = `toast ${type}`;
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add('show'));
  clearTimeout(showMessage.timer);
  showMessage.timer = setTimeout(() => {
    el.classList.remove('show');
    setTimeout(() => el.hidden = true, 300);
  }, 4000);
}

/* Utilities */
function debounce(fn, ms) {
  let t;
  return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
}

function escapeHtml(s) {
  return String(s ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
}

function highlight(text, query) {
  if (!query) return escapeHtml(text);
  const q = escapeHtml(query);
  const regex = new RegExp(`(${q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
  return escapeHtml(text).replace(regex, '<mark>$1</mark>');
}

async function copyGuid(guid) {
  try {
    await navigator.clipboard.writeText(guid);
    showMessage('GUID 已复制到剪贴板');
  } catch {
    showMessage('复制失败', 'error');
  }
}

function setLoading(el, loading) {
  el.classList.toggle('loading', loading);
  el.disabled = loading;
}

async function loadAll() {
  try {
    const health = await api('/api/health');
    state.platform = health.platform;
    $('status').textContent = health.platform.windows ? `已连接。平台角色：${health.platform.role}` : '已连接，但此系统不支持 Windows 电源 API。';
    state.schemes = await api('/api/schemes');
    const active = await api('/api/schemes/active');
    state.activeScheme = active.guid || 'active';
    renderSchemes();
    await loadSettings();
  } catch (err) {
    $('status').textContent = '连接或读取失败';
    showMessage(err.message, 'error');
  }
}

async function loadSettings() {
  $('settings').innerHTML = '<p class="muted">正在加载设置...</p>';
  state.settings = await api(`/api/settings?scheme=${encodeURIComponent(state.activeScheme || 'active')}`);
  renderSettings();
}

function renderSchemes() {
  const select = $('schemeSelect');
  select.innerHTML = state.schemes.map(s =>
    `<option value="${s.guid}" ${s.active ? 'selected' : ''}>${escapeHtml(s.name)} ${s.active ? '(当前)' : ''}</option>`
  ).join('') || '<option disabled>没有电源方案</option>';

  select.onchange = async () => {
    const guid = select.value;
    const scheme = state.schemes.find(s => s.guid === guid);
    if (!scheme) return;
    try {
      if (!scheme.active) await api(`/api/schemes/${guid}/activate`, { method: 'POST' });
      state.activeScheme = guid;
      await loadAll();
      showMessage('电源方案已激活/刷新');
    } catch (err) { showMessage(err.message, 'error'); }
  };

  /* update diff target options */
  const diffSelect = $('diffTarget');
  const currentVal = diffSelect.value;
  diffSelect.innerHTML = `
    <option value="">显示全部</option>
    <option value="default">与默认值对比</option>
    <option disabled>─────────</option>
    ${state.schemes.map(s => `<option value="${s.guid}">与 "${escapeHtml(s.name)}" 对比</option>`).join('')}
  `;
  if ([...diffSelect.options].some(o => o.value === currentVal)) {
    diffSelect.value = currentVal;
  } else {
    diffSelect.value = '';
    if (state.diffTarget && state.diffTarget !== currentVal) {
      state.diffTarget = null;
      state.diffMap = {};
      renderSettings();
    }
  }
}

function renderSettings() {
  const filter = $('filterInput').value.trim().toLowerCase();
  const showHidden = $('showHidden').checked;
  const groups = new Map();
  for (const s of state.settings) {
    const text = `${s.name} ${s.subgroup} ${s.guid} ${s.subgroupGuid}`.toLowerCase();
    if (filter && !text.includes(filter)) continue;
    if (!showHidden && s.hidden) continue;
    if (state.diffTarget && !state.diffMap[s.guid]) continue;
    if (!groups.has(s.subgroup)) groups.set(s.subgroup, []);
    groups.get(s.subgroup).push(s);
  }
  const emptyMsg = state.diffTarget
    ? (state.diffTarget === 'default' ? '没有发现与默认值不同的设置。' : '没有发现与对比方案不同的设置。')
    : '没有匹配的设置。';
  $('settings').innerHTML = [...groups.entries()].map(([name, items]) => `
    <details class="group" open>
      <summary>${highlight(name, filter)} <span class="badge">${items.length}</span></summary>
      ${items.map(s => renderSetting(s, filter)).join('')}
    </details>`).join('') || `<p class="muted">${emptyMsg}</p>`;
  bindSettingEvents();
}

function diffLabel() {
  if (!state.diffTarget) return '默认';
  if (state.diffTarget === 'default') return '默认';
  const scheme = state.schemes.find(s => s.guid === state.diffTarget);
  return scheme ? `"${scheme.name}"` : '对比方案';
}

function renderSetting(s, filter) {
  const v = s.values?.[0] || {};
  const diff = state.diffTarget && state.diffMap[s.guid];
  const isDirty = state.dirty.has(s.guid);
  const input = (kind, value) => {
    if (s.isRanged) return `<input type="number" data-kind="${kind}" data-setting="${s.guid}" value="${value ?? ''}" min="${s.min ?? ''}" max="${s.max ?? ''}" step="${s.increment || 1}">`;
    return `<select data-kind="${kind}" data-setting="${s.guid}">${(s.possibleValues || []).map(p => `<option value="${p.index}" ${p.index === value ? 'selected' : ''}>${escapeHtml(p.name)} (${p.index})</option>`).join('')}</select>`;
  };
  const rangeHint = s.isRanged ? `<span class="full rangeHint">范围: ${s.min ?? '?'} ~ ${s.max ?? '?'}${s.units ? ' ' + escapeHtml(s.units) : ''}</span>` : '';
  return `<div class="setting ${s.hidden ? 'hidden' : ''} ${isDirty ? 'dirty' : ''}" data-row="${s.guid}">
    <div>
      <h3>${highlight(s.name, filter)} ${s.hidden ? '<span class="badge">hidden</span>' : ''} ${s.isRanged ? '<span class="badge">range</span>' : '<span class="badge">enum</span>'} ${isDirty ? '<span class="dirtyBadge">未保存</span>' : ''}</h3>
      <p class="desc">${escapeHtml(s.description || '')}</p>
      <div class="guid">${s.subgroupGuid} / ${s.guid} <button class="copyBtn" data-copy="${s.guid}" title="复制 GUID">⎘</button></div>
      ${diff ? `<div class="diffInfo">
        ${diff.diffs.map(d => `<p><strong>${escapeHtml(d.source)}</strong>: 当前 <span class="diffCurrent">${escapeHtml(d.currentText)}</span>，${escapeHtml(diffLabel())} <span class="diffDefault">${escapeHtml(d.defaultText)}</span></p>`).join('')}
      </div>` : ''}
    </div>
    <div class="editor">
      ${rangeHint}
      <label>AC</label>${input('ac', v.ac)}<span class="default">默认：${formatValue(s, v.acDefault)}</span>
      <label>DC</label>${input('dc', v.dc)}<span class="default">默认：${formatValue(s, v.dcDefault)}</span>
      <label class="full"><input type="checkbox" data-hidden="${s.guid}" ${s.hidden ? 'checked' : ''}> 隐藏</label>
      <button class="full" data-save="${s.guid}">保存</button>
    </div>
  </div>`;
}

function markDirty(guid, dirty = true) {
  const changed = dirty ? state.dirty.add(guid) : state.dirty.delete(guid);
  const row = document.querySelector(`.setting[data-row="${guid}"]`);
  if (!row) return;
  row.classList.toggle('dirty', dirty);
  const badge = row.querySelector('.dirtyBadge');
  if (dirty && !badge) {
    const h3 = row.querySelector('h3');
    h3.insertAdjacentHTML('beforeend', '<span class="dirtyBadge">未保存</span>');
  } else if (!dirty && badge) {
    badge.remove();
  }
}

function bindSettingEvents() {
  document.querySelectorAll('[data-save]').forEach(btn => btn.onclick = async () => {
    const guid = btn.dataset.save;
    const setting = state.settings.find(s => s.guid === guid);
    const ac = document.querySelector(`[data-setting="${setting.guid}"][data-kind="ac"]`)?.value;
    const dc = document.querySelector(`[data-setting="${setting.guid}"][data-kind="dc"]`)?.value;
    setLoading(btn, true);
    try {
      await api(`/api/settings/${setting.guid}`, { method: 'PUT', body: JSON.stringify({ scheme: state.activeScheme, subgroup: setting.subgroupGuid, ac: ac === '' ? null : Number(ac), dc: dc === '' ? null : Number(dc) }) });
      state.dirty.delete(guid);
      showMessage('设置已保存');
      await loadSettings();
    } catch (err) { showMessage(err.message, 'error'); }
    finally { setLoading(btn, false); }
  });
  document.querySelectorAll('[data-hidden]').forEach(chk => chk.onchange = async () => {
    const setting = state.settings.find(s => s.guid === chk.dataset.hidden);
    try {
      await api(`/api/settings/${setting.guid}/hidden`, { method: 'PUT', body: JSON.stringify({ subgroup: setting.subgroupGuid, hidden: chk.checked }) });
      showMessage('hidden 状态已更新');
      await loadSettings();
    } catch (err) { showMessage(err.message, 'error'); chk.checked = !chk.checked; }
  });
  document.querySelectorAll('[data-setting]').forEach(el => {
    el.onchange = () => markDirty(el.dataset.setting, true);
    if (el.tagName === 'INPUT') el.oninput = () => markDirty(el.dataset.setting, true);
  });
  document.querySelectorAll('[data-copy]').forEach(btn => btn.onclick = () => copyGuid(btn.dataset.copy));
}

function formatValue(setting, value) {
  if (value === undefined || value === null) return '-';
  if (setting.isRanged) return `${value}${setting.units ? ' ' + setting.units : ''}`;
  const pv = (setting.possibleValues || []).find(p => p.index === value);
  return pv ? `${pv.name} (${value})` : String(value);
}

async function loadDiff() {
  const target = $('diffTarget').value;
  state.diffTarget = target || null;
  state.diffMap = {};
  if (!state.diffTarget) {
    renderSettings();
    return;
  }
  try {
    const url = state.diffTarget === 'default'
      ? `/api/diff/${encodeURIComponent(state.activeScheme || 'active')}`
      : `/api/diff/${encodeURIComponent(state.activeScheme || 'active')}?compare=${encodeURIComponent(state.diffTarget)}`;
    const items = await api(url);
    for (const item of items) {
      state.diffMap[item.settingGuid] = item;
    }
    renderSettings();
  } catch (err) { showMessage(err.message, 'error'); }
}

function download(url) { window.location.href = url; }

$('refreshBtn').onclick = loadAll;
$('diffTarget').onchange = loadDiff;
$('collapseBtn').onclick = () => {
  const groups = document.querySelectorAll('details.group');
  const anyOpen = [...groups].some(g => g.open);
  groups.forEach(g => g.open = !anyOpen);
  $('collapseBtn').textContent = anyOpen ? '全部展开' : '全部折叠';
};
$('exportBtn').onclick = () => download(`/api/export?scheme=${encodeURIComponent(state.activeScheme || 'active')}`);
$('scriptBtn').onclick = () => download(`/api/script?scheme=${encodeURIComponent(state.activeScheme || 'active')}`);
$('exportDiffBtn').onclick = async () => {
  if (!state.diffTarget) {
    showMessage('请先选择对比目标', 'error');
    return;
  }
  const url = state.diffTarget === 'default'
    ? `/api/diff/${encodeURIComponent(state.activeScheme || 'active')}`
    : `/api/diff/${encodeURIComponent(state.activeScheme || 'active')}?compare=${encodeURIComponent(state.diffTarget)}`;
  const items = await api(url);
  const blob = new Blob([JSON.stringify(items, null, 2)], { type: 'application/json' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `diff-${state.activeScheme}-${state.diffTarget === 'default' ? 'default' : state.diffTarget}.json`;
  a.click();
  URL.revokeObjectURL(a.href);
  showMessage('对比结果已导出为 JSON');
};
$('filterInput').oninput = debounce(renderSettings, 150);
$('showHidden').onchange = renderSettings;

/* Search clear */
const clearSearchBtn = $('clearSearch');
function updateClearBtn() {
  clearSearchBtn.hidden = !$('filterInput').value;
}
$('filterInput').addEventListener('input', updateClearBtn);
clearSearchBtn.onclick = () => {
  $('filterInput').value = '';
  updateClearBtn();
  renderSettings();
  $('filterInput').focus();
};

/* Import with confirmation */
$('importInput').onchange = async (ev) => {
  const file = ev.target.files[0];
  if (!file) return;
  if (!confirm(`确定要导入 "${file.name}" 吗？这将覆盖当前电源方案中的同名设置。`)) {
    ev.target.value = '';
    return;
  }
  try {
    const text = await file.text();
    const result = await api('/api/import', { method: 'POST', headers: { 'Content-Type': 'application/xml' }, body: text });
    showMessage(`导入完成：成功 ${result.applied}，失败 ${result.failed}`);
    await loadSettings();
  } catch (err) { showMessage(err.message, 'error'); }
  ev.target.value = '';
};

/* Keyboard shortcuts */
document.addEventListener('keydown', (e) => {
  if (e.key === 'k' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault();
    $('filterInput').focus();
    $('filterInput').select();
  }
  if (e.key === 'Escape') {
    if (document.activeElement === $('filterInput')) {
      $('filterInput').value = '';
      updateClearBtn();
      renderSettings();
      document.activeElement.blur();
    }
  }
  if (e.key === 'F5') {
    e.preventDefault();
    loadAll();
  }
});

loadAll();
