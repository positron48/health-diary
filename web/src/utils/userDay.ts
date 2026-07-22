export function dayStartMinutes(value = '00:00'): number {
  const match = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(value)
  return match ? Number(match[1]) * 60 + Number(match[2]) : 0
}

export function userDate(
  now: Date,
  timezone: string,
  dayStart = '00:00',
): string {
  const shifted = new Date(now.getTime() - dayStartMinutes(dayStart) * 60_000)
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(shifted)
  const value = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value || ''
  return `${value('year')}-${value('month')}-${value('day')}`
}
