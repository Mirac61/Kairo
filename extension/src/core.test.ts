import * as assert from "node:assert/strict";
import { errorMessage, expandHome, formatElapsed, isIdle, matchProject, parseQuickAdd, parseUriTarget, pendingFresh, Project, Resource, resourcesFirst, startable, Task, weekRange } from "./core";

const p = (id: string, local_path: string | null): Project => ({ id, name: id, description: "", status: "ACTIVE", local_path });
const projects = [p("root", "/code"), p("kairo", "/code/kairo/"), p("ohne", null), p("kairo2", "/code/kairo2")];

assert.equal(matchProject(projects, "/code/kairo/backend")?.id, "kairo"); // längster Pfad gewinnt
assert.equal(matchProject(projects, "/code/kairo")?.id, "kairo"); // exakt
assert.equal(matchProject(projects, "/code/kairo2/x")?.id, "kairo2"); // kein Präfix-Treffer auf kairo
assert.equal(matchProject(projects, "/code/other")?.id, "root");
assert.equal(matchProject(projects, "/tmp"), undefined);
const home = [p("tilde", "~/code/kairo"), p("tilde2", "~code")];
assert.equal(matchProject(home, "/Users/m/code/kairo/backend", "/Users/m")?.id, "tilde"); // ~ wird erweitert
assert.equal(matchProject(home, "/Users/m/code", "/Users/m"), undefined); // "~code" ist kein Home-Pfad
assert.equal(expandHome("~", "/Users/m"), "/Users/m");
assert.equal(isIdle(0, 600_000, 600_000), true);
assert.equal(isIdle(0, 599_999, 600_000), false);
assert.equal(isIdle(0, 1e9, 0), false); // 0 = aus
assert.equal(errorMessage(409, '{"error":"Task hat Zeiteinträge"}'), "Task hat Zeiteinträge");
assert.equal(errorMessage(502, "<html>"), "HTTP 502"); // kein JSON
assert.equal(errorMessage(500, "null"), "HTTP 500");
assert.equal(errorMessage(400, '{"error":""}'), "HTTP 400");
assert.deepEqual(parseQuickAdd("30 min Sport"), { title: "Sport", minutes: 30 });
assert.deepEqual(parseQuickAdd(" Sport 45m "), { title: "Sport", minutes: 45 });
assert.deepEqual(parseQuickAdd("Mathe lernen"), { title: "Mathe lernen", minutes: 0 });
assert.deepEqual(parseQuickAdd("3D Druck 30 min"), { title: "3D Druck", minutes: 30 }); // Titel darf mit einer Ziffer beginnen
assert.equal(formatElapsed(32_000), "0:32");
assert.equal(formatElapsed(3_725_000), "1:02:05");
assert.equal(formatElapsed(-5), "0:00"); // Uhr-Versatz nie negativ
const t = (id: string, status: string, project_id: string | null): Task => ({ id, title: id, status, priority: "NORMAL", estimated_minutes: 0, planned_start_at: null, project_id });
const order = startable([t("a", "PLANNED", null), t("b", "PLANNED", "p"), t("c", "PLANNED", null), t("d", "COMPLETED", "p"), t("e", "CANCELLED", null)], new Set(["c"]), "p");
assert.deepEqual(order.map((x) => x.id), ["c", "b", "a"]); // heute, Projekt, Rest; erledigt/abgebrochen fehlen
const res = (id: string, task_id: string | null, project_id: string | null): Resource => ({ id, type: "FILE", target: id, label: id, task_id, project_id });
const rs = [res("a", null, "x"), res("b", "t1", null), res("c", null, "p"), res("d", "t2", null)];
assert.deepEqual(resourcesFirst(rs, [t("t1", "PLANNED", "p"), t("t2", "PLANNED", "x")], "p").map((x) => x.id), ["b", "c", "a", "d"]); // direkt oder über die Task
assert.deepEqual(resourcesFirst(rs, [], undefined).map((x) => x.id), ["a", "b", "c", "d"]); // ohne Projekt bleibt die Reihenfolge
assert.deepEqual(parseUriTarget("/start", "task=abc"), { action: "start", id: "abc" });
assert.deepEqual(parseUriTarget("/open/", "project=p1&x=1"), { action: "open", id: "p1" });
assert.equal(parseUriTarget("/start", "project=p1"), undefined); // falscher Parameter
assert.equal(parseUriTarget("/start", "task="), undefined);
assert.equal(parseUriTarget("/other", "task=abc"), undefined);
assert.equal(pendingFresh(0, 59_999), true);
assert.equal(pendingFresh(0, 60_000), false); // ab 60 s verfallen
assert.deepEqual(weekRange(new Date(2026, 9, 7)), { from: "2026-10-05", to: "2026-10-11" }); // Mittwoch
assert.deepEqual(weekRange(new Date(2026, 2, 1)), { from: "2026-02-23", to: "2026-03-01" }); // Sonntag gehört zur Woche davor, über den Monatswechsel
assert.deepEqual(weekRange(new Date(2026, 9, 5)), { from: "2026-10-05", to: "2026-10-11" }); // Montag
console.log("core.test ok");
