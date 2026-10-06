import * as vscode from "vscode";
import { taskAction } from "./backendClient";
import { isIdle } from "./core";
import type { ContextProvider } from "./contextView";

const CHECK_MS = 30_000;

/**
 * Meldet sich, wenn ein Timer läuft, aber in VSCodium lange nichts passiert ist.
 * Es pausiert nie von selbst: Arbeit kann auch außerhalb des Editors stattfinden.
 */
export function watchIdle(provider: ContextProvider): vscode.Disposable {
  let last = Date.now();
  let asked = false;
  let busy = false;
  const touch = () => {
    last = Date.now();
    asked = false;
  };
  const check = async () => {
    const minutes = vscode.workspace.getConfiguration("kairo").get<number>("idleMinutes", 10);
    if (busy || asked || !isIdle(last, Date.now(), minutes * 60_000)) {
      return;
    }
    busy = true;
    try {
      await provider.refresh(); // der Stand ist veraltet, solange die Ansicht verborgen ist
      const task = provider.runningTask();
      if (!task) {
        return;
      }
      asked = true;
      const since = new Date(last).toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" });
      const pause = "Pausieren";
      const choice = await vscode.window.showInformationMessage(
        `Seit ${since} keine Aktivität in VSCodium. Der Timer für „${task.title}“ läuft noch.`,
        pause,
        "Weiter laufen",
      );
      if (choice === pause) {
        await taskAction(task.id, "pause");
        await provider.refresh();
      }
    } catch (err) {
      void vscode.window.showErrorMessage(`Kairo: ${err instanceof Error ? err.message : "Pausieren fehlgeschlagen"}`);
    } finally {
      busy = false;
    }
  };
  const timer = setInterval(() => void check(), CHECK_MS);
  return vscode.Disposable.from(
    { dispose: () => clearInterval(timer) },
    vscode.workspace.onDidChangeTextDocument(touch),
    vscode.window.onDidChangeActiveTextEditor(touch),
    vscode.window.onDidChangeTextEditorSelection(touch),
    vscode.window.onDidChangeWindowState((s) => s.focused && touch()),
  );
}
