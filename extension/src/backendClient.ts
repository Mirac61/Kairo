import * as vscode from "vscode";
import { checkHealth, HealthResult, readToken } from "./core";

export { HealthResult, readToken };

export const DEFAULT_BACKEND_URL = "http://127.0.0.1:8742";

export function backendUrl(): string {
  return vscode.workspace.getConfiguration("kairo").get<string>("backendUrl", DEFAULT_BACKEND_URL);
}

export function getHealth(): Promise<HealthResult> {
  return checkHealth(backendUrl(), readToken());
}
