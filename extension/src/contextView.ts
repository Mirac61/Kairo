import * as vscode from "vscode";
import { backendUrl, getHealth, getProjects, getToday, HealthResult, readToken, taskAction } from "./backendClient";
import { matchProject, Project, Task, Today } from "./core";
import { LiveEvents } from "./liveEvents";

interface State {
  health: HealthResult;
  project?: Project;
  today?: Today;
}

export class TaskItem extends vscode.TreeItem {
  constructor(
    readonly task: Task,
    running: boolean,
  ) {
    super(task.title);
    this.contextValue = running ? "task.running" : "task";
    this.iconPath = new vscode.ThemeIcon(running ? "debug-pause" : "circle-outline");
    this.description = task.status === "PAUSED" ? "pausiert" : undefined;
  }
}

export class ContextProvider implements vscode.TreeDataProvider<vscode.TreeItem>, vscode.Disposable {
  private readonly emitter = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.emitter.event;
  private state: State | undefined;
  private checking = false;
  private readonly live = new LiveEvents(backendUrl, readToken, () => void this.refresh());

  getTreeItem(element: vscode.TreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(element?: vscode.TreeItem): vscode.TreeItem[] {
    if (element?.id === "today") {
      return this.todayTasks().map((t) => new TaskItem(t, t.id === this.runningId()));
    }
    const s = this.state;
    if (!s) {
      return [this.item("Backend: prüfe …", "sync")];
    }
    if (!s.health.online) {
      const offline = this.item("Backend: offline", "error");
      offline.tooltip = s.health.reason;
      return [offline];
    }
    const items = [this.item(`Backend: online (v${s.health.version})`, "check")];
    items.push(this.item(s.project ? `Projekt: ${s.project.name}` : "Projekt: –", "folder"));
    const current = this.currentTask();
    items.push(current ? new TaskItem(current, true) : this.item("Aktuelle Task: –", "circle-slash"));
    const today = new vscode.TreeItem("Heute", this.todayTasks().length ? vscode.TreeItemCollapsibleState.Expanded : vscode.TreeItemCollapsibleState.None);
    today.id = "today";
    today.iconPath = new vscode.ThemeIcon("calendar");
    items.push(today);
    return items;
  }

  async refresh(): Promise<void> {
    if (this.checking) {
      return;
    }
    this.checking = true;
    try {
      const health = await getHealth();
      if (!health.online) {
        this.state = { health };
      } else {
        const [projects, today] = await Promise.all([getProjects(), getToday()]);
        this.state = { health, today, project: this.detectProject(projects) };
      }
    } catch (err) {
      this.state = { health: { online: false, reason: err instanceof Error ? err.message : "Fehler" } };
    } finally {
      this.checking = false;
    }
    this.emitter.fire();
  }

  async act(item: TaskItem | undefined, action: "start" | "pause" | "complete"): Promise<void> {
    if (!item) {
      return;
    }
    try {
      await taskAction(item.task.id, action);
    } catch (err) {
      void vscode.window.showErrorMessage(`Kairo: ${err instanceof Error ? err.message : "Aktion fehlgeschlagen"}`);
    }
    await this.refresh(); // das /ws-Ereignis lädt ebenfalls, aber so stimmt die Anzeige auch ohne /ws
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
    this.emitter.dispose();
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

  private currentTask(): Task | undefined {
    const t = this.state?.today;
    const id = this.runningId();
    return id ? [...(t?.tasks ?? []), ...(t?.active_tasks ?? [])].find((x) => x.id === id) : undefined;
  }

  private todayTasks(): Task[] {
    return (this.state?.today?.tasks ?? []).filter((t) => t.status !== "COMPLETED");
  }

  private item(label: string, icon: string): vscode.TreeItem {
    const item = new vscode.TreeItem(label);
    item.iconPath = new vscode.ThemeIcon(icon);
    return item;
  }
}
