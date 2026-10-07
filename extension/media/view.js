// Rendert den Stand, den die Extension per postMessage schickt. Aktionen gehen als Nachricht zurück.
// @ts-check
const vscode = acquireVsCodeApi();
const saved = vscode.getState() ?? {};
/** @type {any} */
let state;
/** @type {ReturnType<typeof setInterval> | undefined} */
let tick;
const TABS = [
  ["now", "Jetzt"],
  ["today", "Heute"],
  ["projects", "Projekte"],
];
const ui = { tab: TABS.some(([k]) => k === saved.tab) ? saved.tab : "now", open: new Set(saved.open ?? ["ACTIVE"]) };
const persist = () => vscode.setState({ tab: ui.tab, open: [...ui.open] });

const GROUPS = [
  ["ACTIVE", "Aktiv"],
  ["PAUSED", "Pausiert"],
  ["DONE", "Archiv"],
];
const PRIO = { URGENT: "dringend", HIGH: "hoch" };
const icon = {
  play: '<svg viewBox="0 0 16 16" fill="currentColor"><path d="M4 2.5l9 5.5-9 5.5z"/></svg>',
  pause: '<svg viewBox="0 0 16 16" fill="currentColor"><path d="M4 2h3v12H4zM9 2h3v12H9z"/></svg>',
  check: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M2.5 8.5l3.5 3.5 7.5-8"/></svg>',
  chev: '<svg class="chev" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M6 4l4 4-4 4"/></svg>',
  folder: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M1.5 4h4.5l1.5 1.5h7v7.5h-13z"/></svg>',
  file: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M9 1.5H3.5v13h9V5zM9 1.5V5h3.5"/></svg>',
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
  <section class="view" data-view="projects" id="v-projects"></section>`;

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
    return state.folder
      ? `<div class="card"><p class="muted">„${esc(state.folder)}“ gehört zu keinem Projekt.</p>
          <div class="btnrow"><button class="btn" data-cmd="linkWorkspace">${icon.link}Diesen Ordner mit Projekt verknüpfen</button></div></div>`
      : "";
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
    ${state.planned ? `<div><div class="row small muted"><span class="fill">Tagesziel</span><span>${Math.round(pct)} %</span></div><div class="bar-track"><i style="width:${pct}%"></i></div></div>` : ""}
    <div class="row small"><span class="fill"><span class="muted">Diese Woche</span> ${hm(state.week)}</span><a href="#" data-cmd="openReview">Rückblick</a></div>`;
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
    off.innerHTML = `<p>Backend nicht erreichbar. Starte <code>kairo</code> im Terminal oder richte Autostart mit <code>kairo install</code> ein.</p>
      <p class="small">${esc(state.reason)}</p><button class="btn" data-cmd="refresh">Erneut versuchen</button>`;
  } else {
    $("title").textContent = state.project ?? "Kairo";
    renderNow();
    renderToday();
    renderProjects();
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
    if (b.tagName === "A") {
      e.preventDefault(); // href="#" nicht folgen; Checkboxen dürfen normal schalten
    }
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
  vscode.postMessage({ cmd: "addTask", raw });
  input.value = "";
});
window.addEventListener("message", (e) => {
  state = e.data;
  render();
});
render();
