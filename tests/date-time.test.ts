import assert from "node:assert/strict";
import test from "node:test";

import { formatIndiaDateTime } from "../lib/date-time.ts";

test("India timestamps are stable and invalid values are safe", () => {
  assert.equal(
    formatIndiaDateTime("2026-08-04T12:30:00.000Z"),
    "4 Aug 2026, 6:00 pm",
  );
  assert.equal(formatIndiaDateTime("not-a-date"), "Time unavailable");
});
