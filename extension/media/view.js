// Rendert den Stand, den die Extension per postMessage schickt. Aktionen gehen als Nachricht zurück.
// @ts-check
const vscode = acquireVsCodeApi();
const saved = vscode.getState() ?? {};
/** @type {any} */
let state;
let receivedAt = 0;
/** @type {ReturnType<typeof setInterval> | undefined} */
let tick;
const TABS = [
  ["today", "Heute"],
  ["projects", "Projekte"],
];
const DONE_KEY = "TODAY_DONE"; // Schlüssel in ui.open: die Liste der erledigten Tasks ist aufgeklappt
const ui = { tab: TABS.some(([k]) => k === saved.tab) ? saved.tab : "today", open: new Set(saved.open ?? ["ACTIVE"]), filter: "" };
const persist = () => vscode.setState({ tab: ui.tab, open: [...ui.open] });

const GROUPS = [
  ["ACTIVE", "Aktiv"],
  ["PAUSED", "Pausiert"],
  ["DONE", "Archiv"],
];
const PRIO = { URGENT: "dringend", HIGH: "hoch" };
const icon = {
  play: '<svg viewBox="0 0 10 10" fill="currentColor"><path d="M2.5 1.2v7.6L8.6 5z"/></svg>',
  pause: '<svg viewBox="0 0 10 10"><path d="M3 1.5v7M7 1.5v7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>',
  check: '<svg viewBox="0 0 10 10"><path d="M2 5.2 4.2 7.3 8 3" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  plus: '<svg viewBox="0 0 12 12"><path d="M6 1.5v9M1.5 6h9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  search: '<svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"><circle cx="5.2" cy="5.2" r="3.5"/><path d="m8 8 2.5 2.5"/></svg>',
  chev: '<svg class="chev" viewBox="0 0 10 10"><path d="M2.5 3.5 5 6l2.5-2.5" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>',
  flag: '<svg viewBox="0 0 9 10"><path d="M1.5 9.5V1h6L6 3.2 7.5 5.5h-6" fill="currentColor"/></svg>',
  folder: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M1.5 4h4.5l1.5 1.5h7v7.5h-13z"/></svg>',
  file: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M9 1.5H3.5v13h9V5zM9 1.5V5h3.5"/></svg>',
  link: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M6.5 9.5l3-3M7 4.5l1.5-1.5a2.5 2.5 0 0 1 3.5 3.5L10.5 8M9 11.5L7.5 13A2.5 2.5 0 0 1 4 9.5L5.5 8"/></svg>',
  window: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3"><path d="M9 3h4v4M13 3L7.5 8.5M11 9.5V13H3V5h3.5"/></svg>',
};

/** @param {unknown} s */
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
/** Minuten als h:mm. @param {number} m */
const hm = (m) => `${Math.floor(m / 60)}:${String(Math.round(m % 60)).padStart(2, "0")}`;
/** Laufzeit als m:ss, ab einer Stunde h:mm:ss. @param {number} ms */
function elapsed(ms) {
  const s = Math.max(0, Math.floor(ms / 1000));
  const pad = (/** @type {number} */ n) => String(n).padStart(2, "0");
  const h = Math.floor(s / 3600);
  return h ? `${h}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}` : `${Math.floor(s / 60)}:${pad(s % 60)}`;
}
const $ = (/** @type {string} */ id) => /** @type {HTMLElement} */ (document.getElementById(id));
const isOpen = (/** @type {any} */ t) => t.status !== "COMPLETED" && t.status !== "CANCELLED";
/** „Mi, 7. Okt“ aus YYYY-MM-DD. @param {string | undefined} day */
function dayLabel(day) {
  if (!day) {
    return "";
  }
  const d = new Date(`${day}T12:00:00`);
  const part = (/** @type {Intl.DateTimeFormatOptions} */ o) => d.toLocaleDateString("de-DE", o).replace(".", "");
  return `${part({ weekday: "short" })}, ${d.getDate()}. ${part({ month: "short" })}`;
}
/** Erfasste Minuten heute; ein laufender Timer zählt seit dem letzten Stand weiter. */
const liveDelta = () => (state?.running ? Math.floor((Date.now() - receivedAt) / 60000) : 0);

