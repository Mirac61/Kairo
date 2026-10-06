// Rendert den Stand, den die Extension per postMessage schickt. Aktionen gehen als Nachricht zurück.
// @ts-check
const vscode = acquireVsCodeApi();
const saved = vscode.getState() ?? {};
/** @type {any} */
let state;
/** @type {ReturnType<typeof setInterval> | undefined} */
let tick;
const ui = { tab: saved.tab ?? "now", open: new Set(saved.open ?? ["ACTIVE"]), dirs: new Set(saved.dirs ?? []) };
const persist = () => vscode.setState({ tab: ui.tab, open: [...ui.open], dirs: [...ui.dirs] });
/** Geladene Ordnerinhalte, Schlüssel = Pfad relativ zum Dokumente-Ordner ("" = Wurzel). */
const dirs = new Map();
let docsRoot = "~/Documents";
const listDir = (/** @type {string} */ path) => vscode.postMessage({ cmd: "listDir", path });

const TABS = [
  ["now", "Jetzt"],
  ["today", "Heute"],
  ["projects", "Projekte"],
  ["stats", "Stats"],
  ["docs", "Dokumente"],
];
const GROUPS = [
  ["ACTIVE", "Aktiv"],
  ["PAUSED", "Pausiert"],
  ["DONE", "Archiv"],
];
const PALETTE = ["var(--accent)", "var(--success)", "var(--warning)", "#b180d7", "var(--muted)"];
const PRIO = { URGENT: "dringend", HIGH: "hoch" };
const icon = {
  play: '<svg viewBox="0 0 16 16" fill="currentColor"><path d="M4 2.5l9 5.5-9 5.5z"/></svg>',
  pause: '<svg viewBox="0 0 16 16" fill="currentColor"><path d="M4 2h3v12H4zM9 2h3v12H9z"/></svg>',
  check: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M2.5 8.5l3.5 3.5 7.5-8"/></svg>',
  chev: '<svg class="chev" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M6 4l4 4-4 4"/></svg>',
  folder: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M1.5 4h4.5l1.5 1.5h7v7.5h-13z"/></svg>',
  file: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M9 1.5H3.5v13h9V5zM9 1.5V5h3.5"/></svg>',
  finder: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M1.5 4h4.5l1.5 1.5h7v7.5h-13z"/><path d="M8 7.5v3.5M6.3 9.3L8 7.5l1.7 1.8"/></svg>',
  refresh: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M13.5 8a5.5 5.5 0 1 1-1.6-3.9M13.5 2.5v2.6h-2.6"/></svg>',
  link: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M6.5 9.5l3-3M7 4.5l1.5-1.5a2.5 2.5 0 0 1 3.5 3.5L10.5 8M9 11.5L7.5 13A2.5 2.5 0 0 1 4 9.5L5.5 8"/></svg>',
  window: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M9 3h4v4M13 3L7.5 8.5M11 9.5V13H3V5h3.5"/></svg>',
};

/** @param {unknown} s */
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
/** @param {number} m */
const hm = (m) => `${Math.floor(m / 60)}:${String(Math.round(m % 60)).padStart(2, "0")}`;
/** @param {number} ms */
function clock(ms) {
  const s = Math.max(0, Math.floor(ms / 1000));
  const pad = (/** @type {number} */ n) => String(n).padStart(2, "0");
  return `${Math.floor(s / 3600)}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}`;
}
const $ = (/** @type {string} */ id) => /** @type {HTMLElement} */ (document.getElementById(id));
/** @param {Date} d */
const dayKey = (d) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
/** @param {Date} d */
const startOfDay = (d) => new Date(d.getFullYear(), d.getMonth(), d.getDate());

