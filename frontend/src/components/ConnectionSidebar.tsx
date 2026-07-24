import { useEffect, useState } from 'react'
import { rest } from '../api/rest'
import { useConnectionStore } from '../store/connectionStore'
import type {
  ConnectionGroup,
  ConnectionSpec,
  ProtocolType,
  SavedConnection,
} from '../types'
import ModalDialog from './ModalDialog'

interface Props {
  onConnect: (c: ConnectionSpec) => void
  onNewConnection: () => void
  onEdit: (c: SavedConnection) => void
}

// toSpec converts a persisted SavedConnection into the ConnectionSpec sent to
// the WebSocket layer. When the connection references a vault credential, only
// credential_id is forwarded; the backend decrypts the secret at dial time.
function toSpec(c: SavedConnection): ConnectionSpec {
  return {
    id: 's' + Date.now() + '-conn-' + c.id,
    protocol: c.protocol as ProtocolType,
    host: c.host,
    port: c.port,
    credential_id: c.credential_id != null ? String(c.credential_id) : undefined,
    initial_cols: 80,
    initial_rows: 24,
    transfer: c.protocol === 'ssh' ? 'sftp' : 'sftp',
    ssh_config_host: c.ssh_config_host || undefined,
    tunnel: c.tunnel ?? (c.options as { tunnel?: { type: string; local_addr: string; remote_addr: string } })?.tunnel,
  }
}

function byOrder<T extends { sort_order: number; id: number; name: string }>(
  a: T,
  b: T,
): number {
  if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
  if (a.name !== b.name) return a.name.localeCompare(b.name)
  return a.id - b.id
}

interface DialogState {
  title: string
  message?: string
  inputLabel?: string
  inputDefault?: string
  confirmText?: string
  onConfirm: (value: string) => void
}

