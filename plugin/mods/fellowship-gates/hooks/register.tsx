import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { Quest } from '../types'

import { gateKey, parseStatus, statusLine } from './status'

const POLL_MS = 10_000
const STATUS_CMD = 'exec "$HOME/.claude/fellowship/bin/fellowship" status --json'

const quests = atom({ plugin: 'fellowship-gates', key: 'quests' } as const, [])
const dismissed = atom({ plugin: 'fellowship-gates', key: 'dismissed' } as const, [])

let cwd: string | undefined
// Gates already announced, so a poll toasts only newly pending ones. null
// until the first poll, which seeds it without toasting.
let announced: Set<string> | null = null

async function refresh($: EngineInterface) {
  let list: Quest[] = []
  try {
    const ran = await $.process.run(['sh', '-c', STATUS_CMD], { cwd, timeoutMs: 15_000 })
    if (ran.exitCode === 0) list = parseStatus(ran.stdout)
  } catch {
    // Binary missing or slow: show nothing rather than a stale count.
  }

  const pending = list.filter(q => q.gatePending).map(gateKey)
  if (announced !== null) {
    for (const key of pending) {
      if (!announced.has(key)) $.ui.toast(`fellowship: ${key.replace(':', ' — ')} gate pending`)
    }
  }
  announced = new Set(pending)

  await update($, quests, () => list)
  // Forget dismissals for gates that are no longer pending, so the same
  // phase gating again after a rejection shows up again.
  await update($, dismissed, keys => keys.filter(k => pending.includes(k)))
  $.ui.status(statusLine(list))
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    cwd = e.cwd
    const started = await next(e)
    void refresh($)
    $.clock.every(POLL_MS, () => refresh($))

    return started
  })

  on('turn.complete', async ($, e, next) => {
    const done = await next(e)
    void refresh($)

    return done
  })

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    if (e.props.hasSurvey) return next(e)

    const hidden = await read($, dismissed)
    const gates = (await read($, quests)).filter(q => q.gatePending && !hidden.includes(gateKey(q)))
    if (gates.length === 0) return next(e)

    const { Box, Button, Text } = $.ui.resolve(e)

    return (
      <Box flexDirection="column">
        {gates.map(q => (
          <Box key={gateKey(q)}>
            <Text>
              🚪 <Text bold>{q.name}</Text> — {q.phase} gate pending{' '}
            </Text>
            <Button
              key={`approve:${gateKey(q)}`}
              label="Approve"
              onPress={() =>
                $.prompt.submit({
                  text: `Approve the ${q.phase} gate for quest "${q.name}" (worktree: ${q.worktree}).`,
                  asUser: true,
                })
              }
            />
            <Text> </Text>
            <Button
              key={`reject:${gateKey(q)}`}
              label="Reject…"
              onPress={() =>
                $.prompt.fill({
                  text: `Reject the ${q.phase} gate for quest "${q.name}" (worktree: ${q.worktree}). Feedback: `,
                })
              }
            />
            <Text> </Text>
            <Button
              key={`later:${gateKey(q)}`}
              label="Later"
              onPress={() => update($, dismissed, keys => [...keys, gateKey(q)])}
            />
          </Box>
        ))}
      </Box>
    )
  })
}
