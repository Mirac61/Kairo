import { randomBytes } from "node:crypto";
import * as vscode from "vscode";
import {
  backendUrl,
  createTask,
  getHealth,
  getProjects,
  getResources,
  getReview,
  getTasks,
  getToday,
  HealthResult,
  readToken,
  taskAction,
  updateTask,
} from "./backendClient";
import { expandHome, joinUrl, matchProject, Project, Resource, Task, Today, todayList, weekRange } from "./core";
import { parseQuickAdd, taskBody } from "./quickAdd";
import { LiveEvents } from "./liveEvents";

export interface State {
  health: HealthResult;
  project?: Project;
  projects: Project[];
  tasks: Task[];
  weekMinutes: number;
  resources: Resource[];
  today?: Today;
}

type Message =
  | { cmd: "refresh" | "openWeb" | "openReview" | "linkWorkspace" }
  | { cmd: "start" | "pause" | "complete" | "reopen" | "openProject" | "openResource"; id: string }
  | { cmd: "addTask"; raw: string };

// Öffnet VSCodium nicht sinnvoll, daher mit der Standard-App des Systems.
const EXTERNAL = /\.(pdf|key|pages|numbers|docx?|pptx?|xlsx?|odt|zip|dmg|png|jpe?g|heic|mp4|mov)$/i;

export class ContextProvider implements vscode.WebviewViewProvider, vscode.Disposable {
  private view: vscode.WebviewView | undefined;
  state: State | undefined;
  private checking = false;
  private readonly live = new LiveEvents(backendUrl, readToken, () => void this.refresh());
  private readonly refreshed = new vscode.EventEmitter<void>();
  /** Feuert nach jedem Neuladen des Stands, auch bei verborgener Ansicht. */
  readonly onDidRefresh = this.refreshed.event;

  constructor(private readonly extensionUri: vscode.Uri) {}

  resolveWebviewView(view: vscode.WebviewView): void {
    this.view = view;
    const media = vscode.Uri.joinPath(this.extensionUri, "media");
    view.webview.options = { enableScripts: true, localResourceRoots: [media] };
    const nonce = randomBytes(16).toString("base64");
    const css = view.webview.asWebviewUri(vscode.Uri.joinPath(media, "view.css"));
    const js = view.webview.asWebviewUri(vscode.Uri.joinPath(media, "view.js"));
    view.webview.html = `<!doctype html><html lang="de"><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${view.webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}';">
<link rel="stylesheet" href="${css}"></head><body><main id="app"></main><script nonce="${nonce}" src="${js}"></script></body></html>`;
    view.webview.onDidReceiveMessage((m: Message) => void this.handle(m));
    view.onDidChangeVisibility(() => view.visible && void this.refresh());
    view.onDidDispose(() => (this.view = undefined));
    void this.refresh();
  }

  private async handle(m: Message): Promise<void> {
    switch (m.cmd) {
      case "refresh":
        return this.refresh();
      case "openWeb":
        return void openWeb();
      case "openReview":
        return void openWeb("/review");
      case "linkWorkspace":
        return void vscode.commands.executeCommand("kairo.linkWorkspace");
      case "openProject":
        return openProject(this.state?.projects.find((p) => p.id === m.id));
      case "addTask":
        return this.addTask(m.raw);
      case "openResource":
        return openResource(this.state?.resources.find((r) => r.id === m.id));
      case "reopen":
        return this.run(() => updateTask(m.id, { status: "PLANNED" }));
      default:
        return this.act(m.id, m.cmd);
    }
  }

  private post(): void {
    const s = this.state;
    if (!s) {
      return;
    }
    const running = this.runningTask();
    const projectOf = (id: string | null) => s.projects.find((p) => p.id === id);
    void this.view?.webview.postMessage({
      online: s.health.online,
      reason: s.health.online ? undefined : s.health.reason,
      date: s.today?.date,
      project: s.project?.name,
      projectId: s.project?.id,
      folder: vscode.workspace.workspaceFolders?.[0]?.name,
      projectTasks: s.tasks.filter((t) => t.project_id === s.project?.id && t.status !== "COMPLETED" && t.status !== "CANCELLED"),
      resources: s.resources.map((r) => ({ id: r.id, type: r.type, label: r.label || r.target })),
      running: running && {
        task: running,
        startedAt: s.today?.running_time_entry?.started_at,
        project: projectOf(running.project_id)?.name,
        color: projectOf(running.project_id)?.color,
        projectHasFolder: !!projectOf(running.project_id)?.local_path,
      },
      tasks: todayList(s.today).map((t) => ({ ...t, project: projectOf(t.project_id)?.name, color: projectOf(t.project_id)?.color })),
      projects: s.projects.map((p) => {
        const own = s.tasks.filter((t) => t.project_id === p.id && t.status !== "CANCELLED");
        return {
          open: own.filter((t) => t.status !== "COMPLETED").length,
          id: p.id,
          name: p.name,
          description: p.description,
          status: p.status,
          color: p.color,
          current: p.id === s.project?.id,
          hasFolder: !!p.local_path,
          total: own.length,
          done: own.filter((t) => t.status === "COMPLETED").length,
        };
      }),
      week: s.weekMinutes,
      tracked: s.today?.tracked_minutes ?? 0,
      planned: s.today?.planned_minutes ?? 0,
    });
  }

