import { useEffect, useState } from 'react'
import { useSessionStore } from './store/sessionStore'
import { useSettingStore } from './store/settingStore'
import { useLogStore } from './store/logStore'
import { useAuthStore } from './store/authStore'
import { migrateStorageKeys } from './lib/storage'
import { rest } from './api/rest'
import { wsRegistry } from './api/ws'
import { schemeUi } from './terminal/theme'
import type { ConnectionSpec, SavedConnection } from './types'
import QuickCommandPanel from './components/QuickCommandPanel'

import XTermView from './terminal/XTermView'
import TabManager from './terminal/TabManager'
import ConnectForm from './ssh/connectForm'
import FilePanel from './filemanager/FilePanel'
import EditorModal from './filemanager/EditorModal'
import Sidebar from './components/Sidebar'
import Toolbar from './components/Toolbar'
import StatusBar from './components/StatusBar'
import QuickInput from './components/QuickInput'
import LogViewer from './components/LogViewer'
import UserPanel from './components/UserPanel'
import Login from './components/Login'
import ConnectionSidebar from './components/ConnectionSidebar'
import VaultPanel from './components/VaultPanel'
import SettingsPanel from './components/SettingsPanel'

type PanelKey = 'connect' | 'files' | 'vault' | 'quick' | 'logs' | 'settings' | 'users' | null

