import * as vscode from "vscode";
import { getHealth, HealthResult } from "./backendClient";

const POLL_INTERVAL_MS = 15_000;

export class ContextProvider implements vscode.TreeDataProvider<vscode.TreeItem>, vscode.Disposable {
  private readonly emitter = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.emitter.event;
  private health: HealthResult | undefined;
  private timer: NodeJS.Timeout | undefined;
  private checking = false;

  getTreeItem(element: vscode.TreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): vscode.TreeItem[] {
    return [this.backendItem(), this.placeholder("Projekt: –"), this.placeholder("Heute: –")];
  }

  async refresh(): Promise<void> {
    if (this.checking) {
      return;
    }
    this.checking = true;
    try {
      this.health = await getHealth();
    } finally {
      this.checking = false;
    }
    this.emitter.fire();
  }

  /** Startet das Polling nur, solange die Ansicht sichtbar ist. */
  setVisible(visible: boolean): void {
    this.stopPolling();
    if (visible) {
      void this.refresh();
      this.timer = setInterval(() => void this.refresh(), POLL_INTERVAL_MS);
    }
  }

  dispose(): void {
    this.stopPolling();
    this.emitter.dispose();
  }

  private stopPolling(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  private backendItem(): vscode.TreeItem {
    const h = this.health;
    if (!h) {
      const item = new vscode.TreeItem("Backend: prüfe …");
      item.iconPath = new vscode.ThemeIcon("sync");
      return item;
    }
    const item = new vscode.TreeItem(h.online ? `Backend: online (v${h.version})` : "Backend: offline");
    item.iconPath = new vscode.ThemeIcon(h.online ? "check" : "error");
    if (!h.online) {
      item.tooltip = h.reason;
    }
    return item;
  }

  private placeholder(label: string): vscode.TreeItem {
    return new vscode.TreeItem(label);
  }
}
