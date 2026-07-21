// Quick commands (snippets) persisted in localStorage, with sane defaults.

import { getItemWithMigration, quickKey } from '../lib/storage'

const KEY = quickKey

export interface QuickCommand {
  id: string
  label: string
  command: string
}

const DEFAULTS: QuickCommand[] = [
  { id: 'qc-1', label: '更新软件包 (apt)', command: 'apt-get update && apt-get upgrade -y' },
  { id: 'qc-2', label: '查看磁盘', command: 'df -h' },
  { id: 'qc-3', label: '查看内存', command: 'free -h' },
  { id: 'qc-4', label: '当前进程', command: 'top -b -n 1 | head -20' },
  { id: 'qc-5', label: '列出文件', command: 'ls -la' },
  { id: 'qc-6', label: '系统信息', command: 'uname -a' },
]

export function loadQuick(): QuickCommand[] {
  try {
    const v = JSON.parse(getItemWithMigration(KEY) || '')
    if (Array.isArray(v) && v.length) return v as QuickCommand[]
  } catch {
    /* ignore */
  }
  return DEFAULTS
}

export function saveQuick(list: QuickCommand[]): void {
  localStorage.setItem(KEY, JSON.stringify(list))
}
