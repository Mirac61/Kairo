import * as assert from "node:assert/strict";
import { matchProject, Project } from "./core";

const p = (id: string, local_path: string | null): Project => ({ id, name: id, local_path });
const projects = [p("root", "/code"), p("kairo", "/code/kairo/"), p("ohne", null), p("kairo2", "/code/kairo2")];

assert.equal(matchProject(projects, "/code/kairo/backend")?.id, "kairo"); // längster Pfad gewinnt
assert.equal(matchProject(projects, "/code/kairo")?.id, "kairo"); // exakt
assert.equal(matchProject(projects, "/code/kairo2/x")?.id, "kairo2"); // kein Präfix-Treffer auf kairo
assert.equal(matchProject(projects, "/code/other")?.id, "root");
assert.equal(matchProject(projects, "/tmp"), undefined);
console.log("core.test ok");
