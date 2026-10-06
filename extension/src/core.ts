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
