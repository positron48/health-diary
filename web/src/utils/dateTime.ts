const parts = (date: Date, timeZone: string) =>
  Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', {
      timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
    }).formatToParts(date).filter((part) => part.type !== 'literal').map((part) => [part.type, part.value]),
  )

export function instantToLocalInput(value: string, timeZone: string) {
  const p = parts(new Date(value), timeZone)
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`
}

export function localInputToUTC(value: string, timeZone: string) {
  const [date, clock] = value.split('T')
  const [year, month, day] = date.split('-').map(Number)
  const [hour, minute] = clock.split(':').map(Number)
  const wallUTC = Date.UTC(year, month - 1, day, hour, minute)
  const guess = new Date(wallUTC)
  const p = parts(guess, timeZone)
  const representedUTC = Date.UTC(Number(p.year), Number(p.month) - 1, Number(p.day), Number(p.hour), Number(p.minute), Number(p.second))
  return new Date(wallUTC - (representedUTC - guess.getTime())).toISOString()
}
