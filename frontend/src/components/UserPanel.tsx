import { useEffect, useState } from 'react'
import { rest } from '../api/rest'
import { useAuthStore } from '../store/authStore'
import ModalDialog from './ModalDialog'
import type { User } from '../types'

const field = 'w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm'
const btn = 'px-2 py-1 rounded text-xs'

interface Lockout {
  ip: string
  fail_count: number
  locked: boolean
  banned: boolean
  retry_after: number
  last_username: string
  updated_at: string
}

// UserPanel: account management moved out of Settings into its own left-rail
// entry. Every user can edit their own account (username + password). Admins
// additionally see and manage all users (rename, change role, reset password,
// create, delete).
export default function UserPanel() {
  const username = useAuthStore((s) => s.user)
  const role = useAuthStore((s) => s.role)
  const setIdentity = useAuthStore((s) => s.setIdentity)
  const isAdmin = role === 'admin'

  const [users, setUsers] = useState<User[]>([])
  const [userErr, setUserErr] = useState('')

  // Login lockouts (admin only).
  const [lockouts, setLockouts] = useState<Lockout[]>([])
  const [lockErr, setLockErr] = useState('')

  // Self-edit form.
  const [selfName, setSelfName] = useState(username)
  const [selfPwd, setSelfPwd] = useState('')
  const [selfMsg, setSelfMsg] = useState('')

  // New-user form (admin) — entered via a modal dialog.
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'user' })
  const [showCreate, setShowCreate] = useState(false)

  // Themed confirm/prompt dialogs (replace native window.prompt / confirm so
  // the popups match the app's dark theme).
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null)
  const [unlockTarget, setUnlockTarget] = useState<string | null>(null)

  // Per-row inline editor state keyed by user id.
  const [editing, setEditing] = useState<Record<number, { username: string; role: string; password: string }>>({})

  useEffect(() => {
    setSelfName(username)
  }, [username])

  const loadUsers = async () => {
    if (!isAdmin) return
    const r = await rest.listUsers()
    if (r.code === 0) setUsers(r.data || [])
    else setUserErr(r.message)
  }

  const loadLockouts = async () => {
    if (!isAdmin) return
    const r = await rest.listLockouts()
    if (r.code === 0) setLockouts(r.data || [])
    else setLockErr(r.message)
  }

  const unlockLockout = async (ip: string) => {
    const r = await rest.unlockLockout(ip)
    if (r.code === 0) await loadLockouts()
    else setLockErr(r.message)
  }

  useEffect(() => {
    void loadUsers()
    void loadLockouts()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isAdmin])

  const saveSelf = async () => {
    setSelfMsg('')
    const r = await rest.updateMe(selfName.trim(), selfPwd)
    if (r.code === 0) {
      setSelfPwd('')
      const tok = r.data?.token
      setIdentity(r.data.user, tok)
      setSelfMsg('已保存')
    } else setSelfMsg(r.message)
  }

  const onCreate = async () => {
    if (!newUser.username || !newUser.password) return
    const r = await rest.createUser(newUser.username, newUser.password, newUser.role)
    if (r.code === 0) {
      setNewUser({ username: '', password: '', role: 'user' })
      setShowCreate(false)
      await loadUsers()
    } else setUserErr(r.message)
  }

  const removeUser = async (id: number) => {
    // Guard against self-deletion: an admin deleting their own account would
    // lock everyone out. The current user's id is resolved from the row.
    const self = users.find((u) => u.username === username)
    if (self && self.id === id) {
      setUserErr('不能删除当前登录的 admin 账户')
      return
    }
    const r = await rest.deleteUser(id)
    if (r.code === 0) await loadUsers()
    else setUserErr(r.message)
  }

  const resetPwd = async (id: number, pwd: string) => {
    if (!pwd) return
    const r = await rest.resetUserPassword(id, pwd)
    if (r.code !== 0) setUserErr(r.message)
  }

  const startEdit = (u: User) =>
    setEditing((e) => ({ ...e, [u.id]: { username: u.username, role: u.role, password: '' } }))

  const cancelEdit = (id: number) =>
    setEditing((e) => {
      const n = { ...e }
      delete n[id]
      return n
    })

  const saveUser = async (u: User) => {
    const d = editing[u.id]
    if (!d) return
    const r = await rest.updateUser(u.id, d.username.trim(), d.role, d.password)
    if (r.code === 0) {
      const tok = (r.data as { token?: string } | undefined)?.token
      if (tok && d.username.trim() !== username) setIdentity(d.username.trim(), tok)
      cancelEdit(u.id)
      await loadUsers()
    } else setUserErr(r.message)
  }

  return (
    <div className="p-4 space-y-5 bg-gray-900 text-sm h-full overflow-auto thin-scroll">
      <div className="text-gray-200 font-semibold text-base">用户管理</div>

      {/* 我的账户：所有登录用户都可修改自己 */}
      <section className="space-y-2 border-b border-gray-800 pb-4">
        <div className="text-gray-400 text-xs uppercase tracking-wide">我的账户</div>
        <div className="text-gray-500 text-xs">
          当前登录：<span className="text-gray-300">{username}</span>（{role}）
        </div>
        <label className="flex items-center justify-between">
          <span>用户名</span>
          <input
            className={field + ' w-48'}
            value={selfName}
            onChange={(e) => setSelfName(e.target.value)}
          />
        </label>
        <label className="flex items-center justify-between">
          <span>新密码</span>
          <input
            type="password"
            className={field + ' w-48'}
            placeholder="留空则不修改"
            value={selfPwd}
            onChange={(e) => setSelfPwd(e.target.value)}
          />
        </label>
        <div className="flex items-center gap-2">
          <button className={btn + ' bg-accent text-black'} onClick={() => void saveSelf()}>
            保存
          </button>
          {selfMsg && <span className="text-xs text-green-400">{selfMsg}</span>}
        </div>
      </section>

      {/* 所有用户：仅 admin 可见可管理 */}
      {isAdmin && (
        <>
        <section className="space-y-2">
          <div className="text-gray-400 text-xs uppercase tracking-wide">所有用户</div>
          {userErr && <div className="text-xs text-red-400">{userErr}</div>}
          <div className="space-y-1">
            {users.map((u) => {
              const d = editing[u.id]
              if (d) {
                return (
                  <div key={u.id} className="rounded bg-gray-800 p-2 space-y-1">
                    <input
                      className={field + ' w-full'}
                      value={d.username}
                      onChange={(e) =>
                        setEditing((p) => ({ ...p, [u.id]: { ...d, username: e.target.value } }))
                      }
                    />
                    <div className="flex gap-1">
                      <select
                        className={field + ' flex-1'}
                        value={d.role}
                        onChange={(e) =>
                          setEditing((p) => ({ ...p, [u.id]: { ...d, role: e.target.value } }))
                        }
                      >
                        <option value="user">user</option>
                        <option value="admin">admin</option>
                      </select>
                      <input
                        type="password"
                        className={field + ' flex-1'}
                        placeholder="新密码（留空不修改）"
                        value={d.password}
                        onChange={(e) =>
                          setEditing((p) => ({ ...p, [u.id]: { ...d, password: e.target.value } }))
                        }
                      />
                    </div>
                    <div className="flex gap-1">
                      <button
                        className={btn + ' bg-accent text-black'}
                        onClick={() => void saveUser(u)}
                      >
                        保存
                      </button>
                      <button
                        className={btn + ' bg-gray-700 text-gray-200'}
                        onClick={() => cancelEdit(u.id)}
                      >
                        取消
                      </button>
                    </div>
                  </div>
                )
              }
              return (
                <div
                  key={u.id}
                  className="flex items-center gap-2 px-2 py-1 rounded bg-gray-800"
                >
                  <span className="flex-1 text-gray-200">
                    {u.username}{' '}
                    <span className="text-[10px] text-gray-500">（{u.role}）</span>
                  </span>
                  <button
                    className={btn + ' bg-gray-700 text-gray-200'}
                    onClick={() => startEdit(u)}
                  >
                    编辑
                  </button>
                  <button
                    className={btn + ' bg-gray-700 text-gray-200'}
                    onClick={() => setResetTarget(u)}
                  >
                    重置密码
                  </button>
                  <button
                    className={btn + ' bg-gray-700 text-red-400 disabled:opacity-40 disabled:cursor-not-allowed'}
                    disabled={u.username === username}
                    title={u.username === username ? '不能删除当前登录账户' : undefined}
                    onClick={() => setDeleteTarget(u)}
                  >
                    删除
                  </button>
                </div>
              )
            })}
          </div>
          <button
            className={btn + ' bg-accent text-black self-start'}
            onClick={() => setShowCreate(true)}
          >
            新增用户
          </button>
        </section>

        <section className="space-y-2 border-t border-gray-800 pt-4">
          <div className="flex items-center justify-between">
            <div className="text-gray-400 text-xs uppercase tracking-wide">登录锁定（按 IP）</div>
            <button
              className={btn + ' bg-gray-700 text-gray-200'}
              onClick={() => void loadLockouts()}
            >
              刷新
            </button>
          </div>
          {lockErr && <div className="text-xs text-red-400">{lockErr}</div>}
          {lockouts.length === 0 && (
            <div className="text-xs text-gray-500">当前没有 IP 被锁定</div>
          )}
          <div className="space-y-1">
            {lockouts.map((l) => {
              const status = l.banned
                ? '永久禁止'
                : l.locked
                  ? `锁定中（${l.retry_after}s）`
                  : '计数但未锁'
              return (
                <div
                  key={l.ip}
                  className="flex items-center gap-2 px-2 py-1 rounded bg-gray-800"
                >
                  <span className="flex-1 text-gray-200 overflow-hidden text-ellipsis whitespace-nowrap">
                    <span className="font-mono">{l.ip}</span>{' '}
                    <span className="text-[10px] text-gray-500">
                      （错误 {l.fail_count} 次{l.last_username ? ` · ${l.last_username}` : ''}）
                    </span>
                  </span>
                  <span
                    className={
                      'text-[10px] px-1.5 py-0.5 rounded ' +
                      (l.banned
                        ? 'bg-red-900 text-red-300'
                        : l.locked
                          ? 'bg-amber-900 text-amber-300'
                          : 'bg-gray-700 text-gray-300')
                    }
                  >
                    {status}
                  </span>
                  <button
                    className={btn + ' bg-gray-700 text-gray-200'}
                    onClick={() => setUnlockTarget(l.ip)}
                  >
                    解锁
                  </button>
                </div>
              )
            })}
          </div>
        </section>
        </>
      )}

      <ModalDialog
        open={showCreate}
        title="新增用户"
        confirmText="创建"
        onCancel={() => {
          setShowCreate(false)
          setNewUser({ username: '', password: '', role: 'user' })
        }}
        onConfirm={() => void onCreate()}
      >
        <div className="space-y-3 mb-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1">用户名</label>
            <input
              autoFocus
              className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 focus:outline-none focus:border-accent"
              value={newUser.username}
              onChange={(e) => setNewUser({ ...newUser, username: e.target.value })}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && newUser.username && newUser.password) void onCreate()
              }}
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">密码</label>
            <input
              type="password"
              className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 focus:outline-none focus:border-accent"
              value={newUser.password}
              onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && newUser.username && newUser.password) void onCreate()
              }}
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">角色</label>
            <select
              className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100"
              value={newUser.role}
              onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
            >
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </div>
        </div>
      </ModalDialog>

      {/* 重置密码：主题化弹窗（替代 window.prompt） */}
      <ModalDialog
        open={resetTarget != null}
        title={`重置密码：${resetTarget?.username ?? ''}`}
        inputLabel="新密码"
        inputType="password"
        inputPlaceholder="输入新密码"
        confirmText="重置"
        onCancel={() => setResetTarget(null)}
        onConfirm={(pwd) => {
          const id = resetTarget?.id
          setResetTarget(null)
          if (id != null) void resetPwd(id, pwd)
        }}
      />

      {/* 删除用户：主题化确认弹窗（替代 window.confirm） */}
      <ModalDialog
        open={deleteTarget != null}
        title="删除用户"
        message={`确定删除用户 “${deleteTarget?.username ?? ''}”？此操作不可撤销。`}
        confirmText="删除"
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() => {
          const id = deleteTarget?.id
          setDeleteTarget(null)
          if (id != null) void removeUser(id)
        }}
      />

      {/* 解锁 IP：主题化确认弹窗（替代 window.confirm） */}
      <ModalDialog
        open={unlockTarget != null}
        title="解锁登录限制"
        message={`确定解锁 IP ${unlockTarget ?? ''} 的登录限制？`}
        confirmText="解锁"
        onCancel={() => setUnlockTarget(null)}
        onConfirm={() => {
          const ip = unlockTarget
          setUnlockTarget(null)
          if (ip != null) void unlockLockout(ip)
        }}
      />
    </div>
  )
}
