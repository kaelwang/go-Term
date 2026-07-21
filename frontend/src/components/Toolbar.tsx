import { ENCODINGS } from '../terminal/encoding'
import { colorSchemes } from '../terminal/theme'
import type { ConnectionSpec } from '../types'
import TransferBar from './TransferBar'

interface Props {
  status: string
  theme: string
  onTheme: (key: string) => void
  encoding: string
  onEncoding: (e: string) => void
  fontSize: number
  onFont: (n: number) => void
  onNew: () => void
  // Active session, used to surface the file-transfer toolbar for ssh /
  // localshell connections (F1).
  activeSession?: { id: string; connection: ConnectionSpec } | null
  // Logout + identity (auth-enabled deployments).
  user?: string
  onLogout?: () => void
}

// Toolbar: brand, connection status, encoding, font size, theme + new.
export default function Toolbar(p: Props) {
  const statusCls =
    p.status === 'open'
      ? 'bg-green-700'
      : p.status === 'error'
        ? 'bg-red-700'
        : 'bg-gray-700'
  const showTransfer =
    p.activeSession &&
    (p.activeSession.connection.protocol === 'ssh' ||
      p.activeSession.connection.protocol === 'localshell')
  return (
    <div className="flex items-center gap-3 px-3 h-10 bg-gray-900 border-b border-gray-700 text-sm">
      <span className="font-semibold text-gray-100">go-Term</span>
      <span className={'px-2 rounded text-xs text-white ' + statusCls}>{p.status || '—'}</span>
      {showTransfer && (
        <TransferBar
          sessionId={p.activeSession!.id}
          connection={p.activeSession!.connection}
        />
      )}
      <div className="flex-1" />
      <select
        value={p.encoding}
        onChange={(e) => p.onEncoding(e.target.value)}
        className="bg-gray-800 border border-gray-700 rounded px-1 text-xs"
      >
        {ENCODINGS.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
      <input
        type="number"
        value={p.fontSize}
        onChange={(e) => p.onFont(Number(e.target.value))}
        className="w-14 bg-gray-800 border border-gray-700 rounded px-1 text-xs"
        title="字号"
      />
      <select
        value={p.theme}
        onChange={(e) => p.onTheme(e.target.value)}
        className="bg-gray-800 border border-gray-700 rounded px-1 text-xs"
        title="主题"
      >
        {Object.entries(colorSchemes).map(([key, cs]) => (
          <option key={key} value={key}>
            {cs.label}
          </option>
        ))}
      </select>
      <button
        onClick={p.onNew}
        className="px-2 py-1 bg-accent text-black rounded text-xs"
      >
        ＋ 新建
      </button>
      {p.user && (
        <span className="px-2 py-1 text-xs text-gray-400" title="当前登录用户">
          {p.user}
        </span>
      )}
      {p.onLogout && (
        <button
          onClick={p.onLogout}
          className="px-2 py-1 bg-gray-800 rounded text-xs"
          title="注销"
        >
          登出
        </button>
      )}
    </div>
  )
}
