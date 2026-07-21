import { create } from 'zustand'

export interface LogEntry {
  ts: number
  level: 'info' | 'warn' | 'error'
  msg: string
}

interface LogState {
  logs: LogEntry[]
  add: (l: Omit<LogEntry, 'ts'>) => void
  clear: () => void
}

// Ring-buffered terminal/system log shared by QuickInput, session events, etc.
export const useLogStore = create<LogState>((set) => ({
  logs: [],
  add: (l) =>
    set((s) => ({
      logs: [...s.logs, { ...l, ts: Date.now() }].slice(-500),
    })),
  clear: () => set({ logs: [] }),
}))
