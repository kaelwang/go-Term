import { useEffect, useRef, useState } from 'react'
import { Terminal, type ITerminalOptions } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { WebglAddon } from '@xterm/addon-webgl'
import '@xterm/xterm/css/xterm.css'
import { WSClient, wsRegistry } from '../api/ws'
import { useSettingStore } from '../store/settingStore'
import { useTransferStore, isTransferring } from '../store/transferStore'
import { resolveXTermTheme } from './theme'
import { base64ToBytes } from './encoding'
import type { ConnectionSpec } from '../types'

interface Props {
  sessionId: string
  connection: ConnectionSpec
  /** When true, the terminal's internal textarea is auto-focused so the user
   *  can type immediately after tab switching. */
  isActive?: boolean
  onStatus?: (s: string) => void
  onClose?: () => void
}

// XTermView mounts a single xterm.js instance wired to a WSClient session.
export default function XTermView({ sessionId, connection, isActive, onStatus, onClose }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WSClient | null>(null)
  const [closed, setClosed] = useState(false)
  const settings = useSettingStore()

  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    // Guard against React 19 StrictMode double-invoke: the cleanup below
    // disposes and nulls the refs, so a second effect run recreates a fresh
    // terminal instead of stacking two on the same container.
    if (termRef.current) return

    // Per-effect-run disposed flag. Declared locally (not a persistent ref) so
    // the StrictMode double-invoke can't poison it for the real run.
    let disposed = false

    const term = new Terminal({
      cols: connection.initial_cols || 80,
      rows: connection.initial_rows || 24,
      fontSize: settings.fontSize,
      fontFamily: settings.fontFamily,
      cursorBlink: settings.cursorBlink,
      cursorStyle: settings.cursorStyle,
      scrollback: settings.scrollback,
      theme: resolveXTermTheme(settings.theme),
    })
    termRef.current = term

    const fit = new FitAddon()
    fitRef.current = fit
    term.loadAddon(fit)
    term.loadAddon(new SearchAddon())
    term.loadAddon(new WebLinksAddon())

    // WebGL renderer accelerates painting; gracefully fall back to the default
    // canvas renderer when WebGL is unavailable (e.g. headless environments).
    try {
      term.loadAddon(new WebglAddon())
    } catch {
      /* WebGL unavailable; canvas renderer remains active */
    }

    term.open(el)
    fit.fit()
    term.focus()

    loadOptionalAddons(term)

    // Keep the pty window size in sync with the rendered terminal at ALL times.
    // A ResizeObserver on the container catches every case a manual
    // `window.resize` listener misses: background tabs going 0→real size when
    // activated, split panes resizing, scrollbar toggling, and layout settling
    // after mount. Without this the remote pty can keep an obsolete size and
    // full-screen programs (vi / less) or bash line wrapping end up 错位/错行.
    let lastW = -1
    let lastH = -1
    const refit = () => {
      const w = el.clientWidth
      const h = el.clientHeight
      if (w === lastW && h === lastH) return
      lastW = w
      lastH = h
      try {
        fit.fit()
      } catch {
        /* container not laid out yet; retry on next observer tick */
      }
    }
    const ro = new ResizeObserver(refit)
    ro.observe(el)
    refit()

    // Web fonts load asynchronously; the first fit() may have used a fallback
    // font with different glyph metrics, leaving cols/rows out of sync with what
    // is actually rendered. Re-fit once fonts are ready so wrapping aligns.
    if (typeof document !== 'undefined' && document.fonts?.ready) {
      document.fonts.ready.then(() => {
        if (!disposed) refit()
      })
    }

    const ws = new WSClient(sessionId)
    wsRef.current = ws
    wsRegistry.set(sessionId, ws)
    ws.onMessage((msg) => {
      if (msg.type === 'data' && msg.payload?.data) {
        term.write(base64ToBytes(msg.payload.data))
      } else if (msg.type === 'error') {
        onStatus?.('error')
        const text = typeof msg.payload === 'string' ? msg.payload : 'connection error'
        term.write('\r\n\x1b[31m' + text + '\x1b[0m\r\n')
      } else if (msg.type === 'close') {
        onStatus?.('closed')
        term.write('\r\n\x1b[33m[connection closed]\x1b[0m\r\n')
        onClose?.()
      }
    })
    ws.onClose = () => {
      setClosed(true)
      onStatus?.('closed')
    }
    ws.onOpen = () => {
      onStatus?.('open')
      // 连接就绪后立刻把 xterm 的真实窗口尺寸告知后端。首次 fit() 触发的
      // resize 事件发生在 ws 握手完成之前，会被静默丢弃；若不补发，远端
      // pty 会停留在默认的 80x24，vi / less 等全屏程序只会占据屏幕左上角
      // 一小块，表现为"显示和输入只有一部分"。
      try {
        fit.fit()
      } catch {
        /* container not laid out yet */
      }
      ws.sendResize(term.cols, term.rows)
    }
    onStatus?.('connecting')
    ws.connect(connection)

    term.onData((data) => {
      // Disable keyboard input while a file transfer owns the session Conn.
      if (isTransferring(useTransferStore.getState().active, sessionId)) {
        return
      }
      ws.sendInput(new TextEncoder().encode(data))
    })
    term.onResize(({ cols, rows }) => {
      ws.sendResize(cols, rows)
    })

    return () => {
      disposed = true
      ro.disconnect()
      ws.close()
      wsRegistry.delete(sessionId)
      term.dispose()
      termRef.current = null
      fitRef.current = null
      wsRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId])

  // Apply setting changes (theme / font / cursor) without re-creating.
  useEffect(() => {
    const term = termRef.current
    const fit = fitRef.current
    if (!term || !fit) return
    const o = term.options as Partial<ITerminalOptions>
    o.fontSize = settings.fontSize
    o.fontFamily = settings.fontFamily
    o.cursorBlink = settings.cursorBlink
    o.cursorStyle = settings.cursorStyle
    o.scrollback = settings.scrollback
    o.theme = resolveXTermTheme(settings.theme)
    fit.fit()
    if (isActive) term.focus()
  }, [settings.theme, settings.fontSize, settings.fontFamily, settings.cursorBlink, settings.cursorStyle, settings.scrollback])

  // Auto-focus the terminal's hidden textarea when this tab becomes active,
  // so the user can type immediately after switching tabs. The 100 ms delay
  // allows React's render and DOM update (z-index / pointer-events) to settle.
  useEffect(() => {
    if (!isActive) return
    const id = setTimeout(() => {
      termRef.current?.focus()
    }, 100)
    return () => clearTimeout(id)
  }, [isActive])

  // Manual reconnect after a graceful close (e.g. `exit`): re-arm the session
  // and let the WSClient dial again. The terminal instance is reused — only the
  // WebSocket is re-created — so the user keeps their scrollback.
  const handleReconnect = () => {
    setClosed(false)
    onStatus?.('connecting')
    wsRef.current?.reconnect()
  }

  return (
    <div className="relative w-full h-full">
      <div
        ref={containerRef}
        className="w-full h-full"
        onClick={() => termRef.current?.focus()}
      />
      {closed && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/60 z-10">
          <div className="flex flex-col items-center gap-3">
            <span className="text-sm text-gray-300">连接已关闭</span>
            <button
              onClick={handleReconnect}
              className="px-3 py-1.5 text-sm bg-accent text-white rounded hover:opacity-90"
            >
              重新连接
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// Dynamically attempt to load optional image / ligature addons. The
// specifier is held in a variable so Rollup treats it as a runtime-only
// import; when the npm package is not installed the import rejects and we
// silently skip it. The ambient shims in xterm-shims.d.ts provide typing.
const OPTIONAL_ADDONS: Record<string, string> = {
  ImageAddon: '@xterm/addon-image',
  LigaturesAddon: '@xterm/addon-ligatures',
}

async function loadOptionalAddons(term: Terminal): Promise<void> {
  for (const [ctor, spec] of Object.entries(OPTIONAL_ADDONS)) {
    try {
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const mod = await import(/* @vite-ignore */ spec)
      const Addon = (mod as Record<string, new () => { activate(terminal: unknown): void; dispose(): void }>)[ctor]
      if (Addon) {
        try {
          term.loadAddon(new Addon())
        } catch {
          /* addon present but incompatible; ignore */
        }
      }
    } catch {
      /* optional addon not installed */
    }
  }
}
