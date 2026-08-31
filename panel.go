package main

const panelHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#ffffff">
<title>Codex Health Monitor</title>
<style>
:root {
  color-scheme: light;
  --canvas: #f4f6f8;
  --surface: #ffffff;
  --surface-subtle: #f8fafb;
  --line: #dfe3e8;
  --line-strong: #c8ced6;
  --text: #18202a;
  --muted: #66717e;
  --muted-strong: #4b5663;
  --primary: #1768d4;
  --primary-hover: #0f57b8;
  --primary-soft: #eaf2fd;
  --green: #16794c;
  --green-soft: #e8f6ef;
  --red: #b42318;
  --red-soft: #fcebea;
  --amber: #946200;
  --amber-soft: #fff4d6;
  --shadow-sm: 0 1px 2px rgba(16, 24, 40, .05);
  --shadow-md: 0 14px 36px rgba(16, 24, 40, .14);
}

* { box-sizing: border-box; letter-spacing: 0; }
html { min-width: 320px; background: var(--canvas); }
body {
  margin: 0;
  background: var(--canvas);
  color: var(--text);
  font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  -webkit-font-smoothing: antialiased;
}
button, input { font: inherit; }
button { color: inherit; }
svg { display: block; }
.icon { width: 18px; height: 18px; flex: 0 0 auto; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
.icon-defs { position: absolute; width: 0; height: 0; overflow: hidden; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }

.app-header {
  position: sticky;
  top: 0;
  z-index: 10;
  border-bottom: 1px solid var(--line);
  background: rgba(255, 255, 255, .96);
  backdrop-filter: blur(12px);
}
.header-inner, .page { width: min(1280px, calc(100% - 48px)); margin: 0 auto; }
.header-inner { min-height: 68px; display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
.brand-mark {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 8px;
  background: #18202a;
  color: #ffffff;
  box-shadow: var(--shadow-sm);
}
.brand-mark .icon { width: 22px; height: 22px; }
.brand-copy { min-width: 0; }
.brand-title { margin: 0; font-size: 16px; line-height: 1.3; font-weight: 720; }
.brand-subtitle { margin-top: 2px; color: var(--muted); font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.header-meta { display: flex; align-items: center; gap: 10px; flex: 0 0 auto; }
.model-chip, .connection, .community-link {
  min-height: 32px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--surface);
  padding: 5px 10px;
  color: var(--muted-strong);
  font-size: 12px;
  font-weight: 620;
  white-space: nowrap;
}
.community-link { text-decoration: none; }
.community-link:hover { border-color: #aeb6c0; background: var(--surface-subtle); color: var(--text); }
.community-link .icon { width: 14px; height: 14px; }
.connection-dot { width: 7px; height: 7px; border-radius: 50%; background: #8a94a0; box-shadow: 0 0 0 3px #eef0f2; }
.connection.online .connection-dot { background: var(--green); box-shadow: 0 0 0 3px var(--green-soft); }
.connection.offline .connection-dot { background: var(--red); box-shadow: 0 0 0 3px var(--red-soft); }

.page { padding-top: 28px; padding-bottom: 48px; }
.overview { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric {
  min-width: 0;
  min-height: 150px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  padding: 18px;
  box-shadow: var(--shadow-sm);
}
.metric-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.metric-label { color: var(--muted-strong); font-size: 13px; font-weight: 650; }
.metric-icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: 7px; background: #eef1f4; color: #55606c; }
.metric-icon .icon { width: 17px; height: 17px; }
.metric.positive .metric-icon { background: var(--green-soft); color: var(--green); }
.metric.negative .metric-icon { background: var(--red-soft); color: var(--red); }
.metric.scheduled .metric-icon { background: var(--primary-soft); color: var(--primary); }
.metric-value { margin-top: 12px; font-size: 30px; line-height: 1.08; font-weight: 740; overflow-wrap: anywhere; }
.metric.positive .metric-value { color: var(--green); }
.metric.negative .metric-value.has-errors { color: var(--red); }
.metric-detail { min-height: 18px; margin-top: 8px; color: var(--muted); font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.metric-progress { height: 3px; margin-top: 8px; overflow: hidden; border-radius: 3px; background: #edf0f2; }
.metric-progress span { display: block; width: 0; height: 100%; border-radius: inherit; background: var(--green); transition: width .3s ease; }

.content-section { margin-top: 30px; }
.section-head { min-height: 42px; display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 10px; }
.section-title-wrap { min-width: 0; }
.section-title-row { display: flex; align-items: center; gap: 8px; }
.section-title-row .icon { width: 17px; height: 17px; color: var(--muted-strong); }
.section-title { margin: 0; font-size: 16px; line-height: 1.35; font-weight: 720; }
.section-meta { min-height: 18px; margin-top: 3px; color: var(--muted); font-size: 12px; }
.actions { display: flex; align-items: center; gap: 8px; flex: 0 0 auto; }
.button {
  height: 38px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid var(--line-strong);
  border-radius: 6px;
  background: var(--surface);
  padding: 0 13px;
  color: var(--text);
  font-weight: 680;
  cursor: pointer;
  box-shadow: var(--shadow-sm);
  transition: border-color .15s ease, background .15s ease, color .15s ease, transform .15s ease;
}
.button:hover { border-color: #aeb6c0; background: var(--surface-subtle); }
.button:active { transform: translateY(1px); }
.button:focus-visible, input:focus-visible { outline: 3px solid rgba(23, 104, 212, .18); outline-offset: 1px; border-color: var(--primary); }
.button.primary { border-color: var(--primary); background: var(--primary); color: #ffffff; }
.button.primary:hover { border-color: var(--primary-hover); background: var(--primary-hover); }
.button:disabled { opacity: .56; cursor: not-allowed; transform: none; }
.icon-button { width: 38px; padding: 0; }
.button.is-loading .icon { animation: spin .8s linear infinite; }

.table-wrap { overflow: auto; border: 1px solid var(--line); border-radius: 8px; background: var(--surface); box-shadow: var(--shadow-sm); }
table { width: 100%; min-width: 980px; border-collapse: collapse; }
th, td { padding: 12px 14px; border-bottom: 1px solid #e8ebee; text-align: left; vertical-align: middle; }
th { background: var(--surface-subtle); color: var(--muted-strong); font-size: 11px; font-weight: 720; text-transform: uppercase; white-space: nowrap; }
tbody tr:last-child td { border-bottom: 0; }
tbody tr { transition: background .12s ease; }
tbody tr:hover { background: #fafbfd; }
.account-cell { min-width: 190px; }
.account { display: flex; align-items: center; gap: 10px; min-width: 0; }
.account-avatar { width: 30px; height: 30px; display: grid; place-items: center; flex: 0 0 auto; border-radius: 7px; background: #edf2f7; color: #405064; font-size: 12px; font-weight: 760; text-transform: uppercase; }
.account-email { min-width: 0; font-weight: 650; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
.code-value { display: inline-block; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted-strong); }
.status { width: max-content; max-width: 100%; display: inline-flex; align-items: center; justify-self: start; gap: 6px; min-height: 26px; border-radius: 999px; padding: 3px 8px; font-size: 12px; font-weight: 700; white-space: nowrap; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }
.status.healthy { background: var(--green-soft); color: var(--green); }
.status.bad { background: var(--red-soft); color: var(--red); }
.status.checking { background: var(--amber-soft); color: var(--amber); }
.http-code { width: max-content; display: inline-flex; min-width: 42px; justify-content: center; justify-self: start; border: 1px solid var(--line); border-radius: 5px; background: var(--surface-subtle); padding: 2px 6px; color: var(--muted-strong); font: 650 12px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; }
.http-code.ok { border-color: #b9dfca; background: var(--green-soft); color: var(--green); }
.http-code.fail { border-color: #f0c6c2; background: var(--red-soft); color: var(--red); }
.latency { white-space: nowrap; color: var(--muted-strong); font-variant-numeric: tabular-nums; }
.date { min-width: 150px; color: var(--muted-strong); font-variant-numeric: tabular-nums; white-space: nowrap; }
.error-message { display: block; max-width: 270px; overflow: hidden; color: #71332d; text-overflow: ellipsis; white-space: nowrap; }
.empty { height: 150px; text-align: center; color: var(--muted); }
.empty-state { display: inline-flex; flex-direction: column; align-items: center; gap: 8px; }
.empty-state .icon { width: 24px; height: 24px; color: #9aa3ad; }

.schedule-panel { border: 1px solid var(--line); border-radius: 8px; background: var(--surface); box-shadow: var(--shadow-sm); }
.schedule-panel-head { min-height: 60px; display: flex; align-items: center; justify-content: space-between; gap: 20px; border-bottom: 1px solid var(--line); padding: 13px 16px; }
.schedule-title { display: flex; align-items: center; gap: 9px; font-weight: 720; }
.schedule-title .icon { color: var(--primary); }
.next-run { color: var(--muted); font-size: 12px; text-align: right; }
.next-run strong { display: block; color: var(--text); font-size: 13px; font-weight: 680; }
.schedule-form { padding: 16px; }
.fields { display: grid; grid-template-columns: 1.15fr .8fr 1.15fr .8fr 1.5fr auto; gap: 12px; align-items: end; }
.field { min-width: 0; }
.field.hidden { display: none; }
.field label, .field-label { display: block; margin-bottom: 6px; color: var(--muted-strong); font-size: 12px; font-weight: 650; }
.field input { width: 100%; height: 38px; border: 1px solid var(--line-strong); border-radius: 6px; background: var(--surface); padding: 7px 10px; color: var(--text); }
.field input::placeholder { color: #9aa3ad; }
.segment { height: 38px; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 3px; border: 1px solid var(--line-strong); border-radius: 6px; background: #eef1f4; padding: 3px; }
.segment label { position: relative; min-width: 0; margin: 0; overflow: hidden; }
.segment input { position: absolute; opacity: 0; pointer-events: none; }
.segment span { width: 100%; height: 30px; display: flex; align-items: center; justify-content: center; border-radius: 4px; padding: 0 6px; color: var(--muted-strong); font-size: 12px; font-weight: 680; white-space: nowrap; cursor: pointer; }
.segment input:checked + span { background: var(--surface); color: var(--text); box-shadow: 0 1px 3px rgba(16, 24, 40, .12); }
.segment input:focus-visible + span { outline: 2px solid rgba(23, 104, 212, .25); }
.schedule-alert { display: none; align-items: flex-start; gap: 8px; margin: 0 16px 16px; border: 1px solid #f2d29a; border-radius: 6px; background: #fff8e8; padding: 9px 11px; color: #7a5200; font-size: 12px; }
.schedule-alert.show { display: flex; }
.schedule-alert .icon { width: 16px; height: 16px; margin-top: 1px; }

.notice { position: fixed; right: 20px; bottom: 20px; z-index: 30; max-width: min(420px, calc(100% - 40px)); display: flex; align-items: center; gap: 9px; border-radius: 7px; background: #18202a; padding: 11px 14px; color: #ffffff; box-shadow: var(--shadow-md); opacity: 0; transform: translateY(8px); pointer-events: none; transition: opacity .18s ease, transform .18s ease; }
.notice.show { opacity: 1; transform: translateY(0); }
.notice.error-notice { background: #8f211a; }
.notice .icon { width: 17px; height: 17px; }
.auth-overlay { position: fixed; inset: 0; z-index: 40; display: none; place-items: center; background: rgba(17, 24, 39, .52); padding: 20px; backdrop-filter: blur(3px); }
.auth-overlay.show { display: grid; }
.auth-dialog { width: min(420px, 100%); border: 1px solid var(--line); border-radius: 8px; background: var(--surface); padding: 22px; box-shadow: var(--shadow-md); }
.auth-dialog-icon { width: 40px; height: 40px; display: grid; place-items: center; border-radius: 8px; background: var(--primary-soft); color: var(--primary); }
.auth-dialog h2 { margin: 16px 0 6px; font-size: 18px; line-height: 1.35; }
.auth-dialog p { margin: 0 0 16px; color: var(--muted); }
.auth-dialog input { width: 100%; height: 42px; border: 1px solid var(--line-strong); border-radius: 6px; padding: 8px 10px; }
.auth-dialog .button { width: 100%; margin-top: 12px; }

@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 1080px) {
  .overview { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .fields { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .schedule-submit { width: 100%; }
}

@media (max-width: 720px) {
  .header-inner, .page { width: min(100% - 28px, 1280px); }
  .header-inner { min-height: 62px; }
  .brand-mark { width: 36px; height: 36px; }
  .brand-subtitle, .model-chip { display: none; }
  .page { padding-top: 18px; padding-bottom: 32px; }
  .overview { gap: 8px; }
  .metric { min-height: 132px; padding: 14px; }
  .metric-value { font-size: 25px; }
  .content-section { margin-top: 24px; }
  .section-head { align-items: center; }
  .fields { grid-template-columns: 1fr; }
  .schedule-panel-head { align-items: flex-start; }
  .next-run { max-width: 52%; }
  .responsive-table { overflow: visible; border: 0; background: transparent; box-shadow: none; }
  .responsive-table table, .responsive-table tbody, .responsive-table tr, .responsive-table td { display: block; width: 100%; min-width: 0; }
  .responsive-table thead { display: none; }
  .responsive-table tbody { display: grid; gap: 10px; }
  .responsive-table tr { overflow: hidden; border: 1px solid var(--line); border-radius: 8px; background: var(--surface); box-shadow: var(--shadow-sm); }
  .responsive-table tr:hover { background: var(--surface); }
  .responsive-table td { display: grid; grid-template-columns: 96px minmax(0, 1fr); gap: 12px; min-height: 44px; align-items: center; border-bottom: 1px solid #e8ebee; padding: 10px 12px; }
  .responsive-table td::before { content: attr(data-label); color: var(--muted); font-size: 11px; font-weight: 680; text-transform: uppercase; }
  .responsive-table td:last-child { border-bottom: 0; }
  .responsive-table .account-cell { min-width: 0; }
  .responsive-table .account-email, .responsive-table .code-value, .responsive-table .error-message { max-width: 100%; white-space: normal; overflow-wrap: anywhere; }
  .responsive-table .date { min-width: 0; white-space: normal; }
  .responsive-table td.empty { display: block; height: auto; min-height: 120px; padding: 38px 16px; }
  .responsive-table td.empty::before { display: none; }
}

@media (max-width: 430px) {
  .community-link { display: none; }
  .connection { width: 32px; justify-content: center; padding: 0; }
  .connection-label { display: none; }
  .metric-label { font-size: 12px; }
  .metric-icon { width: 28px; height: 28px; }
  .metric-value { font-size: 23px; }
  .metric-detail { font-size: 11px; }
  .section-head { align-items: flex-start; }
  .section-meta { max-width: 190px; }
  .button-label-mobile { display: none; }
  .run-button .button-label-mobile { display: inline; }
  .schedule-panel-head { display: block; }
  .next-run { max-width: none; margin-top: 7px; text-align: left; }
  .schedule-form { padding: 14px; }
  .responsive-table td { grid-template-columns: 82px minmax(0, 1fr); }
}
</style>
</head>
<body>
<svg class="icon-defs" aria-hidden="true">
  <symbol id="i-activity" viewBox="0 0 24 24"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></symbol>
  <symbol id="i-users" viewBox="0 0 24 24"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/></symbol>
  <symbol id="i-circle-check" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></symbol>
  <symbol id="i-triangle-alert" viewBox="0 0 24 24"><path d="m21.73 18-8-14a2 2 0 0 0-3.46 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4M12 17h.01"/></symbol>
  <symbol id="i-calendar-clock" viewBox="0 0 24 24"><path d="M21 7.5V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h3.5M16 2v4M8 2v4M3 10h5M17.5 17.5 16 16.3V14"/><circle cx="16" cy="16" r="6"/></symbol>
  <symbol id="i-refresh" viewBox="0 0 24 24"><path d="M20 6v5h-5M4 18v-5h5"/><path d="M18.49 9A7 7 0 0 0 5.64 6.64L4 11m16 2-1.64 4.36A7 7 0 0 1 5.51 15"/></symbol>
  <symbol id="i-play" viewBox="0 0 24 24"><path d="m6 3 14 9-14 9z"/></symbol>
  <symbol id="i-list-checks" viewBox="0 0 24 24"><path d="m3 7 2 2 4-4M3 17l2 2 4-4M13 6h8M13 12h8M13 18h8"/></symbol>
  <symbol id="i-history" viewBox="0 0 24 24"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5M12 7v5l3 2"/></symbol>
  <symbol id="i-save" viewBox="0 0 24 24"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2Z"/><path d="M17 21v-8H7v8M7 3v5h8"/></symbol>
  <symbol id="i-shield" viewBox="0 0 24 24"><path d="M20 13c0 5-3.5 7.5-8 9-4.5-1.5-8-4-8-9V5l8-3 8 3z"/><path d="m9 12 2 2 4-4"/></symbol>
  <symbol id="i-inbox" viewBox="0 0 24 24"><path d="M4 4h16l2 12H2z"/><path d="M2 16h5a3 3 0 0 0 6 0h9"/></symbol>
  <symbol id="i-loader" viewBox="0 0 24 24"><path d="M21 12a9 9 0 1 1-6.22-8.56"/></symbol>
  <symbol id="i-external-link" viewBox="0 0 24 24"><path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></symbol>
</svg>

<header class="app-header">
  <div class="header-inner">
    <div class="brand">
      <div class="brand-mark"><svg class="icon" aria-hidden="true"><use href="#i-activity"/></svg></div>
      <div class="brand-copy">
        <h1 class="brand-title">Codex Health Monitor</h1>
        <div class="brand-subtitle">Codex 账号健康监控</div>
      </div>
    </div>
    <div class="header-meta">
      <a class="community-link" href="https://linux.do/" target="_blank" rel="noopener noreferrer">LINUX DO<svg class="icon" aria-hidden="true"><use href="#i-external-link"/></svg></a>
      <div class="model-chip"><svg class="icon" aria-hidden="true"><use href="#i-shield"/></svg>gpt-5.6-luna</div>
      <div class="connection" id="connection" title="服务连接状态"><span class="connection-dot"></span><span class="connection-label" id="connectionLabel">连接中</span></div>
    </div>
  </div>
</header>

<main class="page">
  <section class="overview" aria-label="检测摘要">
    <article class="metric">
      <div>
        <div class="metric-head"><span class="metric-label">账号总数</span><span class="metric-icon"><svg class="icon" aria-hidden="true"><use href="#i-users"/></svg></span></div>
        <div class="metric-value" id="total">-</div>
      </div>
      <div class="metric-detail" id="lastRun">正在读取状态</div>
    </article>
    <article class="metric positive">
      <div>
        <div class="metric-head"><span class="metric-label">健康账号</span><span class="metric-icon"><svg class="icon" aria-hidden="true"><use href="#i-circle-check"/></svg></span></div>
        <div class="metric-value" id="healthy">-</div>
      </div>
      <div><div class="metric-detail" id="healthyDetail">等待检测结果</div><div class="metric-progress"><span id="healthProgress"></span></div></div>
    </article>
    <article class="metric negative">
      <div>
        <div class="metric-head"><span class="metric-label">异常账号</span><span class="metric-icon"><svg class="icon" aria-hidden="true"><use href="#i-triangle-alert"/></svg></span></div>
        <div class="metric-value" id="unhealthy">-</div>
      </div>
      <div class="metric-detail" id="unhealthyDetail">等待检测结果</div>
    </article>
    <article class="metric scheduled">
      <div>
        <div class="metric-head"><span class="metric-label">执行计划</span><span class="metric-icon"><svg class="icon" aria-hidden="true"><use href="#i-calendar-clock"/></svg></span></div>
        <div class="metric-value" id="mode">-</div>
      </div>
      <div class="metric-detail" id="nextRun">未安排</div>
    </article>
  </section>

  <section class="content-section" aria-labelledby="accountsTitle">
    <div class="section-head">
      <div class="section-title-wrap">
        <div class="section-title-row"><svg class="icon" aria-hidden="true"><use href="#i-list-checks"/></svg><h2 class="section-title" id="accountsTitle">账号状态</h2></div>
        <div class="section-meta" id="accountsMeta">正在同步账号</div>
      </div>
      <div class="actions">
        <button class="button icon-button" id="refresh" type="button" title="刷新数据" aria-label="刷新数据"><svg class="icon" aria-hidden="true"><use href="#i-refresh"/></svg></button>
        <button class="button primary run-button" id="run" type="button"><svg class="icon" aria-hidden="true"><use href="#i-play"/></svg><span class="button-label-mobile" id="runLabel">立即检测</span></button>
      </div>
    </div>
    <div class="table-wrap responsive-table">
      <table>
        <thead><tr><th>账号</th><th>Auth index</th><th>账号 ID</th><th>状态</th><th>HTTP</th><th>耗时</th><th>检测时间</th><th>错误原因</th></tr></thead>
        <tbody id="accounts"><tr><td colspan="8" class="empty"><span class="empty-state"><svg class="icon" aria-hidden="true"><use href="#i-loader"/></svg>正在读取账号</span></td></tr></tbody>
      </table>
    </div>
  </section>

  <section class="content-section schedule-panel" aria-labelledby="scheduleTitle">
    <div class="schedule-panel-head">
      <div class="schedule-title" id="scheduleTitle"><svg class="icon" aria-hidden="true"><use href="#i-calendar-clock"/></svg>自动检测</div>
      <div class="next-run"><span>下次执行</span><strong id="nextRunFull">未安排</strong></div>
    </div>
    <form class="schedule-form" id="scheduleForm">
      <div class="fields">
        <div class="field">
          <span class="field-label">调度模式</span>
          <div class="segment" role="radiogroup" aria-label="调度模式">
            <label><input type="radio" name="scheduleMode" value="interval" checked><span>按间隔</span></label>
            <label><input type="radio" name="scheduleMode" value="daily_times"><span>固定时间</span></label>
          </div>
        </div>
        <div class="field" id="intervalField"><label for="intervalMin">间隔（分钟）</label><input id="intervalMin" type="number" min="5" max="10080" required></div>
        <div class="field hidden" id="dailyField"><label for="dailyTimes">执行时间</label><input id="dailyTimes" placeholder="09:00, 18:00" inputmode="numeric"></div>
        <div class="field"><label for="timezone">时区</label><input id="timezone" list="timezoneOptions" autocomplete="off" required><datalist id="timezoneOptions"><option value="Asia/Shanghai"><option value="Asia/Hong_Kong"><option value="Asia/Tokyo"><option value="UTC"></datalist></div>
        <div class="field"><label for="timeoutSec">超时（秒）</label><input id="timeoutSec" type="number" min="5" max="120" required></div>
        <div class="field"><label for="targetEmails">指定邮箱</label><input id="targetEmails" type="text" placeholder="留空则检测全部" autocomplete="off" spellcheck="false"></div>
        <button class="button primary schedule-submit" id="saveSchedule" type="submit" title="保存检测计划" aria-label="保存检测计划"><svg class="icon" aria-hidden="true"><use href="#i-save"/></svg><span class="button-label-mobile" id="saveLabel">保存计划</span></button>
      </div>
    </form>
    <div class="schedule-alert" id="scheduleAlert"><svg class="icon" aria-hidden="true"><use href="#i-triangle-alert"/></svg><span id="scheduleAlertText"></span></div>
  </section>

  <section class="content-section" aria-labelledby="historyTitle">
    <div class="section-head">
      <div class="section-title-wrap">
        <div class="section-title-row"><svg class="icon" aria-hidden="true"><use href="#i-history"/></svg><h2 class="section-title" id="historyTitle">检测历史</h2></div>
        <div class="section-meta" id="historyMeta">最多保留 100 次运行记录</div>
      </div>
    </div>
    <div class="table-wrap responsive-table">
      <table>
        <thead><tr><th>运行时间</th><th>触发方式</th><th>账号</th><th>状态</th><th>HTTP</th><th>耗时</th><th>错误原因</th></tr></thead>
        <tbody id="history"><tr><td colspan="7" class="empty"><span class="empty-state"><svg class="icon" aria-hidden="true"><use href="#i-inbox"/></svg>暂无检测记录</span></td></tr></tbody>
      </table>
    </div>
  </section>
</main>

<div class="notice" id="notice" role="status" aria-live="polite"><svg class="icon" aria-hidden="true"><use id="noticeIcon" href="#i-circle-check"/></svg><span id="noticeText"></span></div>
<div class="auth-overlay" id="authOverlay">
  <form class="auth-dialog" id="authForm">
    <div class="auth-dialog-icon"><svg class="icon" aria-hidden="true"><use href="#i-shield"/></svg></div>
    <h2>管理员密钥已失效</h2>
    <p>请输入当前 Manager Server 管理员密钥。密钥仅保留在这个标签页中。</p>
    <label class="sr-only" for="adminKey">管理员密钥</label>
    <input id="adminKey" type="password" autocomplete="current-password" placeholder="Manager Server 管理员密钥" required>
    <button class="button primary" type="submit">重新连接</button>
  </form>
</div>

<script>
const API='/v0/management/plugins/codex-health-monitor';
const SESSION_KEY='codex-health-monitor:admin-key';
const el=id=>document.getElementById(id);
let loadInFlight=false;

function decodeManagerStorage(value){
  if(!value||!value.startsWith('enc::v1::'))return value;
  try{
    const encoded=atob(value.slice(9));
    const key=new TextEncoder().encode('cli-proxy-api-webui::secure-storage|'+location.host+'|'+navigator.userAgent);
    const decoded=new Uint8Array(encoded.length);
    for(let i=0;i<encoded.length;i++)decoded[i]=encoded.charCodeAt(i)^key[i%key.length];
    return new TextDecoder().decode(decoded);
  }catch(_){return ''}
}

function parseStored(value){
  let current=decodeManagerStorage(value);
  for(let i=0;i<3&&typeof current==='string';i++){
    try{current=JSON.parse(current)}catch(_){break}
  }
  return current;
}

function extractManagementKey(value){
  const parsed=parseStored(value);
  if(typeof parsed==='string')return parsed;
  if(!parsed||typeof parsed!=='object')return '';
  if(typeof parsed.managementKey==='string')return parsed.managementKey;
  if(parsed.state&&typeof parsed.state.managementKey==='string')return parsed.state.managementKey;
  if(typeof parsed.value==='string')return parsed.value;
  return '';
}

function managementKey(){
  const sessionKey=sessionStorage.getItem(SESSION_KEY)||'';
  if(sessionKey)return sessionKey;
  for(const name of ['cli-proxy-auth','managementKey']){
    const key=extractManagementKey(localStorage.getItem(name)||'');
    if(key)return key;
  }
  return '';
}

async function api(path,options={}){
  const headers={'Content-Type':'application/json',...(options.headers||{})};
  const key=managementKey();
  if(key)headers.Authorization='Bearer '+key;
  const response=await fetch(API+path,{...options,headers});
  let data={};
  try{data=await response.json()}catch(_){ }
  if(response.status===401){const error=new Error(data.error||data.message||'管理员密钥无效');error.auth=true;throw error}
  if(!response.ok)throw new Error(data.error||('HTTP '+response.status));
  return data;
}

function escapeHTML(value){return String(value??'').replace(/[&<>'"]/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[ch]))}
function dateText(value){if(!value||String(value).startsWith('0001-'))return '-';const date=new Date(value);return Number.isNaN(date.getTime())?'-':date.toLocaleString('zh-CN',{hour12:false})}
function statusLabel(value){const labels={healthy:'健康',unauthorized:'未授权',payment_required:'额度异常',forbidden:'账号被拒绝',rate_limited:'限流',upstream_error:'上游异常',network_error:'网络异常',timeout:'超时',response_error:'响应异常',credential_error:'凭证异常',request_error:'请求异常',unexpected_output:'输出异常',disabled:'已停用',not_checked:'未检测',checking:'检测中'};return labels[value]||value||'未知'}
function statusHTML(row){const cls=row.healthy?'healthy':(row.status==='checking'||row.status==='not_checked'?'checking':'bad');return '<span class="status '+cls+'"><span class="status-dot"></span>'+escapeHTML(statusLabel(row.status))+'</span>'}
function httpHTML(status){const value=Number(status)||0;if(!value)return '<span class="http-code">-</span>';const cls=value>=200&&value<300?'ok':'fail';return '<span class="http-code '+cls+'">'+value+'</span>'}
function latencyText(value){const latency=Number(value)||0;return latency>0?latency.toLocaleString('zh-CN')+' ms':'-'}
function emptyHTML(columns,message,icon){return '<tr><td colspan="'+columns+'" class="empty"><span class="empty-state"><svg class="icon" aria-hidden="true"><use href="#'+icon+'"/></svg>'+escapeHTML(message)+'</span></td></tr>'}

function showNotice(message,isError=false){
  el('noticeText').textContent=message;
  el('noticeIcon').setAttribute('href',isError?'#i-triangle-alert':'#i-circle-check');
  el('notice').className='notice show'+(isError?' error-notice':'');
  clearTimeout(window.noticeTimer);
  window.noticeTimer=setTimeout(()=>el('notice').className='notice',3500);
}

function requestAdminKey(message){if(message)showNotice(message,true);el('authOverlay').classList.add('show');setTimeout(()=>el('adminKey').focus(),0)}
function setConnection(state){el('connection').className='connection '+state;el('connectionLabel').textContent=state==='online'?'已连接':state==='offline'?'连接失败':'连接中'}
function setButtonLoading(button,loading,labelNode,loadingLabel,readyLabel,readyIcon){button.disabled=loading;button.classList.toggle('is-loading',loading);const use=button.querySelector('use');if(use)use.setAttribute('href',loading?'#i-loader':readyIcon);if(labelNode)labelNode.textContent=loading?loadingLabel:readyLabel}
function handleActionError(error){if(error.auth)requestAdminKey(error.message);else showNotice(error.message,true)}

function renderAccounts(accounts){
  el('accountsMeta').textContent=accounts.length+' 个 Codex 账号';
  el('accounts').innerHTML=accounts.length?accounts.map(account=>{
    const email=account.email||'-';
    const avatar=email==='-'?'?':email.trim().charAt(0);
    return '<tr>'+
      '<td class="account-cell" data-label="账号"><div class="account"><span class="account-avatar">'+escapeHTML(avatar)+'</span><span class="account-email">'+escapeHTML(email)+'</span></div></td>'+
      '<td data-label="Auth index"><span class="mono code-value" title="'+escapeHTML(account.auth_index||'')+'">'+escapeHTML(account.auth_index||'-')+'</span></td>'+
      '<td data-label="账号 ID"><span class="mono code-value" title="'+escapeHTML(account.account_id||'')+'">'+escapeHTML(account.account_id||'-')+'</span></td>'+
      '<td data-label="状态">'+statusHTML(account)+'</td>'+
      '<td data-label="HTTP">'+httpHTML(account.http_status)+'</td>'+
      '<td class="latency" data-label="耗时">'+latencyText(account.latency_ms)+'</td>'+
      '<td class="date" data-label="检测时间">'+dateText(account.checked_at)+'</td>'+
      '<td data-label="错误原因"><span class="error-message" title="'+escapeHTML(account.error_message||'')+'">'+escapeHTML(account.error_message||'-')+'</span></td>'+
    '</tr>';
  }).join(''):emptyHTML(8,'没有符合条件的 Codex 账号','i-inbox');
}

function renderHistory(history){
  const rows=[];
  history.forEach(run=>(run.accounts||[]).forEach(account=>rows.push(
    '<tr>'+
      '<td class="date" data-label="运行时间">'+dateText(run.started_at)+'</td>'+
      '<td data-label="触发方式">'+escapeHTML(run.trigger==='manual'?'手动':'定时')+'</td>'+
      '<td data-label="账号"><span class="account-email">'+escapeHTML(account.email||('#'+account.auth_index))+'</span></td>'+
      '<td data-label="状态">'+statusHTML(account)+'</td>'+
      '<td data-label="HTTP">'+httpHTML(account.http_status)+'</td>'+
      '<td class="latency" data-label="耗时">'+latencyText(account.latency_ms)+'</td>'+
      '<td data-label="错误原因"><span class="error-message" title="'+escapeHTML(account.error_message||'')+'">'+escapeHTML(account.error_message||'-')+'</span></td>'+
    '</tr>'
  )));
  el('history').innerHTML=rows.length?rows.join(''):emptyHTML(7,'暂无检测记录','i-inbox');
  el('historyMeta').textContent=history.length?history.length+' 次运行 · '+rows.length+' 条账号记录':'最多保留 100 次运行记录';
}

function selectedMode(){const checked=document.querySelector('input[name="scheduleMode"]:checked');return checked?checked.value:'interval'}
function toggleMode(){const daily=selectedMode()==='daily_times';el('dailyField').classList.toggle('hidden',!daily);el('intervalField').classList.toggle('hidden',daily);el('dailyTimes').required=daily;el('intervalMin').required=!daily}

function renderSchedule(data){
  const schedule=data.schedule||{};
  const mode=schedule.schedule_mode||'interval';
  const radio=document.querySelector('input[name="scheduleMode"][value="'+mode+'"]');
  if(radio)radio.checked=true;
  el('intervalMin').value=schedule.interval_min||30;
  el('dailyTimes').value=schedule.daily_times||'';
  el('timezone').value=schedule.timezone||'Asia/Shanghai';
  el('timeoutSec').value=schedule.timeout_sec||30;
  el('targetEmails').value=schedule.target_emails||'';
  toggleMode();
  el('mode').textContent=mode==='daily_times'?'固定时间':'每 '+(schedule.interval_min||30)+' 分钟';
  const next=data.next_run_at?dateText(data.next_run_at):'未安排';
  el('nextRun').textContent=next;
  el('nextRunFull').textContent=next;
  el('scheduleAlert').classList.toggle('show',!!data.schedule_error);
  el('scheduleAlertText').textContent=data.schedule_error||'';
}

function renderSummary(status,accounts){
  const total=accounts.length;
  const healthy=Number(status.healthy)||0;
  const unhealthy=Number(status.unhealthy)||0;
  el('total').textContent=total;
  el('healthy').textContent=healthy;
  el('unhealthy').textContent=unhealthy;
  el('unhealthy').classList.toggle('has-errors',unhealthy>0);
  el('lastRun').textContent=status.running?'检测正在进行':status.latest?'最近检测 '+dateText(status.latest.finished_at):'尚未执行检测';
  const rate=total?Math.round(healthy/total*100):0;
  el('healthyDetail').textContent=total?'健康率 '+rate+'%':'暂无账号';
  el('healthProgress').style.width=rate+'%';
  el('unhealthyDetail').textContent=unhealthy>0?'有 '+unhealthy+' 个账号需要处理':'当前没有异常';
}

async function load(){
  if(loadInFlight)return;
  loadInFlight=true;
  clearTimeout(window.pollTimer);
  setConnection('');
  setButtonLoading(el('refresh'),true,null,'','','#i-refresh');
  try{
    const [status,accounts,history,schedule]=await Promise.all([api('/status'),api('/accounts'),api('/history'),api('/schedule')]);
    el('authOverlay').classList.remove('show');
    setConnection('online');
    renderSummary(status,accounts.accounts||[]);
    renderAccounts(accounts.accounts||[]);
    renderHistory(history.history||[]);
    renderSchedule(schedule);
    setButtonLoading(el('run'),!!status.running,el('runLabel'),'检测中','立即检测','#i-play');
    if(status.running)window.pollTimer=setTimeout(load,1800);
  }catch(error){
    setConnection('offline');
    handleActionError(error);
  }finally{
    loadInFlight=false;
    setButtonLoading(el('refresh'),false,null,'','','#i-refresh');
  }
}

document.querySelectorAll('input[name="scheduleMode"]').forEach(node=>node.addEventListener('change',toggleMode));
el('refresh').addEventListener('click',load);
el('run').addEventListener('click',async()=>{
  setButtonLoading(el('run'),true,el('runLabel'),'检测中','立即检测','#i-play');
  try{await api('/run',{method:'POST',body:'{}'});showNotice('检测已启动');await load()}catch(error){handleActionError(error);setButtonLoading(el('run'),false,el('runLabel'),'检测中','立即检测','#i-play')}
});
el('scheduleForm').addEventListener('submit',async event=>{
  event.preventDefault();
  const payload={enabled:true,schedule_mode:selectedMode(),interval_min:Number(el('intervalMin').value||30),daily_times:el('dailyTimes').value,timezone:el('timezone').value.trim(),timeout_sec:Number(el('timeoutSec').value||30),target_emails:el('targetEmails').value};
  setButtonLoading(el('saveSchedule'),true,el('saveLabel'),'保存中','保存计划','#i-save');
  try{const data=await api('/schedule',{method:'POST',body:JSON.stringify(payload)});renderSchedule(data);showNotice('检测计划已保存')}catch(error){handleActionError(error)}finally{setButtonLoading(el('saveSchedule'),false,el('saveLabel'),'保存中','保存计划','#i-save')}
});
el('authForm').addEventListener('submit',event=>{event.preventDefault();const key=el('adminKey').value.trim();if(!key)return;sessionStorage.setItem(SESSION_KEY,key);el('adminKey').value='';load()});
load();
</script>
</body>
</html>`
