import assert from "node:assert/strict";
import test from "node:test";

import { mergeContainerStatus } from "../src/lib/container-status.ts";

test("mergeContainerStatus changes only state and status", () => {
  const containers = [{
    id: "abc123",
    name: "web",
    image: "nginx:latest",
    state: "created",
    status: "Created",
    ports: "8080:80",
    stack: "frontend",
  }];

  const merged = mergeContainerStatus(containers, {
    containers: [{ id: "abc123", state: "running", status: "Up 2 seconds" }],
  });

  assert.deepEqual(merged, [{
    id: "abc123",
    name: "web",
    image: "nginx:latest",
    state: "running",
    status: "Up 2 seconds",
    ports: "8080:80",
    stack: "frontend",
  }]);
});

test("mergeContainerStatus leaves containers missing from the frame unchanged", () => {
  const containers = [{ id: "abc123", name: "web", image: "nginx", state: "running", status: "Up", ports: "" }];
  assert.deepEqual(mergeContainerStatus(containers, { containers: [] }), containers);
});