// Gerüst einmal bauen; danach werden nur die Inhalte ersetzt, damit Eingaben in den Feldern erhalten bleiben.
$("app").innerHTML = `
  <div id="offline" class="offline" hidden></div>
  <nav class="tabs" role="tablist" aria-label="Ansicht" id="tabs">${TABS.map(([k, l]) => `<button class="tab" role="tab" id="tab-${k}" aria-controls="v-${k}" data-tab="${k}">${l}</button>`).join("")}</nav>
  <section class="view" data-view="today" id="v-today" role="tabpanel" aria-labelledby="tab-today">
    <div class="scroll">
      <div id="t-timer"></div>
      <div id="t-day"></div>
      <div id="t-list" class="tasks"></div>
      <div id="t-project"></div>
    </div>
    <div class="dock">
      <form id="quickadd" autocomplete="off">
        <label class="field focusable">${icon.plus}<input id="qa" class="fill" placeholder="Aufgabe für heute …" aria-label="Neue Aufgabe für heute"><kbd aria-hidden="true">↵</kbd></label>
        <div class="hint">30m · !hoch · @morgen · #projekt</div>
      </form>
      <div class="row small week"><span class="muted">Woche</span><b id="week">0:00</b><span class="fill"></span><a href="#" data-cmd="openReview">Rückblick →</a></div>
    </div>
  </section>
  <section class="view" data-view="projects" id="v-projects" role="tabpanel" aria-labelledby="tab-projects">
    <div class="filterbar"><label class="field focusable">${icon.search}<input id="pf" class="fill" placeholder="Filtern" aria-label="Projekte filtern"></label></div>
    <div class="scroll" id="p-body"></div>
  </section>`;

function showTab() {
  const online = !!state?.online;
  $("tabs").hidden = !online;
  document.querySelectorAll(".tab").forEach((t) => {
    const on = /** @type {HTMLElement} */ (t).dataset.tab === ui.tab;
    t.setAttribute("aria-selected", String(on));
    /** @type {HTMLElement} */ (t).tabIndex = on ? 0 : -1; // Pfeiltasten wechseln, Tab springt aus der Leiste
  });
  document.querySelectorAll(".view").forEach((v) => {
    /** @type {HTMLElement} */ (v).hidden = !online || /** @type {HTMLElement} */ (v).dataset.view !== ui.tab;
  });
}

/** Eine Task-Zeile. sub = Unteraufgabe unter ihrer Aufgabe, running = Timer läuft für diese Task.
 * @param {any} t @param {{ sub?: boolean, running?: boolean }} [o] */
function taskRow(t, { sub = false, running = false } = {}) {
  const done = t.status === "COMPLETED";
  const time = t.planned_start_at ? new Date(t.planned_start_at).toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" }) : "";
  const parts = sub || done
    ? []
    : [
        t.project ? `<span class="pname-cell p-${esc(t.color)}"><span class="pdot"></span><span class="trunc">${esc(t.project)}</span></span>` : "",
        time ? `<span>${time}</span>` : "",
        t.estimated_minutes ? `<span>${t.estimated_minutes} min</span>` : "",
        PRIO[t.priority] ? `<span class="prio">${icon.flag}${PRIO[t.priority]}</span>` : "",
        t.overdue ? '<span class="late">überfällig</span>' : "",
        t.status === "PAUSED" ? "<span>pausiert</span>" : "",
      ].filter(Boolean);
  const right = running
    ? '<span class="run-time"><i class="live-dot s"></i><span data-live="time"></span></span>'
    : (sub || done) && t.estimated_minutes
      ? `<span class="small muted nowrap">${t.estimated_minutes} min</span>`
      : "";
  const play = done || running ? "" : `<button class="iconbtn play" title="Timer starten" aria-label="Timer starten: ${esc(t.title)}" data-cmd="start" data-id="${esc(t.id)}">${icon.play}</button>`;
  return `<div class="task ${done ? "done" : ""} ${running ? "running" : ""} ${sub ? "sub" : ""}">
    <input type="checkbox" class="task-check" ${done ? "checked" : ""} data-cmd="${done ? "reopen" : "complete"}" data-id="${esc(t.id)}" aria-label="${done ? "Wieder öffnen" : "Abhaken"}: ${esc(t.title)}">
    <div class="task-body">
      <div class="task-name clamp" title="${esc(t.title)}">${esc(t.title)}</div>
      ${parts.length ? `<div class="task-meta">${parts.join('<i class="sep">·</i>')}</div>` : ""}
    </div>
    ${right}${play}
  </div>`;
}

