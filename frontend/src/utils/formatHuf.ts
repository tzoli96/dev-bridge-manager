export const formatHuf = (value: number | null) =>
    value === null ? '—' : `${Math.round(value).toLocaleString('hu-HU')} Ft`;