// Gerüst einmal bauen; danach werden nur die Inhalte ersetzt, damit Eingaben im Formular erhalten bleiben.
document.getElementById("app").innerHTML = `
  <header class="head"><h1 class="fill trunc" id="title">Kairo</h1><span class="wsdot" id="ws"></span></header>
  <nav class="tabs" role="tablist">${TABS.map(([k, l]) => `<button class="tab" role="tab" data-tab="${k}">${l}</button>`).join("")}</nav>
  <div id="offline" class="offline" hidden></div>
  <section class="view" data-view="now" id="v-now"></section>
  <section class="view" data-view="today">
    <div><div class="row" style="margin-bottom:6px"><h2 class="fill">Heute</h2><span class="chip warn" id="today-count"></span></div>
      <div class="card list" id="today-list"></div></div>
    <form class="quickadd" id="quickadd" autocomplete="off">
      <div class="fill"><label for="qa">Neue Aufgabe für heute</label><input id="qa" placeholder="z. B. 30 min Sport" required></div>
      <button class="btn primary" type="submit">Hinzufügen</button>
    </form>
  </section>
  <section class="view" data-view="projects" id="v-projects"></section>
  <section class="view" data-view="stats" id="v-stats"></section>
  <section class="view" data-view="docs" id="v-docs"></section>`;

function showTab() {
  document.querySelectorAll(".tab").forEach((t) => t.setAttribute("aria-selected", String(/** @type {HTMLElement} */ (t).dataset.tab === ui.tab)));
  document.querySelectorAll(".view").forEach((v) => {
    /** @type {HTMLElement} */ (v).hidden = !state?.online || /** @type {HTMLElement} */ (v).dataset.view !== ui.tab;
  });
}

/** @param {any} t @param {string | undefined} runningId */
function taskRow(t, runningId) {
  const done = t.status === "COMPLETED";
  const time = t.planned_start_at ? new Date(t.planned_start_at).toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" }) : "";
  return `<div class="task ${done ? "done" : ""} ${t.id === runningId ? "running" : ""}">
    <input type="checkbox" class="task-check" ${done ? "checked" : ""} data-cmd="${done ? "reopen" : "complete"}" data-id="${esc(t.id)}" aria-label="Abhaken: ${esc(t.title)}">
    <div class="task-body">
      <div class="task-name trunc" title="${esc(t.title)}">${esc(t.title)}</div>
      <div class="task-meta">
        ${time ? `<span>${time}</span>` : ""}
        ${t.estimated_minutes ? `<span>${t.estimated_minutes} Min</span>` : ""}
        ${PRIO[t.priority] ? `<span class="chip warn">Priorität ${PRIO[t.priority]}</span>` : ""}
        ${t.status === "PAUSED" ? '<span class="chip">pausiert</span>' : ""}
        ${t.id === runningId ? '<span class="chip ok">läuft</span>' : ""}
      </div>
    </div>
    ${done || t.id === runningId ? "" : `<button class="startbtn" title="Timer starten" data-cmd="start" data-id="${esc(t.id)}">${icon.play}</button>`}
  </div>`;
}

/** Ressourcen und offene Tasks des erkannten Projekts. */
function projectCard() {
  if (!state.project) {
    return "";
  }
  const res = state.resources
    .map(
      (/** @type {any} */ r) => `<button class="doc row" style="--depth:0" data-cmd="openResource" data-id="${esc(r.id)}" title="${esc(r.label)}">
        <span class="doc-icon ${r.type === "FOLDER" ? "is-dir" : ""}">${r.type === "URL" ? icon.link : r.type === "FOLDER" ? icon.folder : icon.file}</span>
        <span class="fill trunc">${esc(r.label)}</span></button>`,
    )
    .join("");
  const tasks = state.projectTasks.map((/** @type {any} */ t) => taskRow(t, state.running?.task.id)).join("");
  return `<div class="card">
    <div class="row"><h2 class="fill trunc">${esc(state.project)}</h2><span class="chip">${state.projectTasks.length} offen</span></div>
    ${res ? `<div class="list docs">${res}</div>` : ""}
    ${tasks ? `<div class="list">${tasks}</div>` : '<p class="muted small">Keine offenen Tasks in diesem Projekt.</p>'}
  </div>`;
}

