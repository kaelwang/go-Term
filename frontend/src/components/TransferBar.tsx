import { useEffect, useRef } from 'react'
import type { ConnectionSpec, TransferProtocol } from '../types'
import { rest } from '../api/rest'
import { wsRegistry } from '../api/ws'
import {
  protocolAvailable,
  useTransferStore,
  type TransferBins,
} from '../store/transferStore'

interface Props {
  sessionId: string
  connection: ConnectionSpec
}

// Protocols offered in the transfer toolbar, in display order.
const PROTOCOLS: { key: TransferProtocol; label: string }[] = [
  { key: 'trzsz', label: 'trzsz' },
]

const STATUS_LABEL: Record<string, string> = {
  running: '进行中',
  done: '完成',
  error: '失败',
}

// TransferBar renders the file-transfer controls for an active terminal
// session: one send/recv pair per protocol. Send uploads the chosen local file
// to the server first, then triggers the WS transfer; recv triggers the WS
// transfer and downloads the resulting file on completion (F1).
export default function TransferBar({ sessionId, connection }: Props) {
  const bins = useTransferStore((s) => s.bins)
  const binsLoaded = useTransferStore((s) => s.binsLoaded)
  const active = useTransferStore((s) => s.active[sessionId])
  const downloaded = useRef<string>('')

  // Resolve external binary availability once on mount.
  useEffect(() => {
    rest
      .transferBins()
      .then((r) => {
        if (r.code === 0 && r.data) {
          useTransferStore.getState().setBins(r.data as TransferBins)
        }
      })
      .catch(() => {
        /* keep defaults */
      })
  }, [])

  // When a recv transfer finishes, download the server-side file.
  useEffect(() => {
    if (
      active &&
      active.direction === 'recv' &&
      active.status === 'done' &&
      active.path
    ) {
      if (downloaded.current === active.path) return
      downloaded.current = active.path
      const path = active.path
      ;(async () => {
        try {
          const res = await fetch(rest.transferFileUrl(path))
          const ct = res.headers.get('content-type') || ''
          if (ct.includes('application/json')) {
            // Backend returned an error envelope, not the file.
            const body = (await res.json().catch(() => null)) as
              | { message?: string }
              | null
            useTransferStore.getState().onStatus(sessionId, {
              protocol: active.protocol,
              direction: 'recv',
              status: 'error',
              error: body?.message || 'download failed',
            })
            return
          }
          const blob = await res.blob()
          const url = URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = path.split('/').pop() || 'download'
          document.body.appendChild(a)
          a.click()
          a.remove()
          URL.revokeObjectURL(url)
        } catch (e) {
          useTransferStore.getState().onStatus(sessionId, {
            protocol: active.protocol,
            direction: 'recv',
            status: 'error',
            error: String(e),
          })
        }
      })()
    }
  }, [active])

  const running = active?.status === 'running'

  const handleSend = (protocol: TransferProtocol) => {
    const input = document.createElement('input')
    input.type = 'file'
    input.onchange = async () => {
      const file = input.files?.[0]
      if (!file) return
      try {
        const r = await rest.transferUpload(file)
        if (r.code !== 0 || !r.data?.path) {
          useTransferStore.getState().onStatus(sessionId, {
            protocol,
            direction: 'send',
            status: 'error',
            error: r.message,
          })
          return
        }
        const ws = wsRegistry.get(sessionId)
        if (!ws) return
        useTransferStore.getState().startTransfer(sessionId, protocol, 'send')
        ws.sendTransfer({ protocol, direction: 'send', file: r.data.path })
      } catch (e) {
        useTransferStore.getState().onStatus(sessionId, {
          protocol,
          direction: 'send',
          status: 'error',
          error: String(e),
        })
      }
    }
    input.click()
  }

  const handleRecv = (protocol: TransferProtocol) => {
    const ws = wsRegistry.get(sessionId)
    if (!ws) return
    useTransferStore.getState().startTransfer(sessionId, protocol, 'recv')
    ws.sendTransfer({ protocol, direction: 'recv', file: '' })
  }

  const btn = 'px-2 py-1 rounded text-xs'

  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="text-gray-400">传输:</span>
      {PROTOCOLS.map((p) => {
        const ok = protocolAvailable(p.key, bins)
        const disabled = running || (binsLoaded && !ok)
        const tip =
          binsLoaded && !ok
            ? `服务器缺少 ${p.key} 所需二进制（请安装 trzsz 或设置 GOTERM_*_BIN 环境变量）`
            : running
              ? '传输进行中，请稍候'
              : ''
        return (
          <div key={p.key} className="flex items-center gap-1" title={tip}>
            <span className="text-gray-300">{p.label}</span>
            <button
              className={btn + ' bg-gray-700 text-gray-200 disabled:opacity-40'}
              disabled={disabled}
              onClick={() => handleRecv(p.key)}
            >
              收
            </button>
            <button
              className={btn + ' bg-gray-700 text-gray-200 disabled:opacity-40'}
              disabled={disabled}
              onClick={() => handleSend(p.key)}
            >
              发
            </button>
          </div>
        )
      })}
      {active && (
        <span
          className={
            'ml-1 ' +
            (active.status === 'error'
              ? 'text-red-400'
              : active.status === 'done'
                ? 'text-green-400'
                : 'text-yellow-400')
          }
        >
          {active.protocol} {active.direction === 'send' ? '发送' : '接收'} ·{' '}
          {STATUS_LABEL[active.status] ?? active.status}
          {active.status === 'error' && active.error ? `: ${active.error}` : ''}
        </span>
      )}
    </div>
  )
}
