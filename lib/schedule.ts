export const defaultTimeZone = "Asia/Kolkata";
export const defaultLessonTimes = ["08:30", "13:00", "20:30"] as const;
export const defaultActiveWeekdays = [1, 2, 3, 4, 5] as const;

export const lessonTimePresets: Readonly<Record<number, readonly string[]>> = {
  1: ["20:30"],
  2: ["08:30", "20:30"],
  3: defaultLessonTimes,
  4: ["08:30", "12:00", "16:30", "20:30"],
  5: ["08:00", "11:00", "14:00", "17:30", "20:30"],
  6: ["08:00", "10:30", "13:00", "15:30", "18:00", "20:30"],
};

const weekdayLabels = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
] as const;

export type SchedulePreset = {
  timeZone: string;
  lessonMinutes: 15 | 30;
  lessonTimes: string[];
  activeWeekdays: number[];
};

export function createDefaultSchedule(): SchedulePreset {
  return {
    timeZone: defaultTimeZone,
    lessonMinutes: 15,
    lessonTimes: [...defaultLessonTimes],
    activeWeekdays: [...defaultActiveWeekdays],
  };
}

export function formatActiveDays(activeWeekdays: readonly number[]): string {
  const normalizedDays = [...new Set(activeWeekdays)]
    .filter((day) => day >= 0 && day <= 6)
    .sort((first, second) => first - second);

  if (
    normalizedDays.length === defaultActiveWeekdays.length &&
    normalizedDays.every((day, index) => day === defaultActiveWeekdays[index])
  ) {
    return "Monday to Friday";
  }

  return normalizedDays.map((day) => weekdayLabels[day]).join(", ");
}
