import type { ConnectionSpec, WsMessage } from '../types'
import { base64ToBytes, bytesToBase64 } from '../terminal/encoding'
import { useTransferStore } from '../store/transferStore'
import { transferStatusOf } from '../api/rest'
import { getItemWithMigration, tokenKey } from '../lib/storage'

export type WsListener = (msg: WsMessage) => void

// wsRegistry holds live WSClient instances keyed by session id so auxiliary
// UI (quick input, quick commands) can send data to the active terminal.
export const wsRegistry = new Map<string, WSClient>()

// WSClient manages a single terminal WebSocket session: connect, send input /
// resize, dispatch incoming envelopes, and auto-reconnect with a heartbeat.
export class WSClient {
  private ws: WebSocket | null = null
  private listeners: WsListener[] = []
  private autoReconnect = true
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private spec: ConnectionSpec | null = null

  readonly sessionId: string
  onClose?: () => void
  onOpen?: () => void

  constructor(public readonly id: string) {
    this.sessionId = id
  }

  private get url(): string {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const tok = getItemWithMigration(tokenKey) || ''
    return `${proto}://${location.host}/ws?token=${encodeURIComponent(tok)}`
  }

  onMessage(cb: WsListener): void {
    this.listeners.push(cb)
  }

  private raw(msg: WsMessage): void {
    const state = this.ws?.readyState
    console.log('[WSClient] raw called, type=', msg.type, 'readyState=', state, 'ws=', !!this.ws)
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
      console.log('[WSClient] sent:', msg.type)
    } else {
      console.warn('[WSClient] dropped (not OPEN):', msg.type, 'readyState=', state)
    }
  }

  connect(spec: ConnectionSpec): void {
    this.spec = spec
    this.ws = new WebSocket(this.url)
    this.ws.onopen = () => {
      this.startHeartbeat()
      // Transport is up and we are about to send the connect request. Signal
      // the UI so the session dot turns green ("connected normally").
      this.onOpen?.()
      this.raw({
        type: 'connect',
        session: this.sessionId,
        payload: { connection: spec },
      })
    }
    this.ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data as string) as WsMessage
        // A server-sent `close` means the remote shell exited (e.g. the user
        // typed `exit`). Treat it as a graceful disconnect and suppress the
        // auto-reconnect loop — the user must click "重新连接" manually.
        if (msg.type === 'close') {
          this.autoReconnect = false
        }
        // Forward terminal file-transfer status updates to the store so the
        // TransferBar can reflect progress and trigger downloads (F1).
        if (msg.type === 'transfer_status') {
          const status = transferStatusOf(msg.payload)
          if (status) {
            useTransferStore.getState().onStatus(this.sessionId, status)
          }
        }
        this.listeners.forEach((l) => l(msg))
      } catch {
        /* ignore malformed frames */
      }
    }
    this.ws.onerror = () => {
      this.ws?.close()
    }
    this.ws.onclose = () => {
      this.stopHeartbeat()
      this.onClose?.()
      if (this.autoReconnect && this.spec) {
        this.reconnectTimer = setTimeout(() => this.connect(this.spec!), 1500)
      }
    }
  }

  private startHeartbeat(): void {
    this.heartbeatTimer = setInterval(() => {
      this.raw({ type: 'keepalive', session: this.sessionId })
    }, 15000)
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  sendInput(bytes: Uint8Array): void {
    this.raw({
      type: 'input',
      session: this.sessionId,
      payload: { data: bytesToBase64(bytes) },
    })
  }

  sendResize(cols: number, rows: number): void {
    this.raw({ type: 'resize', session: this.sessionId, payload: { cols, rows } })
  }

  // sendTransfer requests a file transfer (trzsz) over the live
  // session. The server takes exclusive ownership of the Conn for the duration
  // of the transfer (F1 / A3).
  sendTransfer(req: {
    protocol: 'trzsz'
    direction: 'send' | 'recv'
    file: string
  }): void {
    this.raw({ type: 'transfer', session: this.sessionId, payload: req })
  }

  close(): void {
    this.autoReconnect = false
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.stopHeartbeat()
    this.ws?.close()
    this.ws = null
  }

  // reconnect re-opens the session after a graceful (server-initiated) close,
  // e.g. the user typed `exit` and dismissed the auto-reconnect. It re-arms
  // auto-reconnect (for genuine transport drops) and dials again.
  reconnect(): void {
    if (!this.spec) return
    this.autoReconnect = true
    this.connect(this.spec)
  }
}

export { base64ToBytes }
