// Shared "send a command line to terminal(s)" helper used by the quick input
// box and the quick-command panel. Supports two fan-out modes:
//   - broadcast=false → only the active session receives the command
//   - broadcast=true  → every *connected* session (status === 'open') receives it,
//     so an operator can run one command across many hosts at once.
//
// Returns the number of sessions the command was actually dispatched to.

import { useSessionStore } from '../store/sessionStore'
import { useLogStore } from '../store/logStore'
import { wsRegistry } from '../api/ws'
import { textToBytes } from '../terminal/encoding'

export function sendCommand(cmd: string, broadcast: boolean): number {
  const line = cmd.trim()
  if (!line) return 0
  const bytes = textToBytes(line + '\r')
  const addLog = useLogStore.getState().add

  if (broadcast) {
    let count = 0
    useSessionStore.getState().sessions.forEach((s) => {
      if (s.status !== 'open') return
      const ws = wsRegistry.get(s.id)
      if (ws) {
        ws.sendInput(bytes)
        count++
      }
    })
    addLog({
      level: 'info',
      msg: `📡 广播「${line}」→ ${count} 台已连接主机`,
    })
    return count
  }

  const activeId = useSessionStore.getState().activeId
  if (activeId) {
    wsRegistry.get(activeId)?.sendInput(bytes)
    addLog({ level: 'info', msg: '▶ ' + line })
    return 1
  }
  return 0
}

// Count of currently connected sessions, used to label the broadcast option.
export function connectedCount(): number {
  return useSessionStore
    .getState()
    .sessions.filter((s) => s.status === 'open').length
}
