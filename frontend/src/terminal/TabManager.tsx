import { useSessionStore } from '../store/sessionStore'

interface Props {
  onNew: () => void
  onSplit: () => void
  onCloseOthers?: (id: string) => void
}

const STATUS_COLOR: Record<string, string> = {
  connecting: '#e5e510',
  open: '#22c55e',
  closed: '#ef4444',
  error: '#ef4444',
}

const STATUS_LABEL: Record<string, string> = {
  connecting: '连接中',
  open: '已连接',
  closed: '已关闭',
  error: '错误',
}

// TabManager renders the session tab bar: switch, close, and new/split actions.
export default function TabManager({ onNew, onSplit }: Props) {
  const { sessions, activeId, setActive, remove } = useSessionStore()

  return (
    <div className="flex items-center h-9 bg-gray-900 border-b border-gray-700 overflow-x-auto">
      {sessions.map((s) => {
        const active = s.id === activeId
        return (
          <div
            key={s.id}
            onClick={() => setActive(s.id)}
            className={
              'group flex items-center gap-2 px-3 h-full cursor-pointer border-r border-gray-700 ' +
              (active ? 'bg-gray-800 text-gray-100' : 'text-gray-400 hover:bg-gray-800')
            }
          >
            <span
              className="inline-block w-2 h-2 rounded-full"
              title={STATUS_LABEL[s.status] || '未知'}
              style={{ background: STATUS_COLOR[s.status] || '#6b7280' }}
            />
            <span className="text-sm whitespace-nowrap">{s.name}</span>
            <button
              onClick={(e) => {
                e.stopPropagation()
                remove(s.id)
              }}
              className="ml-1 text-gray-500 hover:text-red-400 text-xs"
              title="关闭"
            >
              ✕
            </button>
          </div>
        )
      })}
      <button
        onClick={onNew}
        className="px-3 h-full text-gray-300 hover:bg-gray-800 text-sm"
        title="新建连接"
      >
        ＋ 新建
      </button>
      <button
        onClick={onSplit}
        disabled={!activeId}
        className="px-3 h-full text-gray-300 hover:bg-gray-800 text-sm disabled:opacity-40"
        title="当前会话分屏"
      >
        ⊟ 分屏
      </button>
    </div>
  )
}
