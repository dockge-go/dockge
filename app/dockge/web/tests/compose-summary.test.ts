import assert from "node:assert/strict";
import test from "node:test";

import { summarizeCompose } from "../src/lib/compose-summary.ts";

test("summarizeCompose extracts services and top-level resources", () => {
  const summary = summarizeCompose(`services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  db:
    image: postgres:16
    volumes:
      - data:/var/lib/postgresql/data
volumes:
  data:
networks:
  edge:
`);

  assert.deepEqual(summary.services, ["web", "db"]);
  assert.equal(summary.ports, 1);
  assert.equal(summary.volumes, 1);
  assert.equal(summary.networks, 1);
});

test("summarizeCompose returns an empty summary without services", () => {
  assert.deepEqual(summarizeCompose("name: invalid"), {
    services: [],
    ports: 0,
    volumes: 0,
    networks: 0,
  });
});
