import { useState } from 'react'
import { loadQuick, saveQuick, type QuickCommand } from '../ssh/quickCommands'
import { sendCommand, connectedCount } from '../ssh/sendCommand'
import { useSessionStore } from '../store/sessionStore'

// QuickCommandPanel lists saved snippet commands and lets the user add, edit and
// delete them (persisted to localStorage via quickCommands). Each command can be
// sent to the active host or broadcast to every connected host via the toggle.
export default function QuickCommandPanel() {
  const [list, setList] = useState<QuickCommand[]>(() => loadQuick())
  const [broadcast, setBroadcast] = useState(false)
  // 'new' = adding a fresh command; otherwise holds the id being edited; null = idle.
  const [editing, setEditing] = useState<string | null>(null)
  const [draft, setDraft] = useState({ label: '', command: '' })
  // id awaiting a delete confirmation (two-click guard, keeps the panel themed).
  const [confirming, setConfirming] = useState<string | null>(null)

  const connected = connectedCount()
  const sessions = useSessionStore((s) => s.sessions)
  const activeId = useSessionStore((s) => s.activeId)
  const activeConnected =
    sessions.find((s) => s.id === activeId)?.status === 'open'
  const sendDisabled = broadcast ? connected === 0 : !activeConnected

  const persist = (next: QuickCommand[]) => {
    setList(next)
    saveQuick(next)
  }

  const startAdd = () => {
    setDraft({ label: '', command: '' })
    setEditing('new')
    setConfirming(null)
  }

  const startEdit = (q: QuickCommand) => {
    setDraft({ label: q.label, command: q.command })
    setEditing(q.id)
    setConfirming(null)
  }

  const cancelEdit = () => {
    setEditing(null)
    setDraft({ label: '', command: '' })
  }

  const saveEdit = () => {
    const label = draft.label.trim()
    const command = draft.command.trim()
    if (!label || !command) return
    if (editing === 'new') {
      const next: QuickCommand[] = [
        ...list,
        { id: 'qc-' + Date.now(), label, command },
      ]
      persist(next)
    } else {
      persist(list.map((q) => (q.id === editing ? { ...q, label, command } : q)))
    }
    cancelEdit()
  }

  const remove = (id: string) => {
    persist(list.filter((q) => q.id !== id))
    setConfirming(null)
  }

  return (
    <div className="flex flex-col h-full bg-gray-900 text-sm">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 text-gray-200">
        <span>快捷命令</span>
        <button
          className="px-2 py-1 rounded text-xs bg-accent text-black"
          onClick={startAdd}
        >
          ＋ 新增
        </button>
      </div>

      {/* 发送目标切换（与快捷输入一致） */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-800">
        <span className="text-xs text-gray-500">发送目标</span>
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
      </div>

      <div className="flex-1 overflow-auto p-2 space-y-1">
        {list.length === 0 && (
          <div className="text-xs text-gray-500">还没有快捷命令，点「新增」创建</div>
        )}
        {list.map((q) => {
          if (editing === q.id) {
            return (
              <div key={q.id} className="rounded bg-gray-800 p-2 space-y-1">
                <input
                  autoFocus
                  className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1 text-xs text-gray-100"
                  placeholder="名称"
                  value={draft.label}
                  onChange={(e) => setDraft({ ...draft, label: e.target.value })}
                />
                <input
                  className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-1 text-xs text-gray-100 font-mono"
                  placeholder="命令"
                  value={draft.command}
                  onChange={(e) => setDraft({ ...draft, command: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') saveEdit()
                  }}
                />
                <div className="flex gap-1">
                  <button
                    className="px-2 py-1 rounded text-xs bg-accent text-black disabled:opacity-50"
                    onClick={saveEdit}
                    disabled={!draft.label.trim() || !draft.command.trim()}
                  >
                    保存
                  </button>
                  <button
                    className="px-2 py-1 rounded text-xs bg-gray-700 text-gray-200"
                    onClick={cancelEdit}
                  >
                    取消
                  </button>
                </div>
              </div>
            )
          }
          return (
            <div
              key={q.id}
              className="flex items-center gap-2 px-2 py-1 rounded bg-gray-800"
            >
              <button
                className="flex-1 text-left overflow-hidden disabled:opacity-50"
                disabled={sendDisabled}
                onClick={() => sendCommand(q.command, broadcast)}
                title={sendDisabled ? '无可发送的主机' : `发送：${q.command}`}
              >
                <div className="text-gray-200 truncate">{q.label}</div>
                <div className="text-xs text-gray-500 font-mono truncate">{q.command}</div>
              </button>
              <button
                className="px-2 py-1 rounded text-xs bg-gray-700 text-gray-200"
                onClick={() => startEdit(q)}
              >
                编辑
              </button>
              {confirming === q.id ? (
                <span className="flex items-center gap-1">
                  <button
                    className="px-2 py-1 rounded text-xs bg-red-600 text-white"
                    onClick={() => remove(q.id)}
                  >
                    确认
                  </button>
                  <button
                    className="px-2 py-1 rounded text-xs bg-gray-700 text-gray-200"
                    onClick={() => setConfirming(null)}
                  >
                    取消
                  </button>
                </span>
              ) : (
                <button
                  className="px-2 py-1 rounded text-xs bg-gray-700 text-red-400"
                  onClick={() => setConfirming(q.id)}
                >
                  删除
                </button>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
