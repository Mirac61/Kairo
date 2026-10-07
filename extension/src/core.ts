// Reine Logik ohne vscode-API, damit sie separat testbar ist.
import { readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

export type HealthResult =
  | { online: true; version: string }
  | { online: false; reason: string };

export const HEALTH_TIMEOUT_MS = 3000;

export function joinUrl(base: string, path: string): string {
  return `${base.replace(/\/+$/, "")}/${path.replace(/^\/+/, "")}`;
}

export function parseHealth(body: unknown): HealthResult {
  if (typeof body === "object" && body !== null) {
    const rec = body as Record<string, unknown>;
    if (rec.status === "ok") {
      const version = typeof rec.version === "string" ? rec.version : "?";
      return { online: true, version };
    }
  }
  return { online: false, reason: "Unerwartete Antwort" };
}

export function readToken(path: string = join(homedir(), ".config", "kairo", "token")): string | undefined {
  try {
    const token = readFileSync(path, "utf8").trim();
    return token === "" ? undefined : token;
  } catch {
    return undefined;
  }
}

export function authHeaders(token: string | undefined): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function checkHealth(baseUrl: string, token?: string): Promise<HealthResult> {
  try {
    const res = await fetch(joinUrl(baseUrl, "/api/health"), {
      headers: authHeaders(token),
      signal: AbortSignal.timeout(HEALTH_TIMEOUT_MS),
    });
    if (!res.ok) {
      return { online: false, reason: `HTTP ${res.status}` };
    }
    return parseHealth(await res.json());
  } catch (err) {
    return { online: false, reason: err instanceof Error ? err.message : "Unbekannter Fehler" };
  }
}

export interface Project {
  id: string;
  name: string;
  description: string;
  status: string;
  local_path: string | null;
  color?: string;
}

export interface Task {
  id: string;
  title: string;
  status: string;
  priority: string;
  estimated_minutes: number;
  planned_start_at: string | null;
  project_id: string | null;
  parent_task_id?: string | null;
}

export interface Resource {
  id: string;
  type: "FILE" | "FOLDER" | "URL";
  target: string;
  label: string;
  task_id: string | null;
  project_id: string | null;
}

export interface Today {
  date: string;
  tasks: Task[];
  overdue: Task[];
  active_tasks: Task[];
  running_time_entry: { task_id: string | null; started_at: string } | null;
  planned_minutes: number;
  tracked_minutes: number;
}

/** Liste „Heute“: überfällige offene Tasks zuerst (markiert), dann die für heute geplanten, auch die erledigten. */
export function todayList(t: Today | undefined): (Task & { overdue?: boolean })[] {
  return t ? [...t.overdue.map((x) => ({ ...x, overdue: true })), ...t.tasks] : [];
}

const trimSlashes = (p: string): string => (p.length > 1 ? p.replace(/\/+$/, "") : p);

/** Ersetzt ein führendes "~" durch das Home-Verzeichnis. */
export const expandHome = (p: string, home: string = homedir()): string => p.replace(/^~(?=$|\/)/, home);

/** Projekt mit dem längsten local_path, der folder gleich ist oder umfasst. "~" in local_path steht für home. */
export function matchProject(projects: Project[], folder: string, home: string = homedir()): Project | undefined {
  const f = trimSlashes(folder);
  let best: Project | undefined;
  let bestLen = -1;
  for (const p of projects) {
    if (!p.local_path) {
      continue;
    }
    const lp = trimSlashes(expandHome(p.local_path, home));
    const covers = f === lp || f.startsWith(lp === "/" ? "/" : `${lp}/`);
    if (covers && lp.length > bestLen) {
      best = p;
      bestLen = lp.length;
    }
  }
  return best;
}

/** Meldung aus dem Backend-Body `{"error": …}`, sonst "HTTP <status>". */
export function errorMessage(status: number, body: string): string {
  try {
    const e = (JSON.parse(body) as { error?: unknown }).error;
    if (typeof e === "string" && e !== "") {
      return e;
    }
  } catch {
    // kein JSON
  }
  return `HTTP ${status}`;
}

/** Laufzeit als m:ss, ab einer Stunde h:mm:ss. */
export function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const pad = (n: number) => String(n).padStart(2, "0");
  const h = Math.floor(s / 3600);
  return h ? `${h}:${pad(Math.floor(s / 60) % 60)}:${pad(s % 60)}` : `${Math.floor(s / 60)}:${pad(s % 60)}`;
}

