import assert from "node:assert/strict";
import test from "node:test";

import { toDiagnostics } from "../src/lib/validate.ts";

test("toDiagnostics maps in-range line numbers to correct offsets", () => {
  const doc = "services:\n  web:\n    image: nginx\n";
  const diags = toDiagnostics([{ line: 3, message: "bad image" }], doc);
  assert.equal(diags.length, 1);
  assert.equal(diags[0].from, "services:\n  web:\n".length);
  assert.equal(diags[0].to, doc.length - 1);
  assert.equal(diags[0].message, "bad image");
  assert.equal(diags[0].severity, "error");
});

test("toDiagnostics keeps empty-range diagnostics for unlocatable errors", () => {
  const doc = "services:\n";
  for (const line of [0, 99]) {
    const diags = toDiagnostics([{ line, message: "services.web.image must be a string" }], doc);
    assert.deepEqual(diags, [{ from: 0, to: 0, message: "services.web.image must be a string", severity: "error" }]);
  }
});

test("toDiagnostics maps each error independently", () => {
  const doc = "a: 1\nb: 2\nc: 3\n";
  const diags = toDiagnostics(
    [
      { line: 1, message: "first" },
      { line: 0, message: "second" },
      { line: 3, message: "third" },
    ],
    doc,
  );
  assert.equal(diags[0].from, 0);
  assert.deepEqual(
    [diags[1].from, diags[1].to],
    [0, 0],
  );
  assert.equal(diags[2].from, "a: 1\nb: 2\n".length);
  assert.equal(doc.slice(diags[2].from, diags[2].to), "c: 3");
});
