import { useEffect, useRef, useState } from 'react'
import { useSessionStore } from '../store/sessionStore'
import ModalDialog from '../components/ModalDialog'

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
// The tab strip is width-constrained; when there are more tabs than fit, left /
// right arrows appear and scroll the strip so it never overflows the page.
export default function TabManager({ onNew, onSplit }: Props) {
  const { sessions, activeId, setActive, remove } = useSessionStore()
  const stripRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)
  // The session id pending close confirmation; null when no dialog is open.
  const [pendingClose, setPendingClose] = useState<string | null>(null)

  // Recompute which arrows should be visible based on the strip's scroll state.
  const updateArrows = () => {
    const el = stripRef.current
    if (!el) return
    setCanLeft(el.scrollLeft > 1)
    setCanRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1)
  }

  // Arrow buttons scroll the strip by ~80% of the visible width for a natural,
  // "paged" feel rather than revealing a single tab at a time.
  const scrollBy = (dir: number) => {
    const el = stripRef.current
    if (!el) return
    el.scrollBy({ left: dir * Math.max(120, el.clientWidth * 0.8), behavior: 'smooth' })
  }

  // Keep arrows in sync with content width, window resizes, and scrolling.
  useEffect(() => {
    updateArrows()
    const el = stripRef.current
    if (!el) return
    el.addEventListener('scroll', updateArrows, { passive: true })
    window.addEventListener('resize', updateArrows)
    return () => {
      el.removeEventListener('scroll', updateArrows)
      window.removeEventListener('resize', updateArrows)
    }
  }, [sessions])

  // Bring the active tab into view whenever it changes, so switching to an
  // off-screen tab scrolls the strip instead of leaving it hidden.
  useEffect(() => {
    const strip = stripRef.current
    if (!strip || !activeId) return
    const activeEl = strip.querySelector<HTMLElement>(`[data-session="${activeId}"]`)
    if (!activeEl) return
    const left = activeEl.offsetLeft
    const right = left + activeEl.offsetWidth
    if (left < strip.scrollLeft) strip.scrollLeft = left - 8
    else if (right > strip.scrollLeft + strip.clientWidth) {
      strip.scrollLeft = right - strip.clientWidth + 8
    }
    updateArrows()
  }, [activeId, sessions])

  return (
    <div className="flex items-center h-9 bg-gray-900 border-b border-gray-700">
      {canLeft && (
        <button
          onClick={() => scrollBy(-1)}
          className="shrink-0 px-2 h-full text-gray-300 hover:bg-gray-800 text-xs"
          title="向左滚动页签"
        >
          ◀
        </button>
      )}

      <div
        ref={stripRef}
        className="flex items-center h-full overflow-x-auto no-scrollbar flex-1 min-w-0"
      >
        {sessions.map((s) => {
          const active = s.id === activeId
          return (
            <div
              key={s.id}
              data-session={s.id}
              onClick={() => setActive(s.id)}
              className={
                'group flex items-center gap-2 px-3 h-full cursor-pointer border-r border-gray-700 shrink-0 ' +
                (active ? 'bg-gray-800 text-gray-100' : 'text-gray-400 hover:bg-gray-800')
              }
            >
              <span
                className="inline-block w-2 h-2 rounded-full shrink-0"
                title={STATUS_LABEL[s.status] || '未知'}
                style={{ background: STATUS_COLOR[s.status] || '#6b7280' }}
              />
              <span className="text-sm whitespace-nowrap">{s.name}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  setPendingClose(s.id)
                }}
                className="ml-1 text-gray-500 hover:text-red-400 text-xs"
                title="关闭"
              >
                ✕
              </button>
            </div>
          )
        })}
      </div>

      {canRight && (
        <button
          onClick={() => scrollBy(1)}
          className="shrink-0 px-2 h-full text-gray-300 hover:bg-gray-800 text-xs"
          title="向右滚动页签"
        >
          ▶
        </button>
      )}

      <button
        onClick={onNew}
        className="shrink-0 px-3 h-full text-gray-300 hover:bg-gray-800 text-sm"
        title="新建连接"
      >
        ＋ 新建
      </button>
        <button
          onClick={onSplit}
          disabled={!activeId}
          className="shrink-0 px-3 h-full text-gray-300 hover:bg-gray-800 text-sm disabled:opacity-40"
          title="当前会话分屏"
        >
          ⊟ 分屏
        </button>

      <ModalDialog
        open={pendingClose !== null}
        title="关闭连接"
        message={
          pendingClose
            ? `确定要关闭连接「${sessions.find((s) => s.id === pendingClose)?.name ?? ''}」吗？\n该操作会断开当前会话。`
            : ''
        }
        confirmText="确认"
        cancelText="取消"
        onConfirm={() => {
          if (pendingClose) remove(pendingClose)
          setPendingClose(null)
        }}
        onCancel={() => setPendingClose(null)}
      />
    </div>
  )
}
