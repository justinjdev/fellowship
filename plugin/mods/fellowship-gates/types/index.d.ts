export type Quest = {
  name: string
  phase: string
  gatePending: boolean
  worktree: string
}

declare module 'claude-code' {
  interface PluginState {
    'fellowship-gates': { quests: Quest[]; dismissed: string[] }
  }
}
