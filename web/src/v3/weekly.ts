import { localDay, type Entry } from "./api";

export function weekWindow(now: Date, offset = 0) {
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  start.setDate(start.getDate() - ((start.getDay() + 6) % 7) + offset * 7);
  const end = new Date(start);
  end.setDate(end.getDate() + 7);
  const days = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(start);
    d.setDate(d.getDate() + i);
    return localDay(d);
  });
  return { start, end, days };
}
export function recordsInWeek(records: Entry[], now: Date, offset = 0) {
  const { days } = weekWindow(now, offset);
  return records.filter(
    (r) =>
      ["note", "checkin", "conversation", "followup"].includes(r.kind) &&
      days.includes(
        r.kind === "checkin" && r.day ? r.day : localDay(new Date(r.createdAt)),
      ),
  );
}