/** Offene Tasks, Unteraufgaben direkt unter ihrer Aufgabe (nur wenn diese ebenfalls in der Liste steht).
 * @param {any[]} list @param {string | undefined} runningId */
function taskTree(list, runningId) {
  const ids = new Set(list.map((t) => t.id));
  const kids = new Map();
  for (const t of list) {
    if (t.parent_task_id && ids.has(t.parent_task_id)) {
      kids.set(t.parent_task_id, [...(kids.get(t.parent_task_id) ?? []), t]);
    }
  }
  return list
    .filter((t) => !(t.parent_task_id && ids.has(t.parent_task_id)))
    .map((t) => taskRow(t, { running: t.id === runningId }) + (kids.get(t.id) ?? []).map((/** @type {any} */ c) => taskRow(c, { sub: true, running: c.id === runningId })).join(""))
    .join("");
}

/** Erledigte Tasks von heute: eingeklappt, damit ein versehentliches „Fertig“ noch zurückgenommen werden kann.
 * @param {any[]} done */
function doneBlock(done) {
  if (!done.length) {
    return "";
  }
  const open = ui.open.has(DONE_KEY);
  return `<button class="group-toggle" data-group="${DONE_KEY}" aria-expanded="${open}">${icon.chev}<span class="group-label">Erledigt</span><span class="count">${done.length}</span></button>
    <div ${open ? "" : "hidden"}>${done.map((t) => taskRow(t)).join("")}</div>`;
}

/** Ressourcen und weitere offene Tasks des erkannten Projekts (die für heute geplanten stehen schon oben). */
function projectBlock() {
  if (!state.project) {
    return state.folder
      ? `<div class="proj-sec"><p class="muted small">„${esc(state.folder)}“ gehört zu keinem Projekt.</p>
          <div class="btnrow"><button class="btn" data-cmd="linkWorkspace">${icon.link}Ordner mit Projekt verknüpfen</button></div></div>`
      : "";
  }
  const res = state.resources
    .map(
      (/** @type {any} */ r) => `<button class="doc" data-cmd="openResource" data-id="${esc(r.id)}" title="${esc(r.label)}">
        <span class="doc-icon ${r.type === "FOLDER" ? "is-dir" : ""}">${r.type === "URL" ? icon.link : r.type === "FOLDER" ? icon.folder : icon.file}</span>
        <span class="fill trunc">${esc(r.label)}</span></button>`,
    )
    .join("");
  const planned = new Set(state.tasks.map((/** @type {any} */ t) => t.id));
  const more = state.projectTasks.filter((/** @type {any} */ t) => !planned.has(t.id));
  const color = state.projects.find((/** @type {any} */ p) => p.id === state.projectId)?.color;
  return `<div class="proj-sec">
    <div class="proj-head p-${esc(color)}"><span class="pdot"></span><span class="proj-name trunc">${esc(state.project)}</span><span class="small muted nowrap">dieser Workspace</span><span class="fill"></span>
      <button class="iconbtn" data-local="focus" title="Aufgabe hinzufügen" aria-label="Aufgabe für ${esc(state.project)} hinzufügen">${icon.plus}</button></div>
    ${res ? `<div class="docs">${res}</div>` : ""}
    ${more.length ? `<div class="tasks">${taskTree(more, state.running?.task.id)}</div>` : '<p class="small muted proj-empty">Keine offenen Aufgaben</p>'}
  </div>`;
}

/** Läuft der Timer für ein anderes Projekt als den Workspace, steht oben ein Hinweis; sonst der Timer oder die nächste Aufgabe.
 * @param {any | undefined} next */
