import { create } from 'zustand'
import { rest } from '../api/rest'
import type { ConnectionGroup, ProtocolType, SavedConnection, TunnelConfig } from '../types'

export interface SaveConnectionInput {
  name: string
  protocol: ProtocolType
  host: string
  port: number
  username: string
  authType: string
  /** Existing vault credential id to reference (skips creating one). */
  credentialId?: number | null
  /** Inline secret: when no vault credential is referenced, a new one is created. */
  password?: string
  privateKey?: string
  passphrase?: string
  groupId?: number | null
  /** SSH config Host alias (server ~/.ssh/config). Optional, user-editable. */
  sshConfigHost?: string
  /** Optional SSH port-forwarding (tunnel) rule, persisted inside the options JSON column. */
  tunnel?: TunnelConfig
}

interface ConnectionState {
  groups: ConnectionGroup[]
  connections: SavedConnection[]
  loading: boolean
  error: string
  /** Fetch groups + connections from the backend (idempotent refresh). */
  load: () => Promise<void>
  createGroup: (name: string) => Promise<boolean>
  renameGroup: (id: number, name: string) => Promise<boolean>
  deleteGroup: (id: number) => Promise<boolean>
  /** Move a group up/down relative to its neighbour (C6: up/down buttons). */
  moveGroup: (id: number, dir: 'up' | 'down') => Promise<boolean>
  deleteConnection: (id: number) => Promise<boolean>
  /**
   * Persist a connection. If no vault credential is referenced but an inline
   * secret is supplied, a credential is created first and its id stored as
   * `credential_id` (B3: secrets never live on the connections row). Returns the
   * new connection id, or null on failure.
   */
  saveConnection: (input: SaveConnectionInput) => Promise<number | null>
  /**
   * Update an existing connection (B2: edit a saved connection). The caller
   * passes the connection `id` plus any subset of SaveConnectionInput fields.
   * Vault credentials are resolved the same way as saveConnection: if inline
   * secrets are present, the referenced credential is updated in place (when
   * `credentialId` is set) or a new one is created (when no id is referenced);
   * otherwise the existing `credential_id` reference is preserved. Returns the
   * connection id, or null on failure.
   */
  updateConnection: (input: { id: number } & Partial<SaveConnectionInput>) => Promise<number | null>
}

