import assert from "node:assert/strict";
import test from "node:test";

import { addService, listServices, readService, removeService, updateService } from "../src/lib/yaml-edit.ts";

const DOC = `# 顶部注释
services:
  web: # 行尾注释
    image: nginx:latest
    ports:
      - "8080:80"
  db:
    image: postgres:16
networks:
  edge:
`;

test("listServices returns service names", () => {
  assert.deepEqual(listServices(DOC), ["web", "db"]);
  assert.deepEqual(listServices("not: yaml"), []);
});

test("addService appends skeleton with restart policy and keeps comments", () => {
  const out = addService(DOC, "cache");
  assert.match(out, /cache:/);
  assert.match(out, /restart: unless-stopped/);
  assert.match(out, /# 顶部注释/);
  assert.match(out, /# 行尾注释/);
  assert.equal(listServices(out).length, 3);
});

test("addService is idempotent on existing name", () => {
  assert.equal(addService(DOC, "web"), DOC);
});

test("removeService drops the service and keeps others", () => {
  const out = removeService(DOC, "web");
  assert.deepEqual(listServices(out), ["db"]);
  assert.match(out, /# 顶部注释/);
});

test("updateService rewrites scalar and list fields, keeps comments", () => {
  const out = updateService(DOC, "web", {
    image: "nginx:1.27",
    ports: ["9090:80"],
    volumes: ["data:/var/www"],
    restart: "always",
    environment: ["KEY=value"],
    dependsOn: ["db"],
  });
  assert.match(out, /image: nginx:1\.27/);
  assert.match(out, /- 9090:80/);
  assert.match(out, /- data:\/var\/www/);
  assert.match(out, /restart: always/);
  assert.match(out, /- KEY=value/);
  assert.match(out, /- db/);
  assert.match(out, /# 行尾注释/);
  assert.match(out, /# 顶部注释/);
});

test("updateService on missing service is a no-op", () => {
  assert.equal(updateService(DOC, "ghost", { image: "x" }), DOC);
});

test("readService returns current fields", () => {
  assert.deepEqual(readService(DOC, "web"), {
    image: "nginx:latest",
    ports: ["8080:80"],
    volumes: undefined,
    restart: undefined,
    environment: undefined,
    dependsOn: undefined,
  });
  assert.deepEqual(readService(DOC, "ghost"), {});
});

test("listTopLevelNetworks / addNetwork / removeNetwork round-trip", async () => {
  const { listTopLevelNetworks, setTopLevelNetworks } = await import("../src/lib/yaml-edit.ts");
  assert.deepEqual(listTopLevelNetworks(DOC), ["edge"]);
  const out = setTopLevelNetworks(DOC, ["edge", "lan"]);
  assert.deepEqual(listTopLevelNetworks(out), ["edge", "lan"]);
  assert.match(out, /# 顶部注释/);
});
