import * as vscode from "vscode";
import { ContextProvider } from "./contextView";

export function activate(context: vscode.ExtensionContext): void {
  const provider = new ContextProvider();
  const view = vscode.window.createTreeView("kairo.context", { treeDataProvider: provider });

  context.subscriptions.push(
    provider,
    view,
    view.onDidChangeVisibility((e) => provider.setVisible(e.visible)),
    vscode.commands.registerCommand("kairo.refresh", () => provider.refresh()),
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
