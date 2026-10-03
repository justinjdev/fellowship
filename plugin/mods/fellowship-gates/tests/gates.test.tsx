import { expect, test } from 'claude-code/testing'

import { parseStatus, statusLine } from '../hooks/status'

const STATUS = JSON.stringify({
  quests: [
    { name: 'auth-fix', phase: 'Plan', gate_pending: true, worktree: '/w/auth', merged: false },
    { name: 'docs', phase: 'Implement', gate_pending: false, worktree: '/w/docs', merged: false },
    { name: 'old', phase: 'Review', gate_pending: false, worktree: '/w/old', merged: true },
  ],
})

test('parses status, dropping merged quests', () => {
  const list = parseStatus(STATUS)
  expect(list.map(q => q.name)).toEqual(['auth-fix', 'docs'])
  expect(statusLine(list)).toBe('⚔ 2 quests · 1 gate pending')
  expect(statusLine([list[1]!])).toBe('⚔ 1 quest · docs: Implement')
  expect(statusLine(parseStatus('not json'))).toBe(undefined)
})

for (const surface of ['terminal', 'desktop'] as const) {
  test(`band offers approve for a pending gate (${surface})`, async ($, on) => {
    const submitted: string[] = []
    const statuses: (string | undefined)[] = []
    let polled!: () => void
    const firstPoll = new Promise<void>(resolve => (polled = resolve))
    on('process.run', () => ({
      value: {
        exitCode: 0,
        stdout: STATUS,
        stderr: '',
        isStdoutTruncated: false,
        isStderrTruncated: false,
      },
    }))
    // Refuse the poll interval: the first refresh runs from session.start.
    on('clock.every', () => ({ deny: 'no timers in tests' }))
    // Stands in for the engine's own band: an empty Box.
    on('ui.render', (h$, e) => {
      const { Box } = h$.ui.resolve(e)
      return <Box key="engine" />
    })
    on('session.start', (_$, e) => ({ cwd: e.cwd }))
    on('turn.complete', () => ({}) as never)
    on('ui.status', (_$, e) => {
      statuses.push(e.text)
      polled()
      return { value: undefined }
    })
    on('ui.toast', () => ({ value: undefined }))
    on('prompt.submit', (_$, e) => {
      submitted.push(e.text)
      return { text: e.text }
    })

    await $.session.start({ cwd: '/repo', surface, isInteractive: true })
    await firstPoll

    const ui = await $.ui.mount({
      plugin: 'fellowship-gates',
      surface,
      component: 'AbovePrompt',
      props: { hasSurvey: false, isWorking: false, maxRows: 10, bodyColumns: 100 } as never,
    })
    expect(statuses).toContain('⚔ 2 quests · 1 gate pending')
    expect(await ui.findAll({ type: 'Button' })).toHaveLength(3)

    await ui.press({ key: 'approve:auth-fix:Plan' })
    expect(submitted[0]).toContain('Approve the Plan gate for quest "auth-fix"')

    await ui.press({ key: 'later:auth-fix:Plan' })
    expect(await ui.findAll({ type: 'Button' })).toHaveLength(0)
  })
}
