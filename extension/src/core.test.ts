import * as assert from "node:assert/strict";
import { expandHome, isIdle, matchProject, Project } from "./core";

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
console.log("core.test ok");
