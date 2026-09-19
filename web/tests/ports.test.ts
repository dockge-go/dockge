import assert from "node:assert/strict";
import test from "node:test";

import { displayPort, portUrl } from "../src/lib/ports.ts";

test("displayPort prefers hostPort when present", () => {
  assert.equal(displayPort({ hostPort: 8080, containerPort: 80 }), 8080);
});

test("displayPort falls back to containerPort when hostPort is missing", () => {
  assert.equal(displayPort({ containerPort: 80 }), 80);
});

test("portUrl uses https for port 443", () => {
  assert.equal(portUrl({ hostPort: 443, containerPort: 443 }, "example.com"), "https://example.com:443");
});

test("portUrl uses http for other ports", () => {
  assert.equal(portUrl({ hostPort: 8080, containerPort: 80 }, "example.com"), "http://example.com:8080");
});

test("portUrl falls back to containerPort and never yields undefined", () => {
  assert.equal(portUrl({ containerPort: 3000 }, "example.com"), "http://example.com:3000");
});

test("portUrl uses the caller-provided hostname fallback", () => {
  // 调用方约定：hostname 为空时传 location.hostname（此处模拟回落结果）
  const hostname = "" || "fallback.local";
  assert.equal(portUrl({ containerPort: 80 }, hostname), "http://fallback.local:80");
});
