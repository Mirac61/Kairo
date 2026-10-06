import * as vscode from "vscode";
import { watchIdle } from "./idleWatch";
import { ContextProvider, openWeb } from "./contextView";

export function activate(context: vscode.ExtensionContext): void {
  const provider = new ContextProvider(context.extensionUri);

  context.subscriptions.push(
    provider,
    vscode.window.registerWebviewViewProvider("kairo.context", provider),
    watchIdle(provider),
    vscode.commands.registerCommand("kairo.refresh", () => provider.refresh()),
    vscode.commands.registerCommand("kairo.openWeb", () => openWeb()),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("kairo.backendUrl")) {
        void provider.refresh();
      }
      if (e.affectsConfiguration("kairo.documentsFolder")) {
        void provider.listDir("");
      }
    }),
  );
}

export function deactivate(): void {
  // Aufräumen übernimmt context.subscriptions.
}
