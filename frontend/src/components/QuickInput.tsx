import { useState } from 'react'
import { useSessionStore } from '../store/sessionStore'
import { sendCommand, connectedCount } from '../ssh/sendCommand'

interface Props {
  onSent?: (text: string, broadcast: boolean) => void
}

// QuickInput sends a command line to the terminal. A "当前 / 广播" toggle sits
// before the send button: when 广播 is selected the command is fanned out to
// every connected session instead of just the active one.
export default function QuickInput({ onSent }: Props) {
  const [text, setText] = useState('')
  const [broadcast, setBroadcast] = useState(false)
  const activeId = useSessionStore((s) => s.activeId)
  const sessions = useSessionStore((s) => s.sessions)

  const activeConnected =
    sessions.find((s) => s.id === activeId)?.status === 'open'
  const connected = connectedCount()
  // "当前" needs an active, connected session; "广播" needs ≥1 connected session.
  const disabled = broadcast ? connected === 0 : !activeConnected

  const send = () => {
    const t = text
    if (!t.trim() || disabled) return
    sendCommand(t, broadcast)
    onSent?.(t, broadcast)
    setText('')
  }

  return (
    <div className="flex items-center gap-2 p-2 bg-gray-900 border-t border-gray-700">
      <span className="text-xs text-gray-500 whitespace-nowrap">快捷输入</span>
      <select
        value={broadcast ? 'broadcast' : 'current'}
        onChange={(e) => setBroadcast(e.target.value === 'broadcast')}
        className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-xs text-gray-200"
        title="选择发送目标"
      >
        <option value="current">当前</option>
        <option value="broadcast" disabled={connected === 0}>
          广播（{connected} 台已连接）
        </option>
      </select>
      <input
        className="flex-1 bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') send()
        }}
        placeholder={disabled ? '无可发送的主机' : '输入命令并回车发送…'}
        disabled={disabled}
      />
      <button
        onClick={send}
        disabled={disabled}
        className="px-3 py-1 text-sm bg-accent text-black rounded disabled:opacity-50"
      >
        发送
      </button>
    </div>
  )
}
