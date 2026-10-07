import * as vscode from "vscode";
import { watchIdle } from "./idleWatch";
import { registerCommands } from "./commands";
import { ContextProvider, openWeb } from "./contextView";
import { registerUriHandler } from "./uriHandler";

export function activate(context: vscode.ExtensionContext): void {
  const provider = new ContextProvider(context.extensionUri);

  context.subscriptions.push(
    provider,
    vscode.window.registerWebviewViewProvider("kairo.context", provider),
    watchIdle(provider),
    registerCommands(provider),
    registerUriHandler(context, provider),
    vscode.commands.registerCommand("kairo.refresh", () => provider.refresh()),
    vscode.commands.registerCommand("kairo.openWeb", () => openWeb()),
    vscode.workspace.onDidChangeWorkspaceFolders(() => void provider.refresh()), // erkennt das Projekt neu
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("kairo.backendUrl")) {
        provider.start(); // verbindet /ws neu, das Ereignis lädt dann neu
      }
    }),
  );
  provider.start();
}

export function deactivate(): void {
  // Aufräumen übernimmt context.subscriptions.
}
