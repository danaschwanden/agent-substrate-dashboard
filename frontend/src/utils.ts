export function stateClass(state: string): string {
  return `state state--${state.toLowerCase()}`;
}

export function titleCase(value: string): string {
  return value.toLowerCase().replace(/(^|_)([a-z])/g, (_match, separator: string, letter: string) => `${separator ? " " : ""}${letter.toUpperCase()}`);
}

export function formatClock(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "--:--:--";
  return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).format(date);
}

export function formatRatio(value: number | null | undefined): string {
  return value == null ? "—" : value.toLocaleString(undefined, { maximumFractionDigits: 1 });
}

export function formatBytes(value: string): string {
  const bytes = Number(value);
  if (!Number.isFinite(bytes)) return value;
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let amount = bytes;
  let index = 0;
  while (amount >= 1024 && index < units.length - 1) {
    amount /= 1024;
    index += 1;
  }
  return `${amount.toLocaleString(undefined, { maximumFractionDigits: amount >= 10 ? 0 : 1 })} ${units[index]}`;
}

export function formatResource(value: string, unit: string): string {
  if (unit === "bytes") return formatBytes(value);
  if (unit === "millicores") return `${value}m`;
  return value;
}