function timerBlock(next) {
  const r = state.running;
  if (!r) {
    return `<div class="next">
      <div class="fill next-text"><span class="small muted">Kein Timer${next ? " · Als Nächstes" : ""}</span><span class="trunc">${next ? esc(next.title) : "Nichts offen"}</span></div>
      ${next ? `<button class="round" title="Timer starten" aria-label="Timer starten: ${esc(next.title)}" data-cmd="start" data-id="${esc(next.id)}">${icon.play}</button>` : ""}
    </div>`;
  }
  const away = state.projectId && r.task.project_id !== state.projectId && r.projectHasFolder; // ohne Ordner gäbe es nichts zu tun; der Projektname steht schon im Timer
  const notice = away
    ? `<div class="notice" role="status"><p>Timer läuft für „${esc(r.project ?? r.task.title)}“, du bist in „${esc(state.project)}“.</p>
        <div class="btnrow"><button class="btn" data-cmd="openProject" data-id="${esc(r.task.project_id)}">${icon.window}Zu „${esc(r.project)}“ wechseln</button></div></div>`
    : "";
  const est = r.task.estimated_minutes;
  return `${notice}<div class="timer">
    <div class="row small muted"><i class="live-dot"></i><span>Läuft</span><span class="fill"></span>
      <span class="pname-cell p-${esc(r.color)}"><span class="pdot"></span><span class="trunc">${esc(r.project ?? "Ohne Projekt")}</span></span></div>
    <div class="timer-title clamp" title="${esc(r.task.title)}">${esc(r.task.title)}</div>
    <div class="timer-clock"><span data-live="time"></span>${est ? `<span class="small muted">/ ${est} min</span>` : ""}</div>
    ${est ? '<div class="bar"><i id="timer-bar"></i></div>' : ""}
    <div class="btns">
      <button class="btn" data-cmd="pause" data-id="${esc(r.task.id)}">${icon.pause}Pause</button>
      <button class="btn primary" data-cmd="complete" data-id="${esc(r.task.id)}">${icon.check}Fertig</button>
    </div>
  </div>`;
}

function renderToday() {
  const runningId = state.running?.task.id;
  // Überfällige zuerst, dann die Tasks des Projekts im Workspace, dann der Rest (stabil: sonst bleibt die Uhrzeit-Reihenfolge).
  const rank = (/** @type {any} */ t) => Number(!t.overdue) * 2 + Number(!state.projectId || t.project_id !== state.projectId);
  const open = state.tasks.filter(isOpen).sort((a, b) => rank(a) - rank(b));
  const done = state.tasks.filter((/** @type {any} */ t) => t.status === "COMPLETED");
  $("t-timer").innerHTML = timerBlock(open[0]);
  $("t-day").innerHTML = `<div class="day">
      <div class="row"><h2>Heute</h2><span class="small muted">${esc(dayLabel(state.date))}</span><span class="fill"></span><span class="small muted">${open.length} offen</span></div>
      <div class="bar"><i id="day-bar"></i></div>
      <div class="row small muted split"><span><b id="tracked"></b> erfasst</span><span><b>${hm(state.planned)}</b> geplant</span></div>
    </div>`;
  $("t-list").innerHTML = taskTree(open, runningId) + doneBlock(done);
  $("t-project").innerHTML = projectBlock();
  paintLive();
}

/** Zahlen und Balken, die mit dem laufenden Timer weiterzählen. */
function paintLive() {
  const r = state?.running;
  const now = Date.now();
  if (r) {
    const ms = now - Date.parse(r.startedAt);
    document.querySelectorAll("[data-live=time]").forEach((e) => (e.textContent = elapsed(ms)));
    const est = r.task.estimated_minutes;
    const bar = document.getElementById("timer-bar");
    if (bar && est) {
      bar.style.width = `${Math.min(100, ms / 60000 / est * 100)}%`;
    }
  }
  const tracked = state.tracked + liveDelta();
  const t = document.getElementById("tracked");
  if (t) {
    t.textContent = hm(tracked);
  }
  const day = document.getElementById("day-bar");
  if (day) {
    day.style.width = `${state.planned ? Math.min(100, tracked / state.planned * 100) : 0}%`;
  }
  $("week").textContent = hm(state.week + liveDelta());
}

