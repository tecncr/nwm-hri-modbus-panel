/** Fixed categorical order; colour follows the entity, never its rank. */
export const SERIES_COLORS = Array.from({ length: 8 }, (_, i) => `var(--chart-${i + 1})`)
export const MAX_SERIES = SERIES_COLORS.length
