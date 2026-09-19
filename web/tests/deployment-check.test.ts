import assert from "node:assert/strict";
import test from "node:test";

import { deploymentProblems, type HealthRuntime } from "../src/lib/deployment-check.ts";

const OK: HealthRuntime = {
  cli: "docker",
  compose: "docker",
  ready: true,
  stacks: { path: "/opt/stacks", mounted: true },
};

test("deploymentProblems returns [] when all healthy", () => {
  assert.deepEqual(deploymentProblems(OK), []);
});

test("deploymentProblems returns [] when runtime is undefined", () => {
  assert.deepEqual(deploymentProblems(undefined), []);
});

test("deploymentProblems reports runtime failure with error", () => {
  const out = deploymentProblems({ ...OK, ready: false, error: "socket gone" });
  assert.deepEqual(out, [{ key: "deploy.runtimeFail", vars: { error: "socket gone" } }]);
});

test("deploymentProblems reports unmounted stacks dir with path", () => {
  const out = deploymentProblems({ ...OK, stacks: { path: "/data/stacks", mounted: false } });
  assert.deepEqual(out, [{ key: "deploy.stacksFail", vars: { path: "/data/stacks" } }]);
});

test("deploymentProblems reports both problems", () => {
  const out = deploymentProblems({ ...OK, ready: false, error: "boom", stacks: { path: "/s", mounted: false } });
  assert.deepEqual(out, [
    { key: "deploy.runtimeFail", vars: { error: "boom" } },
    { key: "deploy.stacksFail", vars: { path: "/s" } },
  ]);
});

test("deploymentProblems tolerates missing error message", () => {
  const out = deploymentProblems({ ...OK, ready: false });
  assert.deepEqual(out, [{ key: "deploy.runtimeFail", vars: { error: "" } }]);
});

