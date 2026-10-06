import * as vscode from "vscode";
import { apiRequest, checkHealth, HealthResult, Project, readToken, Today } from "./core";

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
