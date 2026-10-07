export type Order = "asc" | "desc"
export const orders: readonly Order[] = ["asc", "desc"]

/** How a table is sorted. */
export interface Sorting<K extends string> {
  by: K
  order: Order
  /** Sorts by a column: in the given order, else the current one, or the one the column starts in. */
  sort: (column: K, order?: Order) => void
}

/**
 * Sorts by the column in the address, or by the first of columns, which maps each column to the
 * direction it starts in. The address leaves out the first column and a column's own direction.
 */
export function sortingOf<K extends string>(
  search: { sort?: string; order?: Order },
  columns: Record<K, Order>,
  onChange: (change: { sort?: K; order?: Order }) => void,
): Sorting<K> {
  const keys = Object.keys(columns) as K[]
  // A sort of another table, e.g. of another tab, doesn't apply.
  const known = !search.sort || keys.includes(search.sort as K)
  const by = (known && (search.sort as K)) || keys[0]
  const current = (known && search.order) || columns[by]
  return {
    by,
    order: current,
    sort: (column, order = column === by ? current : columns[column]) =>
      onChange({ sort: column === keys[0] ? undefined : column, order: order === columns[column] ? undefined : order }),
  }
}

/** Compares names as people read them, e.g. lobby-2 before lobby-10. */
const compareNames = new Intl.Collator(undefined, { numeric: true }).compare

/**
 * Sorts items by a value, those with the same value by name. Items without the value, e.g. stopped
 * servers by CPU, come last in either order.
 */
export function sortBy<T>(items: T[], order: Order, value: (item: T) => string | number | undefined, name: (item: T) => string) {
  const sign = order === "asc" ? 1 : -1
  return items.toSorted((a, b) => {
    const [x, y] = [value(a), value(b)]
    const compared =
      x === undefined || y === undefined
        ? Number(x === undefined) - Number(y === undefined)
        : sign * (typeof x === "number" && typeof y === "number" ? x - y : compareNames(String(x), String(y)))
    return compared || compareNames(name(a), name(b))
  })
}
