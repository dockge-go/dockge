import assert from "node:assert/strict";
import test from "node:test";

import { PAGE_SIZE, clampPage, pageCount, paginate } from "../src/lib/pagination.ts";

test("paginate returns fifteen items for the first page", () => {
  const items = Array.from({ length: 31 }, (_, index) => index + 1);
  assert.equal(PAGE_SIZE, 15);
  assert.deepEqual(paginate(items, 1), items.slice(0, 15));
});

test("paginate returns the remaining items for the last page", () => {
  const items = Array.from({ length: 31 }, (_, index) => index + 1);
  assert.deepEqual(paginate(items, 3), [31]);
});

test("pageCount and clampPage keep pages valid", () => {
  assert.equal(pageCount(0), 1);
  assert.equal(pageCount(31), 3);
  assert.equal(clampPage(4, 31), 3);
  assert.equal(clampPage(0, 31), 1);
});