// connectionStore owns the saved-connection + group data shared between the
// sidebar (browse / reorder / delete) and the connect form (save current).
export const useConnectionStore = create<ConnectionState>((set, get) => ({
  groups: [],
  connections: [],
  loading: false,
  error: '',

  load: async () => {
    set({ loading: true, error: '' })
    try {
      const [g, c] = await Promise.all([rest.listGroups(), rest.listConnections()])
      if (g.code === 0) set({ groups: g.data || [] })
      else set({ error: g.message })
      if (c.code === 0) {
        const conns = (c.data || []).map((x: SavedConnection) => ({
          ...x,
          tunnel: (x.options as { tunnel?: TunnelConfig })?.tunnel,
        }))
        set({ connections: conns })
      }
      else set({ error: c.message })
    } catch (e) {
      set({ error: String(e) })
    } finally {
      set({ loading: false })
    }
  },

  createGroup: async (name) => {
    const r = await rest.createGroup(name)
    if (r.code === 0) {
      await get().load()
      return true
    }
    set({ error: r.message })
    return false
  },

  renameGroup: async (id, name) => {
    const r = await rest.updateGroup(id, { name })
    if (r.code === 0) {
      await get().load()
      return true
    }
    set({ error: r.message })
    return false
  },

  deleteGroup: async (id) => {
    const r = await rest.deleteGroup(id)
    if (r.code === 0) {
      await get().load()
      return true
    }
    set({ error: r.message })
    return false
  },

  moveGroup: async (id, dir) => {
    const sorted = [...get().groups].sort(
      (a, b) => a.sort_order - b.sort_order || a.id - b.id,
    )
    const idx = sorted.findIndex((g) => g.id === id)
    const swapIdx = dir === 'up' ? idx - 1 : idx + 1
    if (idx < 0 || swapIdx < 0 || swapIdx >= sorted.length) return false
    const a = sorted[idx]
    const b = sorted[swapIdx]
    // Swap sort_order between the two neighbours so the order is stable.
    await rest.updateGroup(a.id, { name: a.name, sort_order: b.sort_order })
    await rest.updateGroup(b.id, { name: b.name, sort_order: a.sort_order })
    await get().load()
    return true
  },

  deleteConnection: async (id) => {
    const r = await rest.deleteConnection(id)
    if (r.code === 0) {
      await get().load()
      return true
    }
    set({ error: r.message })
    return false
  },

  saveConnection: async (input) => {
    try {
      let credId = input.credentialId ?? null
      if (credId == null && (input.password || input.privateKey)) {
        const c = await rest.createCredential({
          name: input.name,
          type: input.authType === 'publickey' ? 'private_key' : 'password',
          username: input.username,
          password: input.password || '',
          private_key: input.privateKey || '',
          passphrase: input.passphrase || '',
        })
        if (c.code === 0 && c.data) credId = c.data.id
      }
      const r = await rest.createConnection({
        name: input.name,
        protocol: input.protocol,
        host: input.host,
        port: input.port,
        username: input.username,
        auth_type: input.authType,
        credential_id: credId,
        group_id: input.groupId ?? null,
        ssh_config_host: input.sshConfigHost || null,
        options: input.tunnel ? { tunnel: input.tunnel } : {},
      })
      if (r.code === 0) {
        await get().load()
        return r.data?.id ?? null
      }
      set({ error: r.message })
      return null
    } catch (e) {
      set({ error: String(e) })
      return null
    }
  },

  updateConnection: async (input) => {
    const id = input.id
    try {
      let credId: number | null = input.credentialId ?? null
      // Resolve the vault credential when inline secrets are supplied: update
      // the referenced credential in place, or create a new one if none is
      // referenced. Mirrors saveConnection's secret handling.
      if (input.password || input.privateKey) {
        if (credId != null) {
          await rest.updateCredential(credId, {
            type: input.authType === 'publickey' ? 'private_key' : 'password',
            username: input.username || '',
            password: input.password || '',
            private_key: input.privateKey || '',
            passphrase: input.passphrase || '',
          })
        } else {
          const c = await rest.createCredential({
            name: input.name || 'connection',
            type: input.authType === 'publickey' ? 'private_key' : 'password',
            username: input.username || '',
            password: input.password || '',
            private_key: input.privateKey || '',
            passphrase: input.passphrase || '',
          })
          if (c.code === 0 && c.data) credId = c.data.id
        }
      }
      // Build a partial patch so callers (e.g. the connect form) only need to
      // send the fields that changed.
      const patch: Record<string, unknown> = {}
      if (input.name !== undefined) patch.name = input.name
      if (input.protocol !== undefined) patch.protocol = input.protocol
      if (input.host !== undefined) patch.host = input.host
      if (input.port !== undefined) patch.port = input.port
      if (input.username !== undefined) patch.username = input.username
      if (input.authType !== undefined) patch.auth_type = input.authType
      if (input.groupId !== undefined) patch.group_id = input.groupId ?? null
      // Only write credential_id when it resolves to a valid numeric id:
      // either an existing vault reference (input.credentialId) or one that
      // was resolved/created from inline secrets above. When credId is
      // null/undefined we OMIT the key on purpose so the backend's
      // read-modify-write preserves the connection's existing credential
      // reference — this prevents silently clearing the credential to NULL
      // when the form simply has no credential selected.
      if (typeof credId === 'number' && credId >= 0) {
        patch.credential_id = credId
      }
      // ssh_config_host is user-editable in the form. When supplied we pass it
      // through so the backend overwrites it (read-modify-write keeps the
      // existing value when it is not provided). This stops an edited alias
      // from being lost on save.
      if (input.sshConfigHost !== undefined) {
        patch.ssh_config_host = input.sshConfigHost
      }
      // Port-forwarding (tunnel) rule, stored inside the options JSON column.
      // Always written when present so disabling a previously saved tunnel
      // clears it (the store's read-modify-write keeps other option sub-keys).
      if (input.tunnel !== undefined) {
        patch.options = input.tunnel ? { tunnel: input.tunnel } : {}
      }
      const r = await rest.updateConnection(id, patch)
      if (r.code === 0) {
        await get().load()
        return r.data?.id ?? id
      }
      set({ error: r.message })
      return null
    } catch (e) {
      set({ error: String(e) })
      return null
    }
  },
}))
