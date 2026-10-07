import * as vscode from "vscode";
import { apiRequest, checkHealth, HealthResult, Project, readToken, Resource, Task, Today } from "./core";

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

export const createProject = (body: Record<string, unknown>): Promise<Project> =>
  apiRequest(backendUrl(), readToken(), "/projects", { method: "POST", body });

export const updateProject = (id: string, body: Record<string, unknown>): Promise<Project> =>
  apiRequest(backendUrl(), readToken(), `/projects/${id}`, { method: "PATCH", body });

export const getResources = (filter: Record<string, string> = {}): Promise<Resource[]> => {
  const q = new URLSearchParams(filter).toString();
  return apiRequest(backendUrl(), readToken(), q ? `/resources?${q}` : "/resources");
};

export const getProject = (id: string): Promise<Project> => apiRequest(backendUrl(), readToken(), `/projects/${id}`);

export const getTask = (id: string): Promise<Task> => apiRequest(backendUrl(), readToken(), `/tasks/${id}`);

export const getTasks = (): Promise<Task[]> => apiRequest(backendUrl(), readToken(), "/tasks");

/** Erfasste Minuten im Zeitraum (YYYY-MM-DD, beide inklusive). */
export const getReview = (from: string, to: string): Promise<{ tracked_minutes: number }> =>
  apiRequest(backendUrl(), readToken(), `/review?from=${from}&to=${to}`);

export const createTask = (body: Record<string, unknown>): Promise<Task> =>
  apiRequest(backendUrl(), readToken(), "/tasks", { method: "POST", body });

export const updateTask = (id: string, body: Record<string, unknown>): Promise<Task> =>
  apiRequest(backendUrl(), readToken(), `/tasks/${id}`, { method: "PATCH", body });