function renderNow() {
  const r = state.running;
  const pct = state.planned ? Math.min(100, (state.tracked / state.planned) * 100) : 0;
  const open = state.tasks.filter((/** @type {any} */ t) => t.status !== "COMPLETED").length;
  $("v-now").innerHTML = `${projectCard()}
    <div class="card">
      <div class="row"><h2 class="fill">Jetzt</h2>${r ? '<span class="live">läuft</span>' : ""}</div>
      ${
        r
          ? `<p class="timer-time" id="timer">${clock(Date.now() - Date.parse(r.startedAt))}</p>
             <p class="timer-task trunc" title="${esc(r.task.title)}">${esc(r.task.title)}</p>
             <div class="btnrow">
               <button class="btn" data-cmd="pause" data-id="${esc(r.task.id)}">${icon.pause}Pause</button>
               <button class="btn" data-cmd="complete" data-id="${esc(r.task.id)}">${icon.check}Fertig</button>
             </div>`
          : `<p class="muted" style="margin-top:8px">Kein Timer läuft. Starte eine Task unter <a href="#" data-tab="today">Heute</a>.</p>`
      }
    </div>
    <div class="statstrip">
      <div class="stat"><span class="stat-label">Erfasst</span><span class="stat-value">${hm(state.tracked)}</span></div>
      <div class="stat"><span class="stat-label">Geplant</span><span class="stat-value">${hm(state.planned)}</span></div>
      <div class="stat"><span class="stat-label">Offen</span><span class="stat-value">${open}</span></div>
    </div>
    ${state.planned ? `<div><div class="row small muted"><span class="fill">Tagesziel</span><span>${Math.round(pct)} %</span></div><div class="bar-track"><i style="width:${pct}%"></i></div></div>` : ""}`;
  if (r) {
    const start = Date.parse(r.startedAt);
    tick = setInterval(() => ($("timer").textContent = clock(Date.now() - start)), 1000);
  }
}

function renderToday() {
  const runningId = state.running?.task.id;
  const tasks = [...state.tasks].sort((/** @type {any} */ a, /** @type {any} */ b) => Number(a.status === "COMPLETED") - Number(b.status === "COMPLETED"));
  const open = tasks.filter((t) => t.status !== "COMPLETED").length;
  $("today-count").textContent = `${open} offen`;
  $("today-list").innerHTML = tasks.length
    ? tasks.map((t) => taskRow(t, runningId)).join("")
    : '<p class="empty">Nichts geplant. Lege unten eine Aufgabe an.</p>';
}

function renderProjects() {
  /** @param {any} p */
  const row = (p) => {
    const pct = p.total ? Math.round((p.done / p.total) * 100) : null;
    return `<div class="proj ${esc(p.status)} ${p.current ? "current" : ""}" title="${esc(p.description || p.name)}">
      <div class="row"><span class="dot"></span><span class="pname fill trunc">${esc(p.name)}</span>
        ${pct === null ? "" : `<span class="pct">${p.done}/${p.total}</span>`}
        ${p.hasFolder && !p.current ? `<button class="iconbtn" title="In neuem Fenster öffnen" data-cmd="openProject" data-id="${esc(p.id)}">${icon.window}</button>` : ""}
      </div>
      ${pct === null ? "" : `<div class="bar-track"><i style="width:${pct}%"></i></div>`}
    </div>`;
  };
  const groupOf = (/** @type {any} */ p) => (p.status === "ACTIVE" || p.status === "PAUSED" ? p.status : "DONE");
  // aktuelles Projekt zuerst, dann nach Fortschritt
  const sorted = [...state.projects].sort((a, b) => Number(b.current) - Number(a.current) || b.total - a.total || a.name.localeCompare(b.name, "de"));
  const groups = GROUPS.map(([key, label]) => {
    const list = sorted.filter((p) => groupOf(p) === key);
    const open = ui.open.has(key);
    return list.length
      ? `<div class="tree-group"><button class="tree-toggle" data-group="${key}" aria-expanded="${open}">${icon.chev}<span class="fill">${label}</span><span class="count">${list.length}</span></button>
         <div class="tree-body" ${open ? "" : "hidden"}>${list.map(row).join("")}</div></div>`
      : "";
  }).join("");
  $("v-projects").innerHTML = state.projects.length
    ? `<h2>Projekte</h2><div class="card list">${groups}</div>`
    : '<p class="empty">Noch keine Projekte. Lege sie in der WebUI an.</p>';
}

