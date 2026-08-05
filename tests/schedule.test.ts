import assert from "node:assert/strict";
import test from "node:test";

import {
  createDefaultSchedule,
  formatActiveDays,
  lessonTimePresets,
} from "../lib/schedule.ts";

test("default schedule matches the accepted India weekday cadence", () => {
  const schedule = createDefaultSchedule();

  assert.equal(schedule.timeZone, "Asia/Kolkata");
  assert.equal(schedule.lessonMinutes, 15);
  assert.deepEqual(schedule.lessonTimes, ["08:30", "13:00", "20:30"]);
  assert.deepEqual(schedule.activeWeekdays, [1, 2, 3, 4, 5]);
});

test("three-lesson preset is the canonical default schedule", () => {
  assert.deepEqual(lessonTimePresets[3], ["08:30", "13:00", "20:30"]);
  assert.deepEqual(createDefaultSchedule().lessonTimes, lessonTimePresets[3]);
});

test("weekday labels remain deterministic and deduplicated", () => {
  assert.equal(formatActiveDays([1, 2, 3, 4, 5]), "Monday to Friday");
  assert.equal(formatActiveDays([6, 0, 6]), "Sunday, Saturday");
});
