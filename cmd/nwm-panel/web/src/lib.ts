export function combine32(
  high: number | undefined,
  low: number | undefined,
): number | null {
  if (high === undefined || low === undefined) return null;
  return high * 0x10000 + low;
}

export function formatVolume(
  rawCount: number | null | undefined,
  litersPerPulse: number | undefined,
): string | null {
  if (
    rawCount == null ||
    !Number.isSafeInteger(rawCount) ||
    rawCount < 0 ||
    litersPerPulse === undefined ||
    !Number.isInteger(litersPerPulse) ||
    litersPerPulse < 1
  ) {
    return null;
  }
  const liters = rawCount * litersPerPulse;
  if (!Number.isSafeInteger(liters)) return null;
  const format = (value: number, digits: number) =>
    new Intl.NumberFormat(undefined, { maximumFractionDigits: digits }).format(
      value,
    );
  return `${format(liters, 2)} L · ${format(liters / 1000, 3)} m³`;
}

export function split32(value: number): [number, number] {
  if (!Number.isSafeInteger(value) || value < 0 || value > 0xffffffff)
    throw new Error("Value must be an integer between 0 and 4,294,967,295");
  return [Math.floor(value / 0x10000), value & 0xffff];
}

export function decodeClock(words: number[]): string | null {
  if (words.length < 3) return null;
  const year = 2000 + (words[0] >> 8);
  const month = words[0] & 0xff;
  const day = words[1] >> 8;
  const hour = words[1] & 0xff;
  const minute = words[2] >> 8;
  const second = words[2] & 0xff;
  if (
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > 31 ||
    hour > 23 ||
    minute > 59 ||
    second > 59
  )
    return null;
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")} ${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}:${String(second).padStart(2, "0")}`;
}

export function encodeClock(value: string): number[] {
  const date = new Date(value);
  if (!value || Number.isNaN(date.getTime()))
    throw new Error("Enter a valid date and time");
  const year = date.getFullYear();
  if (year < 2000 || year > 2255)
    throw new Error("Meter clock supports years 2000–2255");
  return [
    ((year - 2000) << 8) | (date.getMonth() + 1),
    (date.getDate() << 8) | date.getHours(),
    (date.getMinutes() << 8) | date.getSeconds(),
  ];
}

export function hex(value: number, width = 4): string {
  return `0x${value.toString(16).toUpperCase().padStart(width, "0")}`;
}
