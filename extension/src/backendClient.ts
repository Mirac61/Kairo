import * as vscode from "vscode";
import { apiRequest, checkHealth, HealthResult, Project, readToken, Task, Today, TimeEntry } from "./core";

export { HealthResult, readToken };

export const DEFAULT_BACKEND_URL = "http://127.0.0.1:8742";

export function backendUrl(): string {
  return vscode.workspace.getConfiguration("kairo").get<string>("backendUrl", DEFAULT_BACKEND_URL);
}

export function getHealth(): Promise<HealthResult> {
  return checkHealth(backendUrl(), readToken());
}

export const getProjects = (): Promise<Project[]> => apiRequest(backendUrl(), readToken(), "/projects");

export const getToday = (): Promise<Today> => apiRequest(backendUrl(), readToken(), "/today");

export const taskAction = (id: string, action: "start" | "pause" | "complete"): Promise<unknown> =>
  apiRequest(backendUrl(), readToken(), `/tasks/${id}/${action}`, {
    method: "POST",
    body: action === "start" ? { source: "VSCODIUM" } : undefined,
  });

/** Pausiert den Timer einer Task; der Eintrag endet zum Zeitpunkt endedAt. */
export const pauseTaskAt = (id: string, endedAt: Date): Promise<unknown> =>
  apiRequest(backendUrl(), readToken(), `/tasks/${id}/pause`, { method: "POST", body: { ended_at: endedAt.toISOString() } });

export const getTasks = (): Promise<Task[]> => apiRequest(backendUrl(), readToken(), "/tasks");

export const getTimeEntries = (from: Date): Promise<TimeEntry[]> =>
  apiRequest(backendUrl(), readToken(), `/time-entries?from=${encodeURIComponent(from.toISOString())}`);

export const createTask = (body: Record<string, unknown>): Promise<Task> =>
  apiRequest(backendUrl(), readToken(), "/tasks", { method: "POST", body });

export const updateTask = (id: string, body: Record<string, unknown>): Promise<Task> =>
  apiRequest(backendUrl(), readToken(), `/tasks/${id}`, { method: "PATCH", body });
