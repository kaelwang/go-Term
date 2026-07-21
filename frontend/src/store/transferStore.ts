import { create } from 'zustand'
import type {
  TransferDirection,
  TransferProtocol,
  TransferStatus,
  TransferStatusPayload,
} from '../types'

// ActiveTransfer is the live state of a single terminal file transfer for a
// session. Only one transfer runs per session at a time (the server enforces
// exclusive Conn ownership via gateMu).
export interface ActiveTransfer {
  protocol: TransferProtocol
  direction: TransferDirection
  status: TransferStatus
  error?: string
  path?: string
}

// TransferBins reports which external transfer tools exist on the server.
export interface TransferBins {
  trz: boolean
  tsz: boolean
}

interface TransferStoreState {
  bins: TransferBins
  binsLoaded: boolean
  active: Record<string, ActiveTransfer>
  setBins: (bins: TransferBins) => void
  startTransfer: (
    sessionId: string,
    protocol: TransferProtocol,
    direction: TransferDirection,
  ) => void
  onStatus: (sessionId: string, payload: TransferStatusPayload) => void
  clearActive: (sessionId: string) => void
}

export const useTransferStore = create<TransferStoreState>((set) => ({
  // Assume available until the server reports otherwise, so buttons are not
  // prematurely greyed out before /api/transfer-bins resolves.
  bins: { trz: true, tsz: true },
  binsLoaded: false,
  active: {},
  setBins: (bins) => set({ bins, binsLoaded: true }),
  startTransfer: (sessionId, protocol, direction) =>
    set((s) => ({
      active: {
        ...s.active,
        [sessionId]: { protocol, direction, status: 'running' },
      },
    })),
  onStatus: (sessionId, payload) =>
    set((s) => ({
      active: {
        ...s.active,
        [sessionId]: {
          protocol: payload.protocol,
          direction: payload.direction,
          status: payload.status,
          error: payload.error,
          path: payload.path,
        },
      },
    })),
  clearActive: (sessionId) =>
    set((s) => {
      const next = { ...s.active }
      delete next[sessionId]
      return { active: next }
    }),
}))

// protocolAvailable reports whether a transfer protocol can run given the
// server's available binaries. trzsz requires both the send and recv external
// tools (F1 / A1).
export function protocolAvailable(
  protocol: TransferProtocol,
  bins: TransferBins,
): boolean {
  switch (protocol) {
    case 'trzsz':
      return bins.trz && bins.tsz
    default:
      return false
  }
}

// isTransferring reports whether the given session currently owns its Conn for
// an in-flight transfer (used to disable terminal input).
export function isTransferring(
  active: Record<string, ActiveTransfer> | undefined,
  sessionId: string,
): boolean {
  return active?.[sessionId]?.status === 'running'
}
