export const PAGE_SIZE = 15;

export function pageCount(total: number): number {
  return Math.max(1, Math.ceil(total / PAGE_SIZE));
}

export function clampPage(page: number, total: number): number {
  return Math.min(Math.max(1, page), pageCount(total));
}

export function paginate<T>(items: readonly T[], page: number): readonly T[] {
  const start = (clampPage(page, items.length) - 1) * PAGE_SIZE;
  return items.slice(start, start + PAGE_SIZE);
}
