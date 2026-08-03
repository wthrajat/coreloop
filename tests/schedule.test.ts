import assert from "node:assert/strict";
import test from "node:test";

import { createDefaultSchedule, formatActiveDays } from "../lib/schedule.ts";

test("default schedule matches the accepted India weekday cadence", () => {
  const schedule = createDefaultSchedule();

  assert.equal(schedule.timeZone, "Asia/Kolkata");
  assert.equal(schedule.lessonMinutes, 15);
  assert.deepEqual(schedule.lessonTimes, ["09:00", "14:00", "21:00"]);
  assert.deepEqual(schedule.activeWeekdays, [1, 2, 3, 4, 5]);
});

test("weekday labels remain deterministic and deduplicated", () => {
  assert.equal(formatActiveDays([1, 2, 3, 4, 5]), "Monday to Friday");
  assert.equal(formatActiveDays([6, 0, 6]), "Sunday, Saturday");
});
