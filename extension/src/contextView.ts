import { randomBytes } from "node:crypto";
import * as os from "node:os";
import * as path from "node:path";
import * as vscode from "vscode";
import {
  backendUrl,
  createTask,
  getHealth,
  getProjects,
  getTasks,
  getTimeEntries,
  getToday,
  HealthResult,
  readToken,
  taskAction,
  updateTask,
} from "./backendClient";
import { matchProject, Project, Task, TimeEntry, Today } from "./core";
import { LiveEvents } from "./liveEvents";

const STATS_DAYS = 84; // 12 Wochen für die Heatmap

interface State {
  health: HealthResult;
  project?: Project;
  projects: Project[];
  tasks: Task[];
  entries: TimeEntry[];
  today?: Today;
}

type Message =
  | { cmd: "refresh" | "openWeb" }
  | { cmd: "start" | "pause" | "complete" | "reopen" | "openProject"; id: string }
  | { cmd: "addTask"; title: string; minutes: number }
  | { cmd: "listDir" | "openFile" | "reveal"; path: string };

// Öffnet VSCodium nicht sinnvoll, daher mit der Standard-App des Systems.
const EXTERNAL = /\.(pdf|key|pages|numbers|docx?|pptx?|xlsx?|odt|zip|dmg|png|jpe?g|heic|mp4|mov)$/i;

function documentsRoot(): string {
  const dir = vscode.workspace.getConfiguration("kairo").get<string>("documentsFolder", "~/Documents");
  return dir.replace(/^~(?=$|\/)/, os.homedir());
}

/** Pfad relativ zum Dokumente-Ordner auflösen; alles außerhalb wird abgelehnt. */
function insideDocuments(rel: string): vscode.Uri | undefined {
  const root = documentsRoot();
  const full = path.resolve(root, rel);
  return full === root || full.startsWith(root + path.sep) ? vscode.Uri.file(full) : undefined;
}

