import * as vscode from "vscode";
import { createProject, getResources, updateProject } from "./backendClient";
import { openProject, openResource, type ContextProvider, type State } from "./contextView";
import { formatElapsed, resourceProject, resourcesFirst, startable, type Project, type Task } from "./core";

/** Der Stand, sobald das Backend online ist; sonst ein Hinweis. */
function ready(p: ContextProvider): State | undefined {
  if (!p.state?.health.online) {
    void vscode.window.showWarningMessage("Kairo: Backend nicht erreichbar. Starte `kairo` im Terminal.");
    return undefined;
  }
  return p.state;
}

/** Verknüpft den ersten Workspace-Ordner mit einem Projekt ohne Ordner oder legt ein neues an. */
async function linkWorkspace(p: ContextProvider): Promise<void> {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    void vscode.window.showInformationMessage("Kairo: Öffne zuerst einen Ordner.");
    return;
  }
  const s = ready(p);
  if (!s) {
    return;
  }
  const items: (vscode.QuickPickItem & { project?: Project })[] = [
    ...s.projects.filter((x) => !x.local_path).map((x) => ({ label: x.name, description: x.description, project: x })),
    { label: `$(add) Neues Projekt „${folder.name}“` },
  ];
  const pick = await vscode.window.showQuickPick(items, { placeHolder: `„${folder.name}“ verknüpfen mit …` });
  if (pick) {
    const local_path = folder.uri.fsPath;
    await p.run(() => (pick.project ? updateProject(pick.project.id, { local_path }) : createProject({ name: folder.name, local_path })));
  }
}

/** Startet die gewählte Task; läuft schon ein Timer, wechselt das Backend selbst. */
async function startTask(p: ContextProvider): Promise<void> {
  const s = ready(p);
  if (!s) {
    return;
  }
  const todayIds = new Set([...(s.today?.tasks ?? []), ...(s.today?.active_tasks ?? [])].map((t) => t.id));
  const names = new Map(s.projects.map((x) => [x.id, x.name] as const));
  const items = startable(s.tasks, todayIds, s.project?.id)
    .filter((t) => t.id !== p.runningTask()?.id)
    .map((t) => ({ label: t.title, description: names.get(t.project_id ?? ""), task: t }));
  if (!items.length) {
    void vscode.window.showInformationMessage("Kairo: Keine offenen Tasks.");
    return;
  }
  const pick = await vscode.window.showQuickPick(items, { placeHolder: "Task starten …", matchOnDescription: true });
  if (pick) {
    await p.act(pick.task.id, "start");
  }
}

/** Pausiert bzw. schließt die Task mit laufendem Timer ab. */
function onRunning(p: ContextProvider, action: "pause" | "complete"): Promise<void> | undefined {
  const t: Task | undefined = p.runningTask();
  if (!t) {
    void vscode.window.showInformationMessage("Kairo: Es läuft kein Timer.");
    return undefined;
  }
  return p.act(t.id, action);
}

async function newTask(p: ContextProvider): Promise<void> {
  const project = p.state?.project?.name;
  const raw = await vscode.window.showInputBox({
    prompt: project ? `Neue Task für heute in „${project}“` : "Neue Task für heute",
    placeHolder: "z. B. 30 min Sport",
  });
  if (raw?.trim()) {
    await p.addTask(raw);
  }
}

async function pickProject(p: ContextProvider): Promise<void> {
  const items = (p.state?.projects ?? []).filter((x) => x.local_path).map((x) => ({ label: x.name, description: x.local_path ?? "", project: x }));
  if (!items.length) {
    void vscode.window.showInformationMessage("Kairo: Kein Projekt hat einen lokalen Ordner.");
    return;
  }
  const pick = await vscode.window.showQuickPick(items, { placeHolder: "Projekt in neuem Fenster öffnen …", matchOnDescription: true });
  if (pick) {
    await openProject(pick.project);
  }
}

/** QuickPick über alle Ressourcen aller Projekte und Tasks; die des erkannten Projekts stehen oben. */
async function pickResource(p: ContextProvider): Promise<void> {
  const s = ready(p);
  if (!s) {
    return;
  }
  const resources = await getResources().catch((err: unknown) => {
    void vscode.window.showErrorMessage(`Kairo: ${err instanceof Error ? err.message : "Ressourcen laden fehlgeschlagen"}`);
    return undefined;
  });
  if (!resources?.length) {
    resources && void vscode.window.showInformationMessage("Kairo: Keine Ressourcen vorhanden.");
    return;
  }
  const names = new Map(s.projects.map((x) => [x.id, x.name] as const));
  const items = resourcesFirst(resources, s.tasks, s.project?.id).map((r) => ({
    label: r.label || r.target,
    description: [names.get(resourceProject(r, s.tasks) ?? ""), s.tasks.find((t) => t.id === r.task_id)?.title].filter(Boolean).join(" › "),
    resource: r,
  }));
  const pick = await vscode.window.showQuickPick(items, { placeHolder: "Ressource öffnen …", matchOnDescription: true });
  if (pick) {
    await openResource(pick.resource);
  }
}

/** Menü hinter dem Statusleisten-Eintrag. */
async function statusMenu(p: ContextProvider): Promise<void> {
  const running = p.runningTask();
  const items = [
    ...(running
      ? [
          { label: "$(debug-pause) Pause", command: "kairo.pauseTask" },
          { label: "$(check) Fertig", command: "kairo.completeTask" },
          { label: "$(arrow-swap) Task wechseln …", command: "kairo.startTask" },
        ]
      : [{ label: "$(play) Task starten …", command: "kairo.startTask" }]),
    { label: "$(link-external) Kairo öffnen", command: "workbench.view.extension.kairo" },
  ];
  const pick = await vscode.window.showQuickPick(items, { placeHolder: running?.title ?? "Kairo" });
  if (pick) {
    await vscode.commands.executeCommand(pick.command);
  }
}

/** Links in der Statusleiste: Laufzeit aus dem Startzeitstempel des Backends, jede Sekunde neu gezeichnet. */
function statusBar(p: ContextProvider): vscode.Disposable {
  const item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
  item.command = "kairo.statusMenu";
  let tick: NodeJS.Timeout | undefined;
  const paint = () => {
    clearInterval(tick);
    const task = p.runningTask();
    const since = p.state?.today?.running_time_entry?.started_at;
    if (task && since) {
      const start = Date.parse(since);
      const draw = () => (item.text = `$(clock) ${formatElapsed(Date.now() - start)} · ${task.title}`);
      draw();
      tick = setInterval(draw, 1000);
    } else {
      item.text = "$(play) Kairo";
    }
  };
  paint();
  item.show();
  return vscode.Disposable.from(item, p.onDidRefresh(paint), { dispose: () => clearInterval(tick) });
}

export function registerCommands(p: ContextProvider): vscode.Disposable {
  const cmd = vscode.commands.registerCommand;
  return vscode.Disposable.from(
    cmd("kairo.linkWorkspace", () => linkWorkspace(p)),
    cmd("kairo.startTask", () => startTask(p)),
    cmd("kairo.pauseTask", () => onRunning(p, "pause")),
    cmd("kairo.completeTask", () => onRunning(p, "complete")),
    cmd("kairo.newTask", () => newTask(p)),
    cmd("kairo.openProject", () => pickProject(p)),
    cmd("kairo.openResource", () => pickResource(p)),
    cmd("kairo.statusMenu", () => statusMenu(p)),
    statusBar(p),
  );
}
