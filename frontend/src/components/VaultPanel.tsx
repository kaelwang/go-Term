import { useCallback, useEffect, useState } from 'react'
import { rest } from '../api/rest'
import type { CredentialMeta, CredentialSecret } from '../types'
import ModalDialog from './ModalDialog'

interface EditState {
  id?: number
  name: string
  type: 'password' | 'private_key'
  username: string
  password: string
  private_key: string
  passphrase: string
}

const EMPTY: EditState = {
  name: '',
  type: 'password',
  username: '',
  password: '',
  private_key: '',
  passphrase: '',
}

// VaultPanel manages the per-user credential vault (T-V3). Secrets are stored
// encrypted at rest on the server (AES-GCM) and only ever returned in plaintext
// when explicitly revealed via /api/credentials/:id/secret.
export default function VaultPanel() {
  const [items, setItems] = useState<CredentialMeta[]>([])
  const [editing, setEditing] = useState<EditState | null>(null)
  const [revealed, setRevealed] = useState<Record<number, CredentialSecret>>({})
  const [error, setError] = useState('')
  const [dialog, setDialog] = useState<null | {
    title: string
    message: string
    onConfirm: (value: string) => void
  }>(null)

  const refresh = useCallback(() => {
    rest
      .listCredentials()
      .then((r) => {
        if (r.code === 0) setItems(r.data || [])
        else setError(r.message)
      })
      .catch(() => setError('加载凭证失败'))
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const save = async () => {
    if (!editing || !editing.name) return
    const body: Record<string, unknown> = {
      name: editing.name,
      type: editing.type,
      username: editing.username,
      password: editing.password,
      private_key: editing.private_key,
      passphrase: editing.passphrase,
      meta: '{}',
    }
    const r = editing.id
      ? await rest.updateCredential(editing.id, body)
      : await rest.createCredential(body)
    if (r.code === 0) {
      setEditing(null)
      refresh()
    } else {
      setError(r.message)
    }
  }

  const del = (id: number) => {
    setDialog({
      title: '删除凭证',
      message: '删除该凭证？关联的连接将无法自动登录。',
      onConfirm: async () => {
        setDialog(null)
        const r = await rest.deleteCredential(id)
        if (r.code === 0) {
          setRevealed((prev) => {
            const next = { ...prev }
            delete next[id]
            return next
          })
          refresh()
        } else {
          setError(r.message)
        }
      },
    })
  }

  const reveal = async (id: number) => {
    if (revealed[id]) {
      setRevealed((prev) => {
        const next = { ...prev }
        delete next[id]
        return next
      })
      return
    }
    const r = await rest.getCredentialSecret(id)
    if (r.code === 0 && r.data) {
      setRevealed((prev) => ({ ...prev, [id]: r.data as CredentialSecret }))
    } else {
      setError(r.message)
    }
  }

  const field = 'w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm'
  const label = 'block text-xs text-gray-400 mb-1'

  return (
    <div className="flex flex-col h-full bg-gray-900 text-sm">
      <div className="px-3 py-2 border-b border-gray-700 flex items-center justify-between">
        <span className="text-gray-200 font-semibold">凭证库</span>
        <button
          className="px-2 py-0.5 text-xs bg-gray-800 rounded hover:bg-gray-700"
          onClick={() => setEditing({ ...EMPTY })}
        >
          ＋ 新建
        </button>
      </div>

      {error && <div className="px-3 py-1 text-xs text-red-400">{error}</div>}

      <div className="flex-1 overflow-auto p-2 space-y-2">
        {items.map((c) => {
          const secret = revealed[c.id]
          return (
            <div
              key={c.id}
              className="rounded bg-gray-800/40 border border-gray-800 p-2"
            >
              <div className="flex items-center justify-between">
                <div className="min-w-0">
                  <div className="text-gray-200 truncate">{c.name}</div>
                  <div className="text-[11px] text-gray-500">
                    {c.type === 'password' ? '🔑 密码' : '🗝️ 私钥'}
                    {c.meta?.username ? ' · ' + c.meta.username : ''}
                  </div>
                </div>
                <div className="flex gap-1 text-gray-400">
                  <button
                    className="text-xs hover:text-gray-200"
                    title="查看/隐藏明文"
                    onClick={() => reveal(c.id)}
                  >
                    {secret ? '🙈' : '👁'}
                  </button>
                  <button
                    className="text-xs hover:text-gray-200"
                    title="编辑"
                    onClick={() =>
                      setEditing({
                        id: c.id,
                        name: c.name,
                        type: c.type,
                        username: c.meta?.username || '',
                        password: '',
                        private_key: '',
                        passphrase: '',
                      })
                    }
                  >
                    ✎
                  </button>
                  <button
                    className="text-xs hover:text-red-400"
                    title="删除"
                    onClick={() => del(c.id)}
                  >
                    ✕
                  </button>
                </div>
              </div>
              {secret && (
                <div className="mt-2 space-y-1 text-xs text-gray-300">
                  {secret.username && (
                    <div>
                      <span className="text-gray-500">用户名：</span>
                      {secret.username}
                    </div>
                  )}
                  {secret.password && (
                    <div className="break-all">
                      <span className="text-gray-500">密码：</span>
                      {secret.password}
                    </div>
                  )}
                  {secret.private_key && (
                    <pre className="whitespace-pre-wrap break-all bg-gray-950 rounded p-1 max-h-32 overflow-auto">
                      {secret.private_key}
                    </pre>
                  )}
                  {secret.passphrase && (
                    <div>
                      <span className="text-gray-500">口令：</span>
                      {secret.passphrase}
                    </div>
                  )}
                </div>
              )}
            </div>
          )
        })}
        {items.length === 0 && (
          <div className="px-2 py-4 text-xs text-gray-600 text-center">
            暂无凭证，点击右上角「＋ 新建」
          </div>
        )}
      </div>

      {editing && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="w-[480px] max-h-[90vh] overflow-auto bg-gray-900 border border-gray-700 rounded-lg p-5 shadow-2xl space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold text-gray-100">
                {editing.id ? '编辑凭证' : '新建凭证'}
              </h2>
              <button
                className="text-gray-400 hover:text-gray-200"
                onClick={() => setEditing(null)}
              >
                ✕
              </button>
            </div>
            <div>
              <label className={label}>名称</label>
              <input
                className={field}
                value={editing.name}
                placeholder="例如 生产跳板机"
                onChange={(e) => setEditing({ ...editing, name: e.target.value })}
              />
            </div>
            <div>
              <label className={label}>类型</label>
              <select
                className={field}
                value={editing.type}
                onChange={(e) =>
                  setEditing({ ...editing, type: e.target.value as 'password' | 'private_key' })
                }
              >
                <option value="password">密码</option>
                <option value="private_key">私钥</option>
              </select>
            </div>
            <div>
              <label className={label}>用户名</label>
              <input
                className={field}
                value={editing.username}
                onChange={(e) => setEditing({ ...editing, username: e.target.value })}
              />
            </div>
            {editing.type === 'password' ? (
              <div>
                <label className={label}>密码</label>
                <input
                  className={field}
                  type="password"
                  value={editing.password}
                  placeholder={editing.id ? '（留空则不修改）' : ''}
                  onChange={(e) => setEditing({ ...editing, password: e.target.value })}
                />
              </div>
            ) : (
              <>
                <div>
                  <label className={label}>私钥 (PEM)</label>
                  <textarea
                    className={field + ' h-28'}
                    value={editing.private_key}
                    placeholder={editing.id ? '（留空则不修改）' : ''}
                    onChange={(e) => setEditing({ ...editing, private_key: e.target.value })}
                  />
                </div>
                <div>
                  <label className={label}>私钥口令（可选）</label>
                  <input
                    className={field}
                    type="password"
                    value={editing.passphrase}
                    onChange={(e) => setEditing({ ...editing, passphrase: e.target.value })}
                  />
                </div>
              </>
            )}
            {error && <div className="text-xs text-red-400">{error}</div>}
            <div className="flex gap-2 pt-1">
              <button
                className="px-3 py-1 rounded text-sm bg-accent text-black"
                onClick={save}
              >
                保存
              </button>
              <button
                className="px-3 py-1 rounded text-sm bg-gray-700 text-gray-200"
                onClick={() => setEditing(null)}
              >
                取消
              </button>
            </div>
          </div>
        </div>
      )}

      <ModalDialog
        open={dialog !== null}
        title={dialog?.title || ''}
        message={dialog?.message}
        confirmText="删除"
        onConfirm={(v) => dialog?.onConfirm(v)}
        onCancel={() => setDialog(null)}
      />
    </div>
  )
}