function renderStats() {
  const now = new Date();
  const today = startOfDay(now);
  /** Minuten pro Tag und pro Projekt, jeweils dem Starttag zugeordnet. */
  const perDay = new Map();
  const perProjectWeek = new Map();
  const monday = new Date(today);
  monday.setDate(today.getDate() - ((today.getDay() + 6) % 7));
  for (const e of state.entries) {
    const start = new Date(e.start);
    const min = ((e.end ? Date.parse(e.end) : now.getTime()) - start.getTime()) / 60000;
    const k = dayKey(start);
    perDay.set(k, (perDay.get(k) ?? 0) + min);
    if (start >= monday) {
      perProjectWeek.set(e.project ?? "", (perProjectWeek.get(e.project ?? "") ?? 0) + min);
    }
  }

  const days = [...Array(7)].map((_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return { d, min: perDay.get(dayKey(d)) ?? 0 };
  });
  const maxDay = Math.max(60, ...days.map((x) => x.min));
  const weekTotal = days.reduce((s, x) => s + x.min, 0);
  const week = days
    .map(
      ({ d, min }) => `<div class="col ${dayKey(d) === dayKey(today) ? "today" : ""}" title="${hm(min)} h">
        <div>${min ? `<i style="height:${(min / maxDay) * 100}%"></i>` : ""}</div>
        <span class="d">${d.toLocaleDateString("de-DE", { weekday: "short" }).slice(0, 2)}</span></div>`,
    )
    .join("");

  const names = new Map(state.projects.map((/** @type {any} */ p) => [p.id, p.name]));
  const dist = [...perProjectWeek.entries()].sort((a, b) => b[1] - a[1]);
  const top = dist.slice(0, 4);
  const rest = dist.slice(4).reduce((s, x) => s + x[1], 0);
  if (rest) top.push(["", rest]);
  const distRows = top
    .map(
      ([id, min], i) => `<div class="dist-row"><span class="trunc">${esc(names.get(id) ?? "Sonstiges")}</span>
        <span class="bar-track"><i style="width:${(min / weekTotal) * 100}%;background:${PALETTE[i]}"></i></span>
        <span class="val">${hm(min)}</span></div>`,
    )
    .join("");

  const first = new Date(monday);
  first.setDate(monday.getDate() - 7 * 11);
  const heat = [];
  for (const d = new Date(first); d <= today; d.setDate(d.getDate() + 1)) {
    const min = perDay.get(dayKey(d)) ?? 0;
    const lvl = min >= 300 ? 4 : min >= 180 ? 3 : min >= 60 ? 2 : min > 0 ? 1 : 0;
    heat.push(`<i class="${lvl ? `l${lvl}` : ""}" title="${d.toLocaleDateString("de-DE")}: ${hm(min)} h"></i>`);
  }

  $("v-stats").innerHTML = `
    <div class="card"><div class="row"><h2 class="fill">Diese Woche</h2><span class="chip">${hm(weekTotal)} h</span></div>
      <div class="week">${week}</div></div>
    <div class="card"><h2>Projekt-Verteilung</h2>
      ${distRows ? `<div class="dist">${distRows}</div>` : '<p class="empty">Diese Woche noch nichts erfasst.</p>'}</div>
    <div class="card"><div class="row"><h2 class="fill">Aktivität</h2><span class="chip">12 Wochen</span></div>
      <div class="heat">${heat.join("")}</div></div>`;
}

function renderDocs() {
  /** @param {string} rel @param {number} depth @returns {string} */
  const tree = (rel, depth) => {
    const entries = dirs.get(rel);
    if (!entries) {
      return `<div class="doc muted" style="--depth:${depth}">Lade …</div>`;
    }
    if (!entries.length) {
      return `<div class="doc muted" style="--depth:${depth}">leer</div>`;
    }
    return entries
      .map((/** @type {any} */ e) => {
        const path = rel ? `${rel}/${e.name}` : e.name;
        const open = e.dir && ui.dirs.has(path);
        return `<div class="doc row" style="--depth:${depth}" data-${e.dir ? "dir" : "file"}="${esc(path)}" title="${esc(path)}">
            ${e.dir ? icon.chev.replace('class="chev"', `class="chev ${open ? "open" : ""}"`) : '<span class="chev"></span>'}
            <span class="doc-icon ${e.dir ? "is-dir" : ""}">${e.dir ? icon.folder : icon.file}</span>
            <span class="fill trunc">${esc(e.name)}</span>
            <button class="iconbtn" title="Im Finder zeigen" data-reveal="${esc(path)}">${icon.finder}</button>
          </div>${open ? tree(path, depth + 1) : ""}`;
      })
      .join("");
  };
  $("v-docs").innerHTML = `<div class="row"><h2 class="fill trunc">${esc(docsRoot)}</h2>
      <button class="iconbtn always" title="Im Finder zeigen" data-reveal="">${icon.finder}</button>
      <button class="iconbtn always" title="Neu laden" data-docs-refresh>${icon.refresh}</button></div>
    <div class="card list docs">${tree("", 0)}</div>`;
}

