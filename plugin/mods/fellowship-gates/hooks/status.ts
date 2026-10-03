import type { Quest } from '../types'

type StatusJson = {
  quests?: {
    name: string
    phase: string
    gate_pending: boolean
    worktree: string
    merged: boolean
  }[]
}

export const gateKey = (q: Quest) => `${q.name}:${q.phase}`

export function parseStatus(stdout: string): Quest[] {
  try {
    const parsed = JSON.parse(stdout) as StatusJson
    return (parsed.quests ?? [])
      .filter(q => !q.merged)
      .map(q => ({
        name: q.name,
        phase: q.phase,
        gatePending: q.gate_pending,
        worktree: q.worktree,
      }))
  } catch {
    return []
  }
}

export function statusLine(list: Quest[]): string | undefined {
  if (list.length === 0) return undefined
  const pending = list.filter(q => q.gatePending).length
  const parts = [`⚔ ${list.length} quest${list.length === 1 ? '' : 's'}`]
  if (pending > 0) parts.push(`${pending} gate${pending === 1 ? '' : 's'} pending`)
  const only = list.length === 1 ? list[0] : undefined
  if (only) parts.push(`${only.name}: ${only.phase}`)
  return parts.join(' · ')
}
