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
    set((st) => {
      const idx = st.sessions.findIndex((x) => x.id === id)
      if (idx === -1) return {}
      const sessions = st.sessions.filter((x) => x.id !== id)
      let activeId = st.activeId
      if (st.activeId === id) {
        // After closing the active tab, focus a neighbor: prefer the left one,
        // then the right; if neither exists, there are no tabs left.
        const left = idx > 0 ? st.sessions[idx - 1] : undefined
        const right = idx < st.sessions.length - 1 ? st.sessions[idx + 1] : undefined
        activeId = (left ?? right)?.id ?? null
      }
      return { sessions, activeId }
    }),
  setActive: (id) => set({ activeId: id }),
}))