function renderProjects() {
  const q = ui.filter.trim().toLowerCase();
  const match = (/** @type {any} */ p) => !q || p.name.toLowerCase().includes(q);
  const current = state.projects.find((/** @type {any} */ p) => p.current);
  /** @param {any} p */
  const row = (p) => `<div class="proj p-${esc(p.color)} ${p.status === "ACTIVE" ? "" : "inactive"}" title="${esc(p.description || p.name)}">
      <span class="pdot ${p.status === "ACTIVE" ? "" : "hollow"}"></span><span class="proj-name fill trunc">${esc(p.name)}</span>
      ${p.hasFolder ? `<button class="iconbtn reveal" title="In neuem Fenster öffnen" aria-label="${esc(p.name)} in neuem Fenster öffnen" data-cmd="openProject" data-id="${esc(p.id)}">${icon.window}</button>` : ""}
      <span class="count num">${p.open || ""}</span>
    </div>`;
  const groupOf = (/** @type {any} */ p) => (p.status === "ACTIVE" || p.status === "PAUSED" ? p.status : "DONE");
  // Projekte mit offenen Aufgaben zuerst (die meisten oben), dann alphabetisch; das aktuelle steht oben angeheftet.
  const sorted = state.projects
    .filter((/** @type {any} */ p) => !p.current && match(p))
    .sort((/** @type {any} */ a, /** @type {any} */ b) => b.open - a.open || a.name.localeCompare(b.name, "de"));
  const groups = GROUPS.map(([key, label]) => {
    const list = sorted.filter((p) => groupOf(p) === key);
    const total = state.projects.filter((/** @type {any} */ p) => groupOf(p) === key).length;
    const open = ui.open.has(key) || !!q;
    return list.length
      ? `<div class="group"><button class="group-toggle" data-group="${key}" aria-expanded="${open}">${icon.chev}<span class="group-label">${label}</span><span class="count">${q ? list.length : total}</span><span class="fill"></span>${key === "ACTIVE" ? '<span class="small muted">offen</span>' : ""}</button>
         <div class="group-body" ${open ? "" : "hidden"}>${list.map(row).join("")}</div></div>`
      : "";
  }).join("");
  const pin =
    current && match(current)
      ? `<div class="pin p-${esc(current.color)}"><div class="row"><span class="pdot"></span><span class="proj-name fill trunc">${esc(current.name)}</span><span class="small muted nowrap">dieser Workspace</span></div>
          ${current.total ? `<div class="row"><div class="bar fill done-bar"><i style="width:${Math.round((current.done / current.total) * 100)}%"></i></div><span class="small muted num">${current.done}/${current.total}</span></div>` : ""}</div>`
      : "";
  $("p-body").innerHTML = state.projects.length
    ? pin + groups || '<p class="empty">Keine Treffer.</p>'
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
  off.hidden = state.online;
  if (!state.online) {
    off.innerHTML = `<p>Backend nicht erreichbar. Starte <code>kairo</code> im Terminal oder richte Autostart mit <code>kairo install</code> ein.</p>
      <p class="small">${esc(state.reason)}</p><button class="btn" data-cmd="refresh">Erneut versuchen</button>`;
  } else {
    renderToday();
    renderProjects();
    if (state.running) {
      tick = setInterval(paintLive, 1000);
    }
  }
  showTab();
}

document.addEventListener("click", (e) => {
  const el = /** @type {HTMLElement} */ (e.target);
  const tab = /** @type {HTMLElement | null} */ (el.closest("[data-tab]"));
  if (tab) {
    e.preventDefault();
    ui.tab = tab.dataset.tab ?? "today";
    persist();
    showTab();
    return;
  }
  const group = /** @type {HTMLElement | null} */ (el.closest("[data-group]"));
  if (group) {
    const key = group.dataset.group ?? "";
    ui.open.has(key) ? ui.open.delete(key) : ui.open.add(key);
    persist();
    key === DONE_KEY ? renderToday() : renderProjects();
    return;
  }
  if (el.closest("[data-local=focus]")) {
    $("qa").focus();
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
document.querySelector(".tabs")?.addEventListener("keydown", (e) => {
  const step = { ArrowRight: 1, ArrowLeft: -1 }[/** @type {KeyboardEvent} */ (e).key];
  if (!step) {
    return;
  }
  e.preventDefault();
  const i = TABS.findIndex(([k]) => k === ui.tab);
  ui.tab = TABS[(i + step + TABS.length) % TABS.length][0];
  persist();
  showTab();
  $(`tab-${ui.tab}`).focus();
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
$("pf").addEventListener("input", (e) => {
  ui.filter = /** @type {HTMLInputElement} */ (e.target).value;
  if (state?.online) {
    renderProjects();
  }
});
window.addEventListener("message", (e) => {
  state = e.data;
  receivedAt = Date.now();
  render();
});
render();
