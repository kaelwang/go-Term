interface Props {
  sessionName?: string
  protocol?: string
  encoding: string
  cols?: number
  rows?: number
}

// StatusBar: bottom status strip reflecting the active session.
export default function StatusBar(p: Props) {
  return (
    <div className="flex items-center gap-4 px-3 h-7 bg-gray-900 border-t border-gray-700 text-xs text-gray-400">
      <span>会话：{p.sessionName || '无'}</span>
      {p.protocol && <span>协议：{p.protocol}</span>}
      <span>编码：{p.encoding}</span>
      {p.cols ? <span>{p.cols}×{p.rows}</span> : null}
      <div className="flex-1" />
      <span>go-Term</span>
    </div>
  )
}
