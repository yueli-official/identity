// Returns the current user (display claims only — never the tokens).
export default defineEventHandler(async (event) => {
  const s = await sessionForEvent(event, { clearOnRefreshFailure: true })
  return { user: s?.user ?? null }
})
