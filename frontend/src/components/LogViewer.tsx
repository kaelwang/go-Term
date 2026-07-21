import { useLogStore } from '../store/logStore'

// LogViewer: renders the shared terminal/system log.
export default function LogViewer() {
  const logs = useLogStore((s) => s.logs)
  const clear = useLogStore((s) => s.clear)
  return (
    <div className="flex flex-col h-full bg-gray-900">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700">
        <span className="text-sm text-gray-200">终端日志</span>
        <button onClick={clear} className="text-xs text-gray-400 hover:text-gray-200">
          清空
        </button>
      </div>
      <div className="flex-1 overflow-auto text-xs font-mono p-2 space-y-1">
        {logs.length === 0 && <div className="text-gray-600">暂无日志</div>}
        {logs.map((l) => (
          <div
            key={l.ts}
            className={
              l.level === 'error'
                ? 'text-red-400'
                : l.level === 'warn'
                  ? 'text-yellow-300'
                  : 'text-gray-400'
            }
          >
            <span className="text-gray-600">{new Date(l.ts).toLocaleTimeString()}</span>{' '}
            {l.msg}
          </div>
        ))}
      </div>
    </div>
  )
}
