import { create } from 'zustand'
import type { ConnectionSpec } from '../types'

export type SessionStatus = 'connecting' | 'open' | 'closed' | 'error'

export interface SessionMeta {
  id: string
  name: string
  connection: ConnectionSpec
  status: SessionStatus
  error?: string
}

interface SessionState {
  sessions: SessionMeta[]
  activeId: string | null
  add: (s: SessionMeta) => void
  update: (id: string, patch: Partial<SessionMeta>) => void
  remove: (id: string) => void
  setActive: (id: string) => void
}

export const useSessionStore = create<SessionState>((set) => ({
  sessions: [],
  activeId: null,
  add: (s) =>
    set((st) => ({
      sessions: [...st.sessions, s],
      activeId: st.activeId ?? s.id,
    })),
  update: (id, patch) =>
    set((st) => ({
      sessions: st.sessions.map((x) =>
        x.id === id ? { ...x, ...patch } : x,
      ),
    })),
  remove: (id) =>
    set((st) => ({
      sessions: st.sessions.filter((x) => x.id !== id),
      activeId: st.activeId === id ? null : st.activeId,
    })),
  setActive: (id) => set({ activeId: id }),
}))
