import * as path from "node:path";
import * as vscode from "vscode";
import { getHealth, getProject, getResources, getTask, taskAction } from "./backendClient";
import { openResource, type ContextProvider } from "./contextView";
import { expandHome, parseUriTarget, pendingFresh, type Project } from "./core";

const PENDING = "pendingOpen";

const folderOf = (p: Project | undefined): string | undefined => (p?.local_path ? path.resolve(expandHome(p.local_path)) : undefined);
const inWorkspace = (dir: string): boolean => !!vscode.workspace.workspaceFolders?.some((f) => f.uri.fsPath === dir);

/** Neues Fenster, wenn schon ein Ordner offen ist, sonst dasselbe. */
const openFolder = (dir: string): Thenable<unknown> =>
  vscode.commands.executeCommand("vscode.openFolder", vscode.Uri.file(dir), { forceNewWindow: !!vscode.workspace.workspaceFolders?.length });

const fail = (what: string, err: unknown): void =>
  void vscode.window.showErrorMessage(`Kairo: ${what} fehlgeschlagen: ${err instanceof Error ? err.message : "Fehler"}`);

/** vscodium://kairo-local.kairo/start?task=<id> startet die Task und öffnet Projektordner und Task-Ressourcen; /open?project=<id> nur den Ordner. */
async function handle(context: vscode.ExtensionContext, p: ContextProvider, uri: vscode.Uri): Promise<void> {
  const target = parseUriTarget(uri.path, uri.query);
  if (!target) {
    void vscode.window.showErrorMessage(`Kairo: Unbekannter Link „${uri.path}${uri.query ? `?${uri.query}` : ""}“.`);
    return;
  }
  try {
    const health = await getHealth();
    if (!health.online) {
      throw new Error(`Backend nicht erreichbar (${health.reason}). Starte \`kairo\` im Terminal.`);
    }
    if (target.action === "open") {
      const project = await getProject(target.id);
      const dir = folderOf(project);
      if (!dir) {
        throw new Error(`„${project.name}“ hat keinen lokalen Ordner`);
      }
      if (!inWorkspace(dir)) {
        await openFolder(dir);
      }
      return;
    }
    const task = await getTask(target.id);
    const [project, resources] = await Promise.all([task.project_id ? getProject(task.project_id) : undefined, getResources({ task_id: task.id })]);
    await taskAction(task.id, "start"); // ausdrückliche Aktion: der Link kommt aus einem Klick in der WebUI
    void p.refresh();
    const dir = folderOf(project);
    if (dir && !inWorkspace(dir)) {
      // Der Ordnerwechsel aktiviert die Extension im neuen Fenster neu; resumePending öffnet dort die Ressourcen.
      if (resources.length) {
        await context.globalState.update(PENDING, { taskId: task.id, ts: Date.now() });
      }
      await openFolder(dir);
    } else {
      for (const r of resources) {
        await openResource(r);
      }
    }
  } catch (err) {
    fail(target.action === "start" ? "Task starten" : "Projekt öffnen", err);
  }
}

/** Öffnet die Task-Ressourcen nach dem Ordnerwechsel durch /start, wenn dieses Fenster zum Projekt der Task passt. Startet nie einen Timer. */
async function resumePending(context: vscode.ExtensionContext): Promise<void> {
  const pending = context.globalState.get<{ taskId: string; ts: number }>(PENDING);
  if (!pending) {
    return;
  }
  if (!pendingFresh(pending.ts, Date.now())) {
    await context.globalState.update(PENDING, undefined);
    return;
  }
  if (!vscode.workspace.workspaceFolders) {
    return;
  }
  try {
    const task = await getTask(pending.taskId);
    const dir = folderOf(task.project_id ? await getProject(task.project_id) : undefined);
    if (!dir || !inWorkspace(dir)) {
      return; // anderes Fenster; der Eintrag verfällt nach 60 s
    }
    await context.globalState.update(PENDING, undefined);
    for (const r of await getResources({ task_id: task.id })) {
      await openResource(r);
    }
  } catch (err) {
    fail("Ressourcen öffnen", err);
  }
}

export function registerUriHandler(context: vscode.ExtensionContext, p: ContextProvider): vscode.Disposable {
  void resumePending(context);
  return vscode.window.registerUriHandler({ handleUri: (uri) => handle(context, p, uri) });
}