export class ContextProvider implements vscode.WebviewViewProvider, vscode.Disposable {
  private view: vscode.WebviewView | undefined;
  private state: State | undefined;
  private checking = false;
  private readonly live = new LiveEvents(backendUrl, readToken, () => void this.refresh());

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
    view.onDidChangeVisibility(() => this.setVisible(view.visible));
    view.onDidDispose(() => {
      this.view = undefined;
      this.live.stop();
    });
    this.setVisible(view.visible);
  }

  private async handle(m: Message): Promise<void> {
    switch (m.cmd) {
      case "refresh":
        return this.refresh();
      case "openWeb":
        return void openWeb();
      case "openProject":
        return openProject(this.state?.projects.find((p) => p.id === m.id));
      case "addTask":
        return this.run(() =>
          createTask({
            title: m.title,
            estimated_minutes: m.minutes,
            status: "PLANNED",
            planned_date: this.state?.today?.date,
            project_id: this.state?.project?.id,
          }),
        );
      case "listDir":
        return this.listDir(m.path);
      case "openFile": {
        const uri = insideDocuments(m.path);
        return void (uri && (EXTERNAL.test(uri.path) ? vscode.env.openExternal(uri) : vscode.commands.executeCommand("vscode.open", uri)));
      }
      case "reveal": {
        const uri = insideDocuments(m.path);
        return void (uri && vscode.commands.executeCommand("revealFileInOS", uri));
      }
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
    void this.view?.webview.postMessage({
      online: s.health.online,
      reason: s.health.online ? undefined : s.health.reason,
      version: s.health.online ? s.health.version : undefined,
      project: s.project?.name,
      running: running && { task: running, startedAt: s.today?.running_time_entry?.started_at },
      tasks: s.today?.tasks ?? [],
      projects: s.projects.map((p) => {
        const own = s.tasks.filter((t) => t.project_id === p.id && t.status !== "CANCELLED");
        return {
          id: p.id,
          name: p.name,
          description: p.description,
          status: p.status,
          current: p.id === s.project?.id,
          hasFolder: !!p.local_path,
          total: own.length,
          done: own.filter((t) => t.status === "COMPLETED").length,
        };
      }),
      entries: s.entries.map((e) => ({ start: e.started_at, end: e.ended_at, project: e.project_id })),
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
        this.state = { health, projects: [], tasks: [], entries: [] };
      } else {
        const from = new Date();
        from.setHours(0, 0, 0, 0);
        from.setDate(from.getDate() - STATS_DAYS);
        const [projects, today, tasks, entries] = await Promise.all([getProjects(), getToday(), getTasks(), getTimeEntries(from)]);
        this.state = { health, today, project: this.detectProject(projects), projects, tasks, entries };
      }
    } catch (err) {
      this.state = { health: { online: false, reason: err instanceof Error ? err.message : "Fehler" }, projects: [], tasks: [], entries: [] };
    } finally {
      this.checking = false;
    }
    this.post();
  }

  act(id: string, action: "start" | "pause" | "complete"): Promise<void> {
    return this.run(() => taskAction(id, action));
  }

  /** Führt eine Änderung aus und lädt danach neu; das /ws-Ereignis lädt ebenfalls, aber so stimmt die Anzeige auch ohne /ws. */
  private async run(fn: () => Promise<unknown>): Promise<void> {
    try {
      await fn();
    } catch (err) {
      void vscode.window.showErrorMessage(`Kairo: ${err instanceof Error ? err.message : "Aktion fehlgeschlagen"}`);
    }
    await this.refresh();
  }

  /** Schickt den Inhalt eines Ordners (relativ zum Dokumente-Ordner) an die Webview. Ordner zuerst, ohne versteckte Dateien. */
  async listDir(rel: string): Promise<void> {
    const uri = insideDocuments(rel);
    let entries: { name: string; dir: boolean }[] = [];
    try {
      entries = uri
        ? (await vscode.workspace.fs.readDirectory(uri))
            .filter(([name]) => !name.startsWith("."))
            .map(([name, type]) => ({ name, dir: (type & vscode.FileType.Directory) !== 0 }))
            .sort((a, b) => Number(b.dir) - Number(a.dir) || a.name.localeCompare(b.name, "de"))
        : [];
    } catch {
      // nicht lesbar: leer anzeigen
    }
    void this.view?.webview.postMessage({ dir: rel, root: documentsRoot().replace(os.homedir(), "~"), entries });
  }

  /** Verbindet /ws nur, solange die Ansicht sichtbar ist. */
  setVisible(visible: boolean): void {
    if (visible) {
      void this.refresh();
      this.live.start();
    } else {
      this.live.stop();
    }
  }

  dispose(): void {
    this.live.stop();
  }

  /** Längster passender local_path über alle Workspace-Ordner. */
  private detectProject(projects: Project[]): Project | undefined {
    let best: Project | undefined;
    for (const folder of vscode.workspace.workspaceFolders ?? []) {
      const p = matchProject(projects, folder.uri.fsPath);
      if (p && (p.local_path?.length ?? 0) > (best?.local_path?.length ?? -1)) {
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
    return id ? [...(t?.tasks ?? []), ...(t?.active_tasks ?? [])].find((x) => x.id === id) : undefined;
  }
}

export function openWeb(): Thenable<boolean> {
  return vscode.env.openExternal(vscode.Uri.parse(backendUrl()));
}

/** Öffnet den Projektordner in einem neuen Fenster. */
async function openProject(p: Project | undefined): Promise<void> {
  if (!p?.local_path) {
    void vscode.window.showInformationMessage(`Kairo: „${p?.name ?? "Projekt"}“ hat keinen lokalen Ordner.`);
    return;
  }
  const uri = vscode.Uri.file(p.local_path.replace(/^~(?=$|\/)/, os.homedir()));
  await vscode.commands.executeCommand("vscode.openFolder", uri, { forceNewWindow: true });
}
