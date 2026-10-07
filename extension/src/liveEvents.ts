import WebSocket = require("ws");
import { authHeaders, joinUrl } from "./core";

const DEBOUNCE_MS = 100;
const RECONNECT_MS = 2000;
const MAX_RECONNECT_MS = 15000; // bei ausgeschaltetem Backend nicht alle 2 s anklopfen

/**
 * Hält /ws offen und ruft onChange bei jedem Ereignis sowie beim (Wieder-)Verbinden
 * und Trennen auf. Der Client liest den Stand danach per REST neu.
 */
export class LiveEvents {
  private ws: WebSocket | undefined;
  private debounce: NodeJS.Timeout | undefined;
  private retry: NodeJS.Timeout | undefined;
  private running = false;
  private delay = RECONNECT_MS;

  constructor(
    private readonly url: () => string,
    private readonly token: () => string | undefined,
    private readonly onChange: () => void,
  ) {}

  start(): void {
    this.stop();
    this.running = true;
    this.connect();
  }

  stop(): void {
    this.running = false;
    clearTimeout(this.debounce);
    clearTimeout(this.retry);
    this.ws?.removeAllListeners();
    this.ws?.on("error", () => undefined); // close() vor dem Verbindungsaufbau meldet sonst einen Fehler
    this.ws?.close();
    this.ws = undefined;
  }

  private notify(): void {
    clearTimeout(this.debounce);
    this.debounce = setTimeout(this.onChange, DEBOUNCE_MS);
  }

  private connect(): void {
    const ws = new WebSocket(joinUrl(this.url(), "/ws").replace(/^http/, "ws"), {
      headers: authHeaders(this.token()),
    });
    this.ws = ws;
    ws.on("open", () => {
      this.delay = RECONNECT_MS;
      this.notify();
    });
    ws.on("message", () => this.notify());
    ws.on("error", () => undefined); // "close" folgt und kümmert sich um den Rest
    ws.on("close", () => {
      this.notify();
      if (this.running) {
        this.retry = setTimeout(() => this.connect(), this.delay);
        this.delay = Math.min(this.delay * 2, MAX_RECONNECT_MS);
      }
    });
  }
}
