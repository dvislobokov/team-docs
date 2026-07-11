// Форматирование относительного времени на русском («2 часа назад»).

function plural(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few;
  return many;
}

export function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const sec = Math.max(0, Math.round((Date.now() - then) / 1000));

  if (sec < 45) return "только что";
  const min = Math.round(sec / 60);
  if (min < 60) return `${min} ${plural(min, "минуту", "минуты", "минут")} назад`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr} ${plural(hr, "час", "часа", "часов")} назад`;
  const day = Math.round(hr / 24);
  if (day < 30) return `${day} ${plural(day, "день", "дня", "дней")} назад`;

  return new Date(iso).toLocaleDateString("ru-RU", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

export function readingLabel(min: number): string {
  return `${min} ${plural(min, "минута", "минуты", "минут")} чтения`;
}
