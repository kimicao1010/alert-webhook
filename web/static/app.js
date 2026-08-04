/* Alertmanager Webhook 控制台 — 纯静态前端
 * 对接 /api/* REST 接口，认证走 Authorization: Bearer <token>（--auth-token）
 */
'use strict';

/* ================= 工具与状态 ================= */

const $ = (id) => document.getElementById(id);

const state = {
  token: localStorage.getItem('awh-token') || '',
  tab: 'channels',
  channels: [],
  curChannel: null,
  templates: [],
  curTmpl: null,
  tmplDirty: false,
  sends: { offset: 0, limit: 20, channel: '', status: '', total: 0, records: [] },
  autoRefreshTimer: null,
  previewTimer: null,
  testResult: null,
};

// 渠道元信息（显示名 + 配置字段定义）
const CHANNEL_META = {
  feishu: {
    name: '飞书', desc: '群机器人',
    fields: [
      { key: 'token', label: 'Webhook Token', required: true, secret: true },
      { key: 'msg_type', label: '消息类型', type: 'select', options: ['markdown', 'text', 'post', 'interactive'] },
    ],
  },
  dingtalk: {
    name: '钉钉', desc: '群机器人',
    fields: [
      { key: 'token', label: 'Webhook Token', required: true, secret: true },
      { key: 'secret', label: '加签密钥（可选）', secret: true },
      { key: 'msg_type', label: '消息类型', type: 'select', options: ['markdown', 'text', 'link', 'actionCard'] },
    ],
  },
  weixin: {
    name: '企业微信', desc: '群机器人',
    fields: [
      { key: 'token', label: 'Webhook Token', required: true, secret: true },
      { key: 'msg_type', label: '消息类型', type: 'select', options: ['markdown', 'text', 'news'] },
    ],
  },
  weixinapp: {
    name: '企业微信应用', desc: '应用消息',
    fields: [
      { key: 'corp_id', label: 'Corp ID', required: true },
      { key: 'agent_id', label: 'Agent ID', required: true },
      { key: 'agent_secret', label: 'Agent Secret', required: true, secret: true },
      { key: 'to_user', label: 'To User' },
      { key: 'to_party', label: 'To Party' },
      { key: 'to_tag', label: 'To Tag' },
      { key: 'msg_type', label: '消息类型', type: 'select', options: ['markdown', 'text', 'news'] },
    ],
  },
};

/* ================= API 封装 ================= */

async function api(path, opts = {}) {
  const headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers || {});
  if (state.token) headers['Authorization'] = 'Bearer ' + state.token;
  const resp = await fetch(path, Object.assign({}, opts, { headers }));
  if (resp.status === 401) {
    toast('认证失败：请填写正确的 --auth-token', true);
    throw new Error('unauthorized');
  }
  const text = await resp.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch (e) { data = text; }
  if (!resp.ok) {
    const msg = (data && data.error) ? data.error : ('HTTP ' + resp.status);
    throw new Error(msg);
  }
  return data;
}

function toast(msg, isErr) {
  const t = $('toast');
  t.textContent = msg;
  t.className = 'toast' + (isErr ? ' err' : '');
  t.style.display = 'block';
  clearTimeout(t._timer);
  t._timer = setTimeout(() => { t.style.display = 'none'; }, 2600);
}