/** Offene Tasks: heutige zuerst, dann die des Projekts, dann der Rest; sonst bleibt die Reihenfolge. */
export function startable(tasks: Task[], todayIds: Set<string>, projectId?: string): Task[] {
  const rank = (t: Task) => (todayIds.has(t.id) ? 0 : projectId && t.project_id === projectId ? 1 : 2);
  return tasks.filter((t) => t.status !== "COMPLETED" && t.status !== "CANCELLED").sort((a, b) => rank(a) - rank(b));
}

/** Projekt einer Ressource: direkt oder über ihre Task. */
export const resourceProject = (r: Resource, tasks: Task[]): string | null => r.project_id ?? tasks.find((t) => t.id === r.task_id)?.project_id ?? null;

/** Ressourcen des Projekts zuerst, sonst bleibt die Reihenfolge. */
export function resourcesFirst(resources: Resource[], tasks: Task[], projectId?: string): Resource[] {
  const own = (r: Resource) => !!projectId && resourceProject(r, tasks) === projectId;
  return [...resources.filter(own), ...resources.filter((r) => !own(r))];
}

/** Montag bis Sonntag der Woche von now (lokale Daten) als YYYY-MM-DD. */
export function weekRange(now: Date): { from: string; to: string } {
  const day = (offset: number) => {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + offset);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  };
  const sinceMonday = (now.getDay() + 6) % 7;
  return { from: day(-sinceMonday), to: day(6 - sinceMonday) };
}

/** Ziel eines kairo-URI: /start?task=<id> oder /open?project=<id>. */
export function parseUriTarget(path: string, query: string): { action: "start" | "open"; id: string } | undefined {
  const action = { "/start": "start", "/open": "open" }[path.replace(/\/+$/, "")] as "start" | "open" | undefined;
  const id = new URLSearchParams(query).get(action === "start" ? "task" : "project");
  return action && id ? { action, id } : undefined;
}

export const PENDING_MAX_MS = 60_000;

/** Ob ein vorgemerktes Öffnen (ts) noch jünger als PENDING_MAX_MS ist. */
export const pendingFresh = (ts: number, now: number): boolean => now - ts < PENDING_MAX_MS;

/** Ruft die API auf und wirft bei einem Fehlerstatus. */
export async function apiRequest<T>(
  baseUrl: string,
  token: string | undefined,
  path: string,
  init: { method?: string; body?: unknown } = {},
): Promise<T> {
  const res = await fetch(joinUrl(baseUrl, `/api${path}`), {
    method: init.method ?? "GET",
    headers: {
      ...authHeaders(token),
      ...(init.body !== undefined ? { "Content-Type": "application/json" } : {}),
    },
    body: init.body !== undefined ? JSON.stringify(init.body) : undefined,
    signal: AbortSignal.timeout(HEALTH_TIMEOUT_MS),
  });
  if (!res.ok) {
    throw new Error(errorMessage(res.status, await res.text()));
  }
  return (res.status === 204 ? undefined : await res.json()) as T;
}

/** Ob seit lastActivity mindestens thresholdMs vergangen sind. threshold <= 0 schaltet ab. */
export function isIdle(lastActivity: number, now: number, thresholdMs: number): boolean {
  return thresholdMs > 0 && now - lastActivity >= thresholdMs;
}
