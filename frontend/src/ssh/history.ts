// Per-connection command history persisted in localStorage.

import { getItemWithMigration, historyKey } from '../lib/storage'

const KEY = historyKey

export interface HistoryEntry {
  id: string
  connId: string
  command: string
  ts: number
}

export function loadHistory(): HistoryEntry[] {
  try {
    return JSON.parse(getItemWithMigration(KEY) || '[]') as HistoryEntry[]
  } catch {
    return []
  }
}

export function addHistory(connId: string, command: string): void {
  const all = loadHistory().filter((h) => !(h.connId === connId && h.command === command))
  all.unshift({ id: String(Date.now()), connId, command, ts: Date.now() })
  localStorage.setItem(KEY, JSON.stringify(all.slice(0, 300)))
}

export function historyFor(connId: string): HistoryEntry[] {
  return loadHistory().filter((h) => h.connId === connId).slice(0, 50)
}

export function clearHistory(): void {
  localStorage.removeItem(KEY)
}