// ConnectionSidebar lists the user's saved connections grouped into
// collapsible folders (T-V4). Clicking a connection opens a terminal session;
// groups can be created, reordered (up/down) and deleted.
export default function ConnectionSidebar({ onConnect, onNewConnection, onEdit }: Props) {
  // Data comes from the shared connectionStore so that saving a connection in
  // ConnectForm (which writes to the store) automatically refreshes this
  // sidebar. Keeping a private local copy here would make the two views drift
  // apart — the original cause of "新建连接后列表不刷新".
  const groups = useConnectionStore((s) => s.groups)
  const conns = useConnectionStore((s) => s.connections)
  const error = useConnectionStore((s) => s.error)
  const refresh = useConnectionStore((s) => s.load)
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set())
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [dialog, setDialog] = useState<DialogState | null>(null)

  useEffect(() => {
    void refresh()
  }, [refresh])

  const toggle = (id: number) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const addGroup = () => {
    setDialog({
      title: '新建分组',
      inputLabel: '分组名称',
      confirmText: '创建',
      onConfirm: async (value) => {
        const name = value.trim()
        setDialog(null)
        if (!name) return
        const r = await rest.createGroup(name)
        if (r.code === 0) refresh()
      },
    })
  }

  const moveGroup = async (g: ConnectionGroup, dir: -1 | 1) => {
    await rest.updateGroup(g.id, { name: g.name, sort_order: g.sort_order + dir })
    refresh()
  }

  const delGroup = (g: ConnectionGroup) => {
    setDialog({
      title: '删除分组',
      message: `删除分组「${g.name}」？\n其中的连接将变为未分组。`,
      confirmText: '删除',
      onConfirm: async () => {
        setDialog(null)
        const r = await rest.deleteGroup(g.id)
        if (r.code === 0) refresh()
      },
    })
  }

  const delConn = (c: SavedConnection) => {
    setDialog({
      title: '删除连接',
      message: `删除连接「${c.name}」？`,
      confirmText: '删除',
      onConfirm: async () => {
        setDialog(null)
        const r = await rest.deleteConnection(c.id)
        if (r.code === 0) refresh()
      },
    })
  }

  // B4: per-row selection toggle (distinct from clicking the row to connect).
  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const allSelected = conns.length > 0 && conns.every((c) => selected.has(c.id))
  const toggleSelectAll = () => {
    setSelected(allSelected ? new Set() : new Set(conns.map((c) => c.id)))
  }

  // B4: move every selected connection into a target group (or 未分组).
  const moveSelectedTo = async (gid: number | null) => {
    const ids = [...selected]
    await Promise.all(ids.map((id) => rest.updateConnection(id, { group_id: gid })))
    setSelected(new Set())
    refresh()
  }

  // B4: batch delete the selected connections.
  const batchDelete = () => {
    setDialog({
      title: '批量删除',
      message: `确认删除选中的 ${selected.size} 个连接？`,
      confirmText: '删除',
      onConfirm: async () => {
        setDialog(null)
        const ids = [...selected]
        await Promise.all(ids.map((id) => rest.deleteConnection(id)))
        setSelected(new Set())
        refresh()
      },
    })
  }

  // B3: rename an existing group via a themed dialog.
  const renameGroup = (g: ConnectionGroup) => {
    setDialog({
      title: '重命名分组',
      inputLabel: '分组名称',
      inputDefault: g.name,
      confirmText: '保存',
      onConfirm: async (value) => {
        const name = value.trim()
        setDialog(null)
        if (!name) return
        const r = await rest.updateGroup(g.id, { name })
        if (r.code === 0) refresh()
      },
    })
  }

  const sortedGroups = [...groups].sort(byOrder)
  const ungrouped = conns.filter((c) => c.group_id == null)
  const connsOf = (gid: number | null) =>
    conns.filter((c) => (c.group_id ?? null) === gid)

  const renderConn = (c: SavedConnection) => (
    <div
      key={c.id}
      className="group flex items-center gap-2 px-2 py-1 rounded hover:bg-gray-800"
    >
      <input
        type="checkbox"
        className="shrink-0"
        checked={selected.has(c.id)}
        onClick={(e) => e.stopPropagation()}
        onChange={() => toggleSelect(c.id)}
      />
      <div className="flex-1 min-w-0 cursor-pointer" onClick={() => onConnect(toSpec(c))}>
        <div className="text-sm text-gray-200 truncate">{c.name}</div>
        <div className="text-[11px] text-gray-500 truncate">
          {c.protocol} {c.host || ''}
          {c.credential_id != null ? ' · 🔐' : ''}
        </div>
      </div>
      <button
        className="opacity-0 group-hover:opacity-100 text-xs text-gray-400 hover:text-accent"
        title="编辑"
        onClick={(e) => {
          e.stopPropagation()
          onEdit(c)
        }}
      >
        ✎
      </button>
      <button
        className="opacity-0 group-hover:opacity-100 text-xs text-gray-400 hover:text-red-400"
        title="删除"
        onClick={(e) => {
          e.stopPropagation()
          delConn(c)
        }}
      >
        ✕
      </button>
    </div>
  )

  return (
    <div className="flex flex-col h-full bg-gray-900 text-sm">
      <div className="px-3 py-2 border-b border-gray-700 flex items-center justify-between">
        <span className="text-gray-200 font-semibold">连接</span>
        <div className="flex gap-1">
          <button
            className="px-2 py-0.5 text-xs bg-gray-800 rounded hover:bg-gray-700"
            onClick={onNewConnection}
            title="新建连接"
          >
            ＋
          </button>
          <button
            className="px-2 py-0.5 text-xs bg-gray-800 rounded hover:bg-gray-700"
            onClick={addGroup}
            title="新建分组"
          >
            分组
          </button>
        </div>
      </div>

      {error && <div className="px-3 py-1 text-xs text-red-400">{error}</div>}

      <div className="flex-1 overflow-auto p-2 space-y-2">
        {selected.size > 0 && (
          <div className="flex flex-wrap items-center gap-2 px-2 py-1 mb-1 bg-gray-800 rounded">
            <span className="text-xs text-gray-300">已选 {selected.size}</span>
            <button
              className="px-2 py-0.5 text-xs bg-gray-700 rounded hover:bg-gray-600"
              onClick={toggleSelectAll}
            >
              {allSelected ? '取消全选' : '全选'}
            </button>
            <select
              className="px-1 py-0.5 text-xs bg-gray-700 rounded"
              value=""
              onChange={(e) => {
                const v = e.target.value
                if (v === '') return
                void moveSelectedTo(v === 'ungrouped' ? null : Number(v))
              }}
            >
              <option value="">移动到分组…</option>
              <option value="ungrouped">未分组</option>
              {groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </select>
            <button
              className="px-2 py-0.5 text-xs bg-red-700 rounded hover:bg-red-600"
              onClick={() => void batchDelete()}
            >
              批量删除
            </button>
          </div>
        )}
        {sortedGroups.map((g) => {
          const open = !collapsed.has(g.id)
          return (
            <div key={g.id} className="rounded bg-gray-800/40">
              <div className="flex items-center justify-between px-2 py-1 bg-gray-800 rounded">
                <button
                  className="flex-1 text-left text-gray-200 font-medium"
                  onClick={() => toggle(g.id)}
                >
                  {open ? '▾' : '▸'} {g.name}
                </button>
                <div className="flex gap-1 text-gray-400">
                  <button
                    className="text-xs hover:text-gray-200"
                    title="重命名分组"
                    onClick={() => renameGroup(g)}
                  >
                    ✎
                  </button>
                  <button
                    className="text-xs hover:text-gray-200"
                    title="上移"
                    onClick={() => moveGroup(g, -1)}
                  >
                    ↑
                  </button>
                  <button
                    className="text-xs hover:text-gray-200"
                    title="下移"
                    onClick={() => moveGroup(g, 1)}
                  >
                    ↓
                  </button>
                  <button
                    className="text-xs hover:text-red-400"
                    title="删除分组"
                    onClick={() => delGroup(g)}
                  >
                    ✕
                  </button>
                </div>
              </div>
              {open && (
                <div className="mt-1 space-y-0.5 pl-1">
                  {connsOf(g.id).map(renderConn)}
                  {connsOf(g.id).length === 0 && (
                    <div className="px-2 py-1 text-xs text-gray-600">空分组</div>
                  )}
                </div>
              )}
            </div>
          )
        })}

        {ungrouped.length > 0 && (
          <div>
            <div className="px-2 py-1 text-xs text-gray-500">未分组</div>
            <div className="space-y-0.5">{ungrouped.map(renderConn)}</div>
          </div>
        )}
      </div>

      <ModalDialog
        open={dialog !== null}
        title={dialog?.title || ''}
        message={dialog?.message}
        inputLabel={dialog?.inputLabel}
        inputDefault={dialog?.inputDefault}
        confirmText={dialog?.confirmText}
        onConfirm={(v) => dialog?.onConfirm(v)}
        onCancel={() => setDialog(null)}
      />
    </div>
  )
}
