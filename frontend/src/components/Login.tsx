import { useEffect, useState } from 'react'
import { useAuthStore } from '../store/authStore'

interface Props {
  version?: string
}

// Login is the gate shown when ENABLE_AUTH=1 and no valid session token is
// present (T-V2). It posts to /api/login and stores the returned JWT.
export default function Login({ version }: Props) {
  const login = useAuthStore((s) => s.login)
  const [user, setUser] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [warn, setWarn] = useState('')
  const [busy, setBusy] = useState(false)
  // Seconds remaining before the brute-force lock on this IP lifts; 0 = unlocked.
  const [lockLeft, setLockLeft] = useState(0)
  const [banned, setBanned] = useState(false)

  // Tick the lock countdown down every second while it's active.
  useEffect(() => {
    if (lockLeft <= 0) return
    const t = setInterval(() => setLockLeft((s) => (s > 0 ? s - 1 : 0)), 1000)
    return () => clearInterval(t)
  }, [lockLeft])

  // When the lock lifts, drop the stale "(N 秒后可重试)" message.
  useEffect(() => {
    if (lockLeft === 0) {
      setError('')
      setBanned(false)
    }
  }, [lockLeft])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (lockLeft > 0) return
    setBusy(true)
    setError('')
    setWarn('')
    try {
      const res = await login(user.trim(), password)
      if (res.ok) return
      let msg = res.message || '用户名或密码错误'
      if (res.banned) {
        setBanned(true)
        msg = res.message || '该 IP 已被永久禁止登录'
      } else if (res.locked && res.retryAfter != null) {
        setLockLeft(res.retryAfter)
        msg = `登录失败：${msg}（${res.retryAfter} 秒后可重试）`
      } else if (res.remaining != null) {
        msg = `登录失败：${msg}（剩余 ${res.remaining} 次重试机会）`
      }
      setError(msg)
      if (res.warn) setWarn(res.warn)
    } catch {
      setError('登录失败，请检查服务是否可用')
    } finally {
      setBusy(false)
    }
  }

  // While locked, keep the live countdown in the error line instead of a stale number.
  const lockMessage =
    lockLeft > 0 ? `该 IP 已被锁定，请 ${lockLeft} 秒后重试` : error

  return (
    <div className="h-screen flex items-center justify-center bg-gray-950 text-gray-200">
      <form
        onSubmit={submit}
        className="w-80 bg-gray-900 border border-gray-700 rounded-lg p-6 shadow-2xl space-y-4"
      >
        <div className="text-center">
          <div className="text-lg font-semibold text-gray-100">go-Term 登录</div>
          {version && <div className="text-xs text-gray-500 mt-1">v{version}</div>}
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">用户名</label>
          <input
            className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
            value={user}
            autoFocus
            disabled={lockLeft > 0}
            onChange={(e) => setUser(e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">密码</label>
          <input
            className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
            type="password"
            value={password}
            disabled={lockLeft > 0}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        {lockMessage && <div className="text-xs text-red-400">{lockMessage}</div>}
        {warn && <div className="text-xs text-amber-400">{warn}</div>}
        <button
          type="submit"
          disabled={busy || !user || !password || lockLeft > 0 || banned}
          className="w-full px-3 py-2 rounded text-sm bg-accent text-black font-medium disabled:opacity-50"
        >
          {lockLeft > 0 ? `${lockLeft}s 后可重试` : busy ? '登录中…' : '登录'}
        </button>
      </form>
    </div>
  )
}
