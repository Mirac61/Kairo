import * as vscode from "vscode";
import { ContextProvider, TaskItem } from "./contextView";

export function activate(context: vscode.ExtensionContext): void {
  const provider = new ContextProvider();
  const view = vscode.window.createTreeView("kairo.context", { treeDataProvider: provider });

  context.subscriptions.push(
    provider,
    view,
    view.onDidChangeVisibility((e) => provider.setVisible(e.visible)),
    vscode.commands.registerCommand("kairo.refresh", () => provider.refresh()),
    vscode.commands.registerCommand("kairo.startTask", (i?: TaskItem) => provider.act(i, "start")),
    vscode.commands.registerCommand("kairo.pauseTask", (i?: TaskItem) => provider.act(i, "pause")),
    vscode.commands.registerCommand("kairo.completeTask", (i?: TaskItem) => provider.act(i, "complete")),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("kairo.backendUrl")) {
        void provider.refresh();
      }
    }),
  );

  provider.setVisible(view.visible);
}

export function deactivate(): void {
  // Aufräumen übernimmt context.subscriptions.
}