function fmtTime(ts) {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

/* ================= Tab 切换 ================= */

function switchTab(tab) {
  state.tab = tab;
  document.querySelectorAll('.nav-item').forEach((n) => n.classList.toggle('active', n.dataset.tab === tab));
  const pages = { channels: 'page-channels', templates: 'page-templates', sends: 'page-sends', test: 'page-test' };
  Object.entries(pages).forEach(([k, id]) => { $(id).style.display = (k === tab) ? 'block' : 'none'; });
  $('crumb').textContent = document.querySelector(`.nav-item[data-tab="${tab}"]`).textContent.trim();

  if (tab === 'channels') loadChannels();
  if (tab === 'templates') loadTemplates();
  if (tab === 'sends') { loadSends(); startAutoRefresh(); }
  if (tab === 'test') { loadTestChannelOptions(); }
  if (tab !== 'sends') stopAutoRefresh();
}

document.querySelectorAll('.nav-item').forEach((n) => n.addEventListener('click', () => switchTab(n.dataset.tab)));

/* ================= 渠道配置 ================= */

async function loadChannels() {
  try {
    const list = await api('/api/channels');
    state.channels = Array.isArray(list) ? list : [];
    renderChannelList();
  } catch (e) { /* 401 已提示 */ }
}

function renderChannelList() {
  const box = $('chan-list');
  $('chan-count').textContent = state.channels.length + ' 个渠道';
  if (state.channels.length === 0) {
    box.innerHTML = '<div class="empty"><div class="big">▤</div>暂无渠道，点击"新增渠道"开始配置</div>';
    return;
  }
  const configured = new Set(state.channels);
  const all = Object.keys(CHANNEL_META);
  const ordered = [...new Set([...state.channels, ...all])];
  box.innerHTML = ordered.map((ch) => {
    const meta = CHANNEL_META[ch];
    const isCfg = configured.has(ch);
    return `<div class="chan ${ch === state.curChannel ? 'active' : ''}" data-channel="${esc(ch)}">
      <div class="top"><span class="cn">${meta ? esc(meta.name) : esc(ch)}</span>
      <span class="badge ${isCfg ? 'green' : 'gray'}">${isCfg ? '已配置' : '未配置'}</span></div>
      <div class="cd">${ch}${meta ? ' · ' + esc(meta.desc) : ''}</div>
    </div>`;
  }).join('');
  box.querySelectorAll('.chan').forEach((el) => el.addEventListener('click', () => {
    state.curChannel = el.dataset.channel;
    renderChannelList();
    renderChannelForm();
  }));
}

function renderChannelForm() {
  const form = $('chan-form');
  const ch = state.curChannel;
  const meta = CHANNEL_META[ch];
  $('chan-form-title').textContent = (meta ? meta.name : ch) + ' · 配置';
  $('chan-form-hint').textContent = 'channel: ' + (ch || '—');

  if (!ch) {
    form.innerHTML = '<div class="empty"><div class="big">←</div>从左侧选择渠道，或点击"新增渠道"</div>';
    return;
  }
  if (!meta) {
    form.innerHTML = `<div class="empty">未知渠道 ${esc(ch)}</div>`;
    return;
  }

  let cfg = {};
  api(`/api/channels/${ch}`).then((c) => {
    cfg = c || {};
    form.innerHTML = meta.fields.map((f) => {
      const val = cfg[f.key] != null ? cfg[f.key] : (f.type === 'select' && f.options ? f.options[0] : '');
      if (f.type === 'select') {
        return `<div class="field"><label>${f.label}</label>
          <select data-key="${f.key}">${f.options.map((o) => `<option ${o === val ? 'selected' : ''}>${o}</option>`).join('')}</select></div>`;
      }
      if (f.secret) {
        return `<div class="field"><label>${f.label} ${f.required ? '<span class="req">*</span>' : ''}</label>
          <div class="secret"><input type="password" data-key="${f.key}" value="${esc(val)}" placeholder="输入后保存" />
          <span class="eye">👁</span></div></div>`;
      }
      return `<div class="field"><label>${f.label} ${f.required ? '<span class="req">*</span>' : ''}</label>
        <input data-key="${f.key}" value="${esc(val)}" /></div>`;
    }).join('') + `
    <div class="form-actions">
      <button class="btn primary" id="btn-save-chan">保存配置</button>
      <button class="btn" id="btn-test-chan">测试发送</button>
      <button class="btn danger" style="margin-left:auto" id="btn-del-chan">删除渠道</button>
    </div>`;
    form.querySelectorAll('.eye').forEach((e) => e.addEventListener('click', () => {
      const inp = e.previousElementSibling;
      inp.type = inp.type === 'password' ? 'text' : 'password';
    }));
    $('btn-save-chan').addEventListener('click', () => saveChannel(ch, form));
    $('btn-del-chan').addEventListener('click', () => deleteChannel(ch));
    $('btn-test-chan').addEventListener('click', () => {
      switchTab('test');
      $('test-channel').value = ch;
    });
  }).catch(() => {});
}

function collectForm(form) {
  const cfg = {};
  form.querySelectorAll('[data-key]').forEach((el) => { cfg[el.dataset.key] = el.value.trim(); });
  // 清空未填的 secret 字段（不覆盖已有值）
  return cfg;
}

async function saveChannel(ch, form) {
  const cfg = collectForm(form);
  const meta = CHANNEL_META[ch];
  // 必填校验
  for (const f of meta.fields) {
    if (f.required && !cfg[f.key]) {
      toast(`请填写 ${f.label}`, true);
      return;
    }
  }
  try {
    await api(`/api/channels/${ch}`, { method: 'POST', body: JSON.stringify(cfg) });
    toast('渠道配置已保存');
    await loadChannels();
  } catch (e) { toast('保存失败：' + e.message, true); }
}

async function deleteChannel(ch) {
  if (!confirm(`确认删除渠道 ${ch} 的配置？`)) return;
  try {
    await api(`/api/channels/${ch}`, { method: 'DELETE' });
    state.curChannel = null;
    toast('渠道已删除');
    await loadChannels();
    renderChannelForm();
  } catch (e) { toast('删除失败：' + e.message, true); }
}

$('btn-add-channel').addEventListener('click', () => {
  const keys = Object.keys(CHANNEL_META).filter((k) => !state.channels.includes(k));
  if (keys.length === 0) { toast('所有渠道均已配置', true); return; }
  const name = prompt('新增渠道：输入渠道名\n可用：' + keys.join(', '), keys[0]);
  if (name && CHANNEL_META[name]) {
    state.curChannel = name;
    renderChannelList();
    renderChannelForm();
  } else if (name) {
    toast('未知渠道类型', true);
  }
});

/* ================= 模板编辑（所见即所得） ================= */

async function loadTemplates() {
  try {
    const list = await api('/api/templates');
    state.templates = Array.isArray(list) ? list : [];
    renderTemplateList();
  } catch (e) { /* 401 已提示 */ }
}

function renderTemplateList() {
  const box = $('tmpl-list');
  $('tmpl-count').textContent = state.templates.length + ' 个';
  if (state.templates.length === 0) {
    box.innerHTML = '<div class="empty">暂无模板</div>';
    return;
  }
  box.innerHTML = state.templates.map((t) => {
    const lang = t.endsWith('.zh.tmpl') ? 'zh' : (t.endsWith('.tmpl') ? 'en' : '');
    return `<div class="tmpl-file ${t === state.curTmpl ? 'active' : ''}" data-tmpl="${esc(t)}">
      <span>${esc(t)}</span>${lang ? `<span class="lang">${lang}</span>` : ''}</div>`;
  }).join('');
  box.querySelectorAll('.tmpl-file').forEach((el) => el.addEventListener('click', () => {
    state.curTmpl = el.dataset.tmpl;
    renderTemplateList();
    loadTemplateContent();
  }));
  if (!state.curTmpl || !state.templates.includes(state.curTmpl)) {
    state.curTmpl = state.templates[0];
  }
  loadTemplateContent();
}

async function loadTemplateContent() {
  const name = state.curTmpl;
  $('tmpl-filename').textContent = name || '—';
  if (!name) { $('tmpl-editor').value = ''; return; }
  try {
    const data = await api(`/api/templates/${encodeURIComponent(name)}`);
    $('tmpl-editor').value = data.content || '';
    state.tmplDirty = false;
    doPreview();
  } catch (e) { toast('加载模板失败：' + e.message, true); }
}

// 所见即所得：input 事件 + 300ms 防抖 → 实时预览
$('tmpl-editor').addEventListener('input', () => {
  state.tmplDirty = true;
  clearTimeout(state.previewTimer);
  state.previewTimer = setTimeout(doPreview, 300);
});

// 示例告警数据（与后端渲染用同一结构）
const SAMPLE_ALERT = {
  status: 'firing',
  groupLabels: { alertname: 'KubePodCrashLooping', severity: 'warning' },
  commonLabels: { alertname: 'KubePodCrashLooping' },
  commonAnnotations: { summary: 'Pod 处于 CrashLoopBackOff 状态' },
  alerts: [
    {
      status: 'firing',
      labels: { alertname: 'KubePodCrashLooping', instance: 'prod/nginx-7d8f9b', severity: 'warning' },
      annotations: { summary: 'Pod 处于 CrashLoopBackOff 状态' },
      startsAt: '2026-08-03T07:00:00Z',
    },
    {
      status: 'firing',
      labels: { alertname: 'KubePodCrashLooping', instance: 'prod/redis-6f2a88', severity: 'warning' },
      annotations: { summary: 'Pod 处于 CrashLoopBackOff 状态' },
      startsAt: '2026-08-03T07:00:00Z',
    },
  ],
  externalURL: 'http://alertmanager.example.com',
};

async function doPreview() {
  const content = $('tmpl-editor').value;
  if (!content.trim()) {
    $('tmpl-preview').innerHTML = '<div class="empty"><div class="big">◷</div>输入模板内容后实时预览</div>';
    return;
  }
  try {
    const out = await api(`/api/templates/${encodeURIComponent(state.curTmpl || 'x.tmpl')}/preview`, {
      method: 'POST',
      body: JSON.stringify({ content, alert: SAMPLE_ALERT }),
    });
    const rows = ['prom.title', 'prom.text', 'prom.markdown']
      .filter((k) => out[k] != null)
      .map((k) => `<div class="pv-row"><div class="pv-label">${k.replace('prom.', '')}</div><div class="pv-val">${esc(out[k])}</div></div>`)
      .join('');
    $('tmpl-preview').innerHTML = rows || '<div class="empty">模板中未定义可预览的段（prom.title/text/markdown）</div>';
  } catch (e) {
    $('tmpl-preview').innerHTML = `<div class="pv-error">渲染失败：${esc(e.message)}</div>`;
  }
}

$('btn-save-tmpl').addEventListener('click', async () => {
  const name = state.curTmpl;
  if (!name) { toast('请先选择模板文件', true); return; }
  const content = $('tmpl-editor').value;
  try {
    await api(`/api/templates/${encodeURIComponent(name)}`, { method: 'POST', body: JSON.stringify({ content }) });
    state.tmplDirty = false;
    toast('模板已保存，已热重载');
  } catch (e) { toast('保存失败：' + e.message, true); }
});

window.addEventListener('beforeunload', (e) => {
  if (state.tmplDirty) { e.preventDefault(); e.returnValue = ''; }
});

/* ================= 发送结果 ================= */

async function loadSends() {
  const { offset, limit, channel, status } = state.sends;
  const q = new URLSearchParams({ offset, limit });
  if (channel) q.set('channel', channel);
  if (status) q.set('status', status);
  try {
    const data = await api('/api/sends?' + q.toString());
    state.sends.total = data.total || 0;
    state.sends.records = data.records || [];
    renderSends();
  } catch (e) { /* 401 已提示 */ }
}

function renderSends() {
  const { records, total, offset, limit } = state.sends;

  // 统计卡
  const succ = records.filter((r) => r.status === 'success').length;
  const fail = records.filter((r) => r.status === 'failure').length;
  const avgMs = records.length
    ? Math.round(records.reduce((s, r) => s + (r.durationMs || 0), 0) / records.length)
    : 0;
  $('send-stats').innerHTML = `
    <div class="stat"><span class="k">当前页发送</span><span class="v">${records.length}</span><span class="delta">共 ${total} 条</span></div>
    <div class="stat"><span class="k">成功</span><span class="v ok">${succ}</span><span class="delta">本页</span></div>
    <div class="stat"><span class="k">失败</span><span class="v fail">${fail}</span><span class="delta">本页</span></div>
    <div class="stat"><span class="k">平均耗时</span><span class="v warn">${avgMs}ms</span><span class="delta">本页</span></div>`;

  const tbody = $('sends-tbody');
  if (records.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7"><div class="empty"><div class="big">◷</div>暂无发送记录</div></td></tr>';
  } else {
    tbody.innerHTML = records.map((r, i) => {
      const pill = r.status === 'success' ? '<span class="pill succ">成功</span>' : '<span class="pill fail">失败</span>';
      return `<tr data-idx="${i}">
        <td class="time">${fmtTime(r.timestamp)}</td>
        <td><span class="chan-tag">${esc(r.channel)}</span>${r.kind === 'test' ? ' <span class="pill" style="background:var(--warn-bg);color:var(--warn)">测试</span>' : ''}</td>
        <td>${pill}</td>
        <td>${r.alertCount != null ? r.alertCount : '—'}</td>
        <td class="time">${r.durationMs != null ? r.durationMs + 'ms' : '—'}</td>
        <td>${r.error ? `<span class="err-text" title="${esc(r.error)}">${esc(r.error)}</span>` : '—'}</td>
        <td><span class="chev">▸</span></td>
      </tr>
      <tr class="row-detail" data-detail="${i}">
        <td colspan="7"><div class="detail-body">
          <div class="detail-grid">
            <div class="detail-item"><div class="dk">渠道</div><div class="dv">${esc(r.channel)}</div></div>
            <div class="detail-item"><div class="dk">类型</div><div class="dv">${r.kind === 'test' ? '测试发送' : '真实发送'}</div></div>
            <div class="detail-item"><div class="dk">状态</div><div class="dv">${r.status}</div></div>
            <div class="detail-item"><div class="dk">触发时间</div><div class="dv">${fmtTime(r.timestamp)}</div></div>
          </div>
          <div class="detail-block"><div class="db-t">告警数</div><pre>${r.alertCount != null ? r.alertCount : '—'}</pre></div>
          ${r.error ? `<div class="detail-block"><div class="db-t">错误信息</div><pre class="err">${esc(r.error)}</pre></div>` : ''}
        </div></td>
      </tr>`;
    }).join('');
    tbody.querySelectorAll('tr[data-idx]').forEach((row) => row.addEventListener('click', () => {
      const idx = row.dataset.idx;
      const detail = tbody.querySelector(`.row-detail[data-detail="${idx}"]`);
      const chev = row.querySelector('.chev');
      const wasOpen = detail.classList.contains('open');
      tbody.querySelectorAll('.row-detail.open').forEach((d) => d.classList.remove('open'));
      tbody.querySelectorAll('.chev').forEach((c) => (c.textContent = '▸'));
      if (!wasOpen) { detail.classList.add('open'); chev.textContent = '▾'; }
    }));
  }

  const pages = Math.max(1, Math.ceil(total / limit));
  $('sends-total').textContent = `共 ${total} 条`;
  $('page-info').textContent = `${Math.floor(offset / limit) + 1} / ${pages}`;
}

function startAutoRefresh() {
  stopAutoRefresh();
  state.autoRefreshTimer = setInterval(loadSends, 5000);
}
function stopAutoRefresh() {
  if (state.autoRefreshTimer) { clearInterval(state.autoRefreshTimer); state.autoRefreshTimer = null; }
}

$('f-channel').addEventListener('change', () => { state.sends.channel = $('f-channel').value; state.sends.offset = 0; loadSends(); });
$('f-status').addEventListener('change', () => { state.sends.status = $('f-status').value; state.sends.offset = 0; loadSends(); });
$('page-size').addEventListener('change', () => { state.sends.limit = parseInt($('page-size').value, 10); state.sends.offset = 0; loadSends(); });
$('btn-prev').addEventListener('click', () => {
  if (state.sends.offset > 0) { state.sends.offset -= state.sends.limit; loadSends(); }
});
$('btn-next').addEventListener('click', () => {
  if (state.sends.offset + state.sends.limit < state.sends.total) { state.sends.offset += state.sends.limit; loadSends(); }
});
$('btn-refresh-sends').addEventListener('click', loadSends);

async function loadChannelFilterOptions() {
  try {
    const list = await api('/api/channels');
    const opts = ['<option value="">全部渠道</option>'].concat(
      list.map((c) => `<option value="${esc(c)}">${esc(c)}</option>`)
    );
    $('f-channel').innerHTML = opts.join('');
  } catch (e) { /* ignore */ }
}

/* ================= 测试发送 ================= */

async function loadTestChannelOptions() {
  try {
    const [list, tmpls] = await Promise.all([api('/api/channels'), api('/api/templates')]);
    state.templates = Array.isArray(tmpls) ? tmpls : [];
    const sel = $('test-channel');
    if (list.length === 0) {
      sel.innerHTML = '<option value="">（请先在渠道配置页添加渠道）</option>';
    } else {
      sel.innerHTML = list.map((c) => `<option value="${esc(c)}">${esc(c)}</option>`).join('');
    }
    updateTemplateOptions();
  } catch (e) { /* ignore */ }
}

// 模板渠道约束：模板名以 <channel>. 开头（如 feishu.tmpl / feishu.zh.tmpl）
function updateTemplateOptions() {
  const channel = $('test-channel').value;
  const sel = $('test-template');
  if (!channel) {
    sel.innerHTML = '<option value="">不使用模板（简单文本）</option>';
    sel.disabled = true;
  } else {
    const matched = state.templates.filter((t) => t.startsWith(channel + '.'));
    sel.disabled = false;
    sel.innerHTML = '<option value="">不使用模板（简单文本）</option>' +
      matched.map((t) => `<option value="${esc(t)}">${esc(t)}</option>`).join('');
  }
  toggleTemplateFields();
}

function toggleTemplateFields() {
  const useTmpl = $('test-template').value !== '';
  $('test-template-fields').style.display = useTmpl ? 'block' : 'none';
  $('test-text-wrap').style.display = useTmpl ? 'none' : 'block';
}

// 自定义 KV 行管理
function addKvRow(k = '', v = '') {
  const row = document.createElement('div');
  row.style.cssText = 'display:flex;gap:8px;margin-bottom:6px';
  row.innerHTML = `
    <input placeholder="key" value="${esc(k)}" style="flex:1" />
    <input placeholder="value" value="${esc(v)}" style="flex:1" />
    <button class="btn ghost" type="button" style="padding:4px 10px">✕</button>`;
  row.querySelector('button').addEventListener('click', () => row.remove());
  $('kv-rows').appendChild(row);
}

$('test-channel').addEventListener('change', updateTemplateOptions);
$('test-template').addEventListener('change', toggleTemplateFields);
$('btn-add-kv').addEventListener('click', () => addKvRow());

function collectTestSendBody() {
  const channel = $('test-channel').value;
  const template = $('test-template').value;
  const body = { channel };
  if (template) {
    body.template = template;
    body.fields = {
      status: $('fld-status').value,
      alertname: $('fld-alertname').value.trim(),
      severity: $('fld-severity').value.trim(),
      instance: $('fld-instance').value.trim(),
      summary: $('fld-summary').value.trim(),
      externalURL: $('fld-externalURL').value.trim(),
    };
    body.labels = {};
    document.querySelectorAll('#kv-rows > div').forEach((row) => {
      const [k, v] = row.querySelectorAll('input');
      if (k.value.trim()) body.labels[k.value.trim()] = v.value.trim();
    });
  } else {
    body.text = $('test-text').value.trim();
  }
  return body;
}

$('btn-test-send').addEventListener('click', async () => {
  const body = collectTestSendBody();
  if (!body.channel) { toast('请先配置渠道', true); return; }
  if (!body.template && !body.text) { toast('请输入测试内容', true); return; }
  const btn = $('btn-test-send');
  btn.disabled = true;
  btn.textContent = '发送中…';
  try {
    await api('/api/test-send', { method: 'POST', body: JSON.stringify(body) });
    $('test-result').innerHTML = `<div class="res-box ok">✓ 发送成功：${esc(body.channel)}${body.template ? '（模板 ' + esc(body.template) + '）' : ''} 已收到测试消息<br>
      <span style="font-size:11px;color:var(--muted)">${fmtTime(Math.floor(Date.now() / 1000))}</span></div>`;
    toast('测试消息发送成功');
  } catch (e) {
    $('test-result').innerHTML = `<div class="res-box fail">✗ 发送失败：${esc(e.message)}</div>`;
    toast('测试发送失败', true);
  } finally {
    btn.disabled = false;
    btn.textContent = '发送测试消息';
  }
});

/* ================= 初始化 ================= */

$('token-input').value = state.token;
$('token-input').addEventListener('change', () => {
  state.token = $('token-input').value.trim();
  localStorage.setItem('awh-token', state.token);
  toast('Token 已更新');
  // 刷新当前页
  if (state.tab === 'channels') loadChannels();
  if (state.tab === 'templates') loadTemplates();
  if (state.tab === 'sends') loadSends();
  if (state.tab === 'test') loadTestChannelOptions();
});

// 健康检查
fetch('/healthz').then((r) => {
  $('health-dot').style.background = r.ok ? 'var(--ok)' : 'var(--err)';
}).catch(() => { $('health-dot').style.background = 'var(--err)'; });

// 启动
(async () => {
  loadChannelFilterOptions();
  await loadChannels();
  renderChannelForm();
  switchTab('channels');
})();