  async refresh(): Promise<void> {
    if (this.checking) {
      return;
    }
    this.checking = true;
    try {
      const health = await getHealth();
      if (!health.online) {
        this.state = { health, projects: [], tasks: [], resources: [], weekMinutes: 0 };
      } else {
        const { from, to } = weekRange(new Date());
        const [projects, today, tasks, review] = await Promise.all([getProjects(), getToday(), getTasks(), getReview(from, to)]);
        const project = this.detectProject(projects);
        const resources = project ? await getResources({ project_id: project.id }) : [];
        this.state = { health, today, project, projects, tasks, resources, weekMinutes: review.tracked_minutes };
      }
    } catch (err) {
      this.state = { health: { online: false, reason: err instanceof Error ? err.message : "Fehler" }, projects: [], tasks: [], resources: [], weekMinutes: 0 };
    } finally {
      this.checking = false;
    }
    this.post();
    this.refreshed.fire();
  }

  /** Legt eine Task an, standardmäßig für heute im erkannten Projekt; „Sport 30m !hoch @morgen #Uni“ überschreibt das (wie in der WebUI). */
  addTask(raw: string): Promise<void> {
    const s = this.state;
    const q = parseQuickAdd(raw, s?.projects.filter((p) => p.status !== "ARCHIVED") ?? [], s?.today?.date);
    if (!q.title) {
      return Promise.resolve();
    }
    return this.run(() => createTask({ title: q.title, status: "PLANNED", planned_date: s?.today?.date, project_id: s?.project?.id, ...taskBody(q, s?.today?.date) }));
  }

  act(id: string, action: "start" | "pause" | "complete"): Promise<void> {
    return this.run(() => taskAction(id, action));
  }

  /** Führt eine Änderung aus und lädt danach neu; das /ws-Ereignis lädt ebenfalls, aber so stimmt die Anzeige auch ohne /ws. */
  async run(fn: () => Promise<unknown>): Promise<void> {
    try {
      await fn();
    } catch (err) {
      void vscode.window.showErrorMessage(`Kairo: ${err instanceof Error ? err.message : "Aktion fehlgeschlagen"}`);
    }
    await this.refresh();
  }

  /** Verbindet /ws (neu); die Statusleiste braucht den Stand auch bei verborgener Ansicht. */
  start(): void {
    this.live.start();
  }

  dispose(): void {
    this.live.stop();
    this.refreshed.dispose();
  }

  /** Längster passender local_path über alle Workspace-Ordner. */
  private detectProject(projects: Project[]): Project | undefined {
    let best: Project | undefined;
    for (const folder of vscode.workspace.workspaceFolders ?? []) {
      const p = matchProject(projects, folder.uri.fsPath);
      const len = (x: Project | undefined) => (x?.local_path ? expandHome(x.local_path).length : -1);
      if (p && len(p) > len(best)) {
        best = p;
      }
    }
    return best;
  }

  private runningId(): string | null | undefined {
    return this.state?.today?.running_time_entry?.task_id;
  }

  /** Die Task, deren Timer gerade läuft. */
  runningTask(): Task | undefined {
    const t = this.state?.today;
    const id = this.runningId();
    return id ? [...(t?.tasks ?? []), ...(t?.active_tasks ?? []), ...(this.state?.tasks ?? [])].find((x) => x.id === id) : undefined;
  }
}

export function openWeb(path = ""): Thenable<boolean> {
  return vscode.env.openExternal(vscode.Uri.parse(joinUrl(backendUrl(), path)));
}

/** Öffnet den Projektordner in einem neuen Fenster. */
export async function openProject(p: Project | undefined): Promise<void> {
  if (!p?.local_path) {
    void vscode.window.showInformationMessage(`Kairo: „${p?.name ?? "Projekt"}“ hat keinen lokalen Ordner.`);
    return;
  }
  const uri = vscode.Uri.file(expandHome(p.local_path));
  await vscode.commands.executeCommand("vscode.openFolder", uri, { forceNewWindow: true });
}

/** URLs im Browser, Ordner im Finder, Dateien im Editor oder mit der Standard-App. */
export async function openResource(r: Resource | undefined): Promise<void> {
  if (!r) {
    return;
  }
  if (r.type === "URL") {
    const uri = vscode.Uri.parse(r.target);
    return void (/^https?$/.test(uri.scheme) && vscode.env.openExternal(uri));
  }
  const uri = vscode.Uri.file(expandHome(r.target));
  if (r.type === "FOLDER") {
    return void vscode.commands.executeCommand("revealFileInOS", uri);
  }
  await (EXTERNAL.test(uri.path) ? vscode.env.openExternal(uri) : vscode.commands.executeCommand("vscode.open", uri));
}