function render() {
  clearInterval(tick);
  const off = $("offline");
  if (!state) {
    off.hidden = false;
    off.innerHTML = "<p>Verbinde …</p>";
    showTab();
    return;
  }
  $("ws").className = `wsdot ${state.online ? "" : "off"}`;
  $("ws").textContent = state.online ? `v${state.version}` : "offline";
  off.hidden = state.online;
  if (!state.online) {
    off.innerHTML = `<p>${esc(state.reason)}</p><button class="btn" data-cmd="refresh">Erneut versuchen</button>`;
  } else {
    $("title").textContent = state.project ?? "Kairo";
    renderNow();
    renderToday();
    renderProjects();
    renderStats();
    renderDocs();
  }
  showTab();
}

document.addEventListener("click", (e) => {
  const el = /** @type {HTMLElement} */ (e.target);
  const tab = /** @type {HTMLElement | null} */ (el.closest("[data-tab]"));
  if (tab) {
    e.preventDefault();
    ui.tab = tab.dataset.tab ?? "now";
    persist();
    showTab();
    return;
  }
  const reveal = /** @type {HTMLElement | null} */ (el.closest("[data-reveal]"));
  if (reveal) {
    vscode.postMessage({ cmd: "reveal", path: reveal.dataset.reveal });
    return;
  }
  if (el.closest("[data-docs-refresh]")) {
    dirs.clear();
    ["", ...ui.dirs].forEach(listDir);
    renderDocs();
    return;
  }
  const dir = /** @type {HTMLElement | null} */ (el.closest("[data-dir]"));
  if (dir) {
    const path = dir.dataset.dir ?? "";
    if (ui.dirs.has(path)) {
      ui.dirs.delete(path);
    } else {
      ui.dirs.add(path);
      if (!dirs.has(path)) {
        listDir(path);
      }
    }
    persist();
    renderDocs();
    return;
  }
  const file = /** @type {HTMLElement | null} */ (el.closest("[data-file]"));
  if (file) {
    vscode.postMessage({ cmd: "openFile", path: file.dataset.file });
    return;
  }
  const group = /** @type {HTMLElement | null} */ (el.closest("[data-group]"));
  if (group) {
    const key = group.dataset.group ?? "";
    ui.open.has(key) ? ui.open.delete(key) : ui.open.add(key);
    persist();
    renderProjects();
    return;
  }
  const b = /** @type {HTMLElement | null} */ (el.closest("[data-cmd]"));
  if (b) {
    vscode.postMessage({ cmd: b.dataset.cmd, id: b.dataset.id });
  }
});
$("quickadd").addEventListener("submit", (e) => {
  e.preventDefault();
  const input = /** @type {HTMLInputElement} */ ($("qa"));
  const raw = input.value.trim();
  if (!raw) {
    return;
  }
  // „30 min Sport“ oder „Sport 30m“ setzt die Schätzung
  const m = raw.match(/^(\d{1,3})\s*m(?:in)?\s+(.+)$/i) ?? raw.match(/^(.+?)\s+(\d{1,3})\s*m(?:in)?$/i);
  const [title, minutes] = !m ? [raw, 0] : /^\d/.test(m[1]) ? [m[2], Number(m[1])] : [m[1], Number(m[2])];
  vscode.postMessage({ cmd: "addTask", title, minutes });
  input.value = "";
});
window.addEventListener("message", (e) => {
  if ("dir" in e.data) {
    dirs.set(e.data.dir, e.data.entries);
    docsRoot = e.data.root;
    if (state?.online) {
      renderDocs();
    }
    return;
  }
  state = e.data;
  render();
});
render();
// Wurzel und zuletzt aufgeklappte Ordner laden; verschwundene Ordner liefern einfach „leer“.
["", ...ui.dirs].forEach(listDir);