export default function App() {
  const sessions = useSessionStore((s) => s.sessions)
  const activeId = useSessionStore((s) => s.activeId)
  const addSession = useSessionStore((s) => s.add)
  const updateSession = useSessionStore((s) => s.update)
  const removeSession = useSessionStore((s) => s.remove)
  const setActive = useSessionStore((s) => s.setActive)

  const theme = useSettingStore((s) => s.theme)
  const fontSize = useSettingStore((s) => s.fontSize)
  const encoding = useSettingStore((s) => s.encoding)
  const cursorBlink = useSettingStore((s) => s.cursorBlink)
  const scrollback = useSettingStore((s) => s.scrollback)
  const setSetting = useSettingStore((s) => s.set)

  const user = useAuthStore((s) => s.user)
  const authEnabled = useAuthStore((s) => s.authEnabled)
  const token = useAuthStore((s) => s.token)
  const showLogin = authEnabled && !token

  const addLog = useLogStore((s) => s.add)

  const [panel, setPanel] = useState<PanelKey>('connect')
  const [connectOpen, setConnectOpen] = useState(false)
  // B2: the connection currently being edited (null when creating new).
  const [editingConn, setEditingConn] = useState<SavedConnection | null>(null)
  const [ready, setReady] = useState(false)
  const [split, setSplit] = useState(false)
  const [splitWith, setSplitWith] = useState<string | null>(null)
  const [editor, setEditor] = useState<{ path: string } | null>(null)

  // Bug fix: When a user switches tabs to the session that is currently the
  // splitWith target, exit split mode. Without this, the active tab would
  // become the right panel, the old left panel would vanish (because the
  // activeId no longer matches), and the split layout would be broken.
  useEffect(() => {
    if (split && activeId === splitWith) {
      setSplit(false)
      setSplitWith(null)
    }
  }, [activeId, split, splitWith])

  // Brute-force protection: force a fresh credential challenge on every page
  // load by discarding any persisted session token. The login gate must always
  // require the username and password rather than silently restoring a session.
  useEffect(() => {
    migrateStorageKeys()
    useAuthStore.getState().logout()
  }, [])

  // On startup, ask the no-auth public config whether login is required.
  useEffect(() => {
    void useAuthStore.getState().loadPublicConfig()
    setReady(true)
  }, [])

  // Once the gate is open, restore per-user settings and (re)fetch identity.
  useEffect(() => {
    if (!ready || showLogin) return
    void useSettingStore.getState().load()
    if (token) void useAuthStore.getState().loadMe()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, showLogin, token])

  // Toggle the <html> dark class based on the selected color scheme's ui
  // mode (dark/light) so any Tailwind dark: variants follow the scheme.
  useEffect(() => {
    const root = document.documentElement
    if (schemeUi(theme) === 'dark') root.classList.add('dark')
    else root.classList.remove('dark')
  }, [theme])

  const active = sessions.find((s) => s.id === activeId) || null

  const onConnect = (conn: ConnectionSpec) => {
    const name =
      conn.protocol === 'localshell'
        ? '本地终端'
        : `${conn.protocol} ${conn.host || ''}:${conn.port}`
    addSession({ id: conn.id, name, connection: conn, status: 'connecting' })
    setActive(conn.id)
    addLog({ level: 'info', msg: '🔌 连接 ' + name })
  }

  // B2: open the connect form pre-filled for editing the given connection.
  const onEdit = (c: SavedConnection) => {
    setEditingConn(c)
    setConnectOpen(true)
  }

  const onSplit = () => {
    if (!activeId || !active) return
    const clone: ConnectionSpec = { ...active.connection, id: 's' + Date.now() + '-split' }
    addSession({
      id: clone.id,
      name: active.name + ' (分屏)',
      connection: clone,
      status: 'connecting',
    })
    setSplitWith(clone.id)
    setSplit(true)
  }

  const onStatus = (id: string, status: string) =>
    updateSession(id, { status: status as any })

  const onClose = (id: string) => updateSession(id, { status: 'closed' })

  const logout = () => {
    // Tear down live sessions, then drop the token (gate re-shows Login).
    sessions.forEach((s) => wsRegistry.get(s.id)?.close())
    useAuthStore.getState().logout()
  }

  // Unified rendering: all sessions live in one relative container.
  // In split mode the active and splitWith sessions each get 50% width side-by-side;
  // other sessions stay full-screen stacked behind (z-index 0, pointer-events none).
  // Because every XTermView's DOM parent is always the same <div key={s.id}>,
  // React never unmounts/remounts a terminal — no SSH reconnect on split toggle.
  const renderTerminal = () => {
    if (sessions.length === 0) {
      return (
        <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">
          点击右上角「＋ 新建」发起一个连接
        </div>
      )
    }

    const isSplit = split && splitWith && sessions.some((s) => s.id === splitWith) && active

    return (
      <div className="relative w-full h-full">
        {/* Split divider — pure CSS, fixed at 50%. Dragging is not supported yet. */}
        {isSplit && (
          <div
            className="absolute top-0 bottom-0 bg-gray-600 hover:bg-accent cursor-col-resize z-10"
            style={{ left: '50%', width: 4, transform: 'translateX(-2px)' }}
          />
        )}

        {sessions.map((s) => {
          const inSplit = isSplit && (s.id === activeId || s.id === splitWith)

          return (
            <div
              key={s.id}
              className="absolute h-full"
              style={
                inSplit
                  ? {
                      top: 0,
                      width: 'calc(50% - 2px)',
                      left: s.id === activeId ? 0 : 'calc(50% + 2px)',
                      zIndex: 1,
                      pointerEvents: 'auto',
                    }
                  : {
                      inset: 0,
                      zIndex: s.id === activeId ? 1 : 0,
                      pointerEvents: s.id === activeId ? 'auto' : 'none',
                    }
              }
            >
              <XTermView
                sessionId={s.id}
                connection={s.connection}
                isActive={s.id === activeId}
                onStatus={(st) => onStatus(s.id, st)}
                onClose={() => onClose(s.id)}
              />
            </div>
          )
        })}
      </div>
    )
  }

  const renderPanel = () => {
    switch (panel) {
      case 'connect':
        return <ConnectionSidebar onConnect={onConnect} onNewConnection={() => setConnectOpen(true)} onEdit={onEdit} />
      case 'files':
        return active ? (
          <FilePanel
            connection={active.connection}
            onOpenFile={(p) => setEditor({ path: p })}
          />
        ) : (
          <div className="p-4 text-sm text-gray-500">请先选择一个会话</div>
        )
      case 'vault':
        return <VaultPanel />
      case 'quick':
        return <QuickCommandPanel />
      case 'logs':
        return <LogViewer />
      case 'settings':
        return <SettingsPanel />
      case 'users':
        return <UserPanel />
      default:
        return (
          <div className="p-4 text-sm text-gray-500">
            使用左侧导航切换「连接 / 文件 / 凭证 / 快捷命令 / 日志 / 用户管理 / 设置」。
          </div>
        )
    }
  }

  if (!ready) {
    return (
      <div className="h-screen flex items-center justify-center text-gray-500 text-sm">
        加载中…
      </div>
    )
  }

  if (showLogin) {
    return <Login />
  }

  return (
    <div className="h-screen flex flex-col bg-gray-950 text-gray-200">
      <div className="flex flex-1 min-h-0">
        <Sidebar active={panel || ''} onSelect={(v) => setPanel(v as PanelKey)} />
        <div className="flex-1 flex flex-col min-w-0">
          <Toolbar
            status={active?.status || '—'}
            theme={theme}
            onTheme={(key) => setSetting({ theme: key })}
            encoding={encoding}
            onEncoding={(e) => setSetting({ encoding: e })}
            fontSize={fontSize}
            onFont={(n) => setSetting({ fontSize: n })}
            onNew={() => setConnectOpen(true)}
            activeSession={active}
            user={authEnabled ? user : undefined}
            onLogout={authEnabled ? logout : undefined}
          />
          <TabManager onNew={() => setConnectOpen(true)} onSplit={onSplit} />
          <div className="flex-1 flex min-h-0">
            <div className="flex-1 min-w-0">{renderTerminal()}</div>
            <div className="w-80 border-l border-gray-800 min-h-0 overflow-hidden">
              {renderPanel()}
            </div>
          </div>
          <QuickInput />
          <StatusBar
            sessionName={active?.name}
            protocol={active?.connection.protocol}
            encoding={encoding}
          />
        </div>
      </div>

      {connectOpen && (
        <ConnectForm
          onConnect={onConnect}
          onClose={() => {
            setConnectOpen(false)
            setEditingConn(null)
          }}
          initial={editingConn}
        />
      )}
      {editor && active && (
        <EditorModal
          connection={active.connection}
          path={editor.path}
          onClose={() => setEditor(null)}
        />
      )}
    </div>
  )
}
