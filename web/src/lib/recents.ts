// История недавно открытых страниц (id) в localStorage — для палитры поиска.
const KEY = "td-recents";
const MAX = 8;

export function getRecents(): number[] {
  try {
    const a = JSON.parse(localStorage.getItem(KEY) ?? "[]");
    return Array.isArray(a) ? a.filter((x) => typeof x === "number") : [];
  } catch {
    return [];
  }
}

export function pushRecent(id: number): void {
  const next = [id, ...getRecents().filter((x) => x !== id)].slice(0, MAX);
  localStorage.setItem(KEY, JSON.stringify(next));
}
