// Shared TypeScript types mirroring the backend Go structs.

export type ProtocolType = 'ssh' | 'telnet' | 'vnc' | 'localshell'

export interface Credential {
  type: string
  username: string
  password?: string
  private_key?: string
  passphrase?: string
  answers?: string[]
}

// ---- Backend-backed entities (T-V2 .. T-V5) ----

export interface User {
  id: number
  username: string
  role: 'admin' | 'user'
  created_at: string
}

export interface ConnectionGroup {
  id: number
  user_id: number
  name: string
  parent_id: number | null
  sort_order: number
}

export interface SavedConnection {
  id: number
  user_id: number
  group_id: number | null
  name: string
  protocol: ProtocolType
  host: string
  port: number
  username: string
  auth_type: string
  credential_id: number | null
  ssh_config_host?: string
  proxy?: any
  hops?: any
  options?: any
  tunnel?: TunnelConfig
  created_at: string
  updated_at: string
}

export interface CredentialMeta {
  id: number
  name: string
  type: 'password' | 'private_key'
  meta: any
  created_at: string
}

export interface CredentialSecret {
  username: string
  password?: string
  private_key?: string
  passphrase?: string
}

export interface UserSettings {
  /** Color-scheme key (see colorSchemes in terminal/theme.ts). Legacy 'dark'/'light' still resolve. */
  theme: string
  fontSize: number
  fontFamily: string
  encoding: string
  cursorBlink: boolean
  cursorStyle: 'block' | 'bar' | 'underline'
  scrollback: number
  webgl: boolean
  lineHeight: number
  letterSpacing: number
  defaultProtocol: ProtocolType
  defaultAuthType: string
  defaultTransfer: 'sftp' | 'ftp'
  recvAutoDownload: boolean
  strictHostKeyChecking: boolean
  connectTimeoutSec: number
}

export interface ProxyConfig {
  host: string
  port: number
  username: string
  password?: string
  private_key?: string
  passphrase?: string
  use_agent?: boolean
}

export interface HopConfig {
  host: string
  port: number
  username: string
  password?: string
  private_key?: string
  passphrase?: string
  use_agent?: boolean
}

export interface TunnelConfig {
  type: string
  local_addr: string
  remote_addr: string
}

export interface ConnectionSpec {
  id: string
  protocol: ProtocolType
  host: string
  port: number
  credential?: Credential
  initial_cols?: number
  initial_rows?: number
  strict_host_key_checking?: boolean
  known_hosts_path?: string
  command?: string
  proxy?: ProxyConfig
  hops?: HopConfig[]
  tunnel?: TunnelConfig
  transfer?: 'sftp' | 'ftp'
  // ssh_config_host references a Host alias declared in the server's
  // ~/.ssh/config. When set, the backend applies that alias's
  // HostName/Port/User/IdentityFile/ProxyJump/StrictHostKeyChecking, with any
  // explicitly filled fields taking precedence.
  ssh_config_host?: string
  // credential_id references a saved vault credential (T-V3). When set the
  // backend resolves the secret server-side at dial time; do not send an
  // inline credential alongside it. It is a string over the WS envelope.
  credential_id?: string
}

// ---- Auth / identity DTOs (T-V2 / C7) ----

export interface PublicConfig {
  auth_enabled: boolean
  version: string
}

export interface LoginResponse {
  token: string
  user: string
  role: 'admin' | 'user'
}

export interface MeResponse {
  user: string
  role: 'admin' | 'user'
}

// Terminal file-transfer protocols supported over the live session.
export type TransferProtocol = 'trzsz'
export type TransferDirection = 'send' | 'recv'
export type TransferStatus = 'running' | 'done' | 'error'

// TransferRequestPayload is sent client -> server as the body of a WS
// "transfer" message.
export interface TransferRequestPayload {
  protocol: TransferProtocol
  direction: TransferDirection
  // For send: server-side temp path returned by /api/transfer-upload.
  // For recv: optional output name; empty lets the server use its DownloadDir.
  file: string
}

// TransferStatusPayload is the body of a server -> client "transfer_status"
// WS message.
export interface TransferStatusPayload {
  protocol: TransferProtocol
  direction: TransferDirection
  status: TransferStatus
  error?: string
  path?: string
}

export interface FileEntry {
  name: string
  path: string
  size: number
  mode: number
  mod_time: number
  is_dir: boolean
  is_symlink: boolean
}

export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

export interface WsMessage {
  type: string
  session?: string
  payload?: any
}

export const ErrorCode = {
  OK: 0,
  AuthFail: 1001,
  PermissionDenied: 1002,
  ConnFail: 2001,
  AuthRejected: 2002,
  HostKey: 2003,
  Unsupported: 2004,
  TransferFail: 3001,
  BadParam: 4001,
} as const
