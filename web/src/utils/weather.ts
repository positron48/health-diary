export const PRESSURE_CHANGE_THRESHOLD_HPA = 5

function decimal(value: number) {
  return value.toFixed(1).replace('.', ',')
}

export function isSignificantPressureChange(value?: number | null) {
  return value != null
    && Number.isFinite(Number(value))
    && Math.abs(Number(value)) >= PRESSURE_CHANGE_THRESHOLD_HPA
}

export function pressureChangeArrow(value: number) {
  const numeric = Number(value)
  return `${numeric < 0 ? '↓' : '↑'}${decimal(Math.abs(numeric))}`
}

export function signedPressureChange(value?: number | null) {
  if (value == null || !Number.isFinite(Number(value))) return 'нет данных'
  const numeric = Number(value)
  return `${numeric > 0 ? '+' : ''}${decimal(numeric)} гПа за 24 ч`
}
