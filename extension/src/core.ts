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
  local_path: string | null;
}

export interface Task {
  id: string;
  title: string;
  status: string;
  project_id: string | null;
}

export interface Today {
  tasks: Task[];
  active_tasks: Task[];
  running_time_entry: { task_id: string | null } | null;
}

const trimSlashes = (p: string): string => (p.length > 1 ? p.replace(/\/+$/, "") : p);

/** Projekt mit dem längsten local_path, der folder gleich ist oder umfasst. */
export function matchProject(projects: Project[], folder: string): Project | undefined {
  const f = trimSlashes(folder);
  let best: Project | undefined;
  let bestLen = -1;
  for (const p of projects) {
    if (!p.local_path) {
      continue;
    }
    const lp = trimSlashes(p.local_path);
    const covers = f === lp || f.startsWith(lp === "/" ? "/" : `${lp}/`);
    if (covers && lp.length > bestLen) {
      best = p;
      bestLen = lp.length;
    }
  }
  return best;
}

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
    throw new Error(`HTTP ${res.status}`);
  }
  return (res.status === 204 ? undefined : await res.json()) as T;
}
