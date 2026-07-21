import { useSettingStore } from '../store/settingStore'
import { colorSchemes } from '../terminal/theme'
import type { ProtocolType } from '../types'

const FONT_OPTIONS = [
  'Menlo, Monaco, Consolas, "Courier New", monospace',
  '"Fira Code", "Cascadia Code", Consolas, monospace',
  '"JetBrains Mono", Consolas, monospace',
  'Consolas, "Courier New", monospace',
  'monospace',
]

const PROTOCOL_OPTIONS: ProtocolType[] = ['ssh', 'telnet', 'vnc', 'localshell']
const AUTH_OPTIONS = ['password', 'publickey', 'keyboard-interactive', 'agent']

const field = 'w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm'
const btn = 'px-2 py-1 rounded text-xs'

// SettingsPanel: per-user settings (persisted via GET/PUT /api/settings).
// Account/user management has moved to its own left-rail entry (UserPanel).
export default function SettingsPanel() {
  const s = useSettingStore()

  return (
    <div className="p-4 space-y-5 bg-gray-900 text-sm h-full overflow-auto thin-scroll">
      <div className="text-gray-200 font-semibold text-base">设置</div>

      {/* 终端外观 */}
      <section className="space-y-2">
        <div className="text-gray-400 text-xs uppercase tracking-wide">终端外观</div>
        <label className="flex items-center justify-between">
          <span>字体族</span>
          <select
            className={field + ' w-64'}
            value={s.fontFamily}
            onChange={(e) => s.set({ fontFamily: e.target.value })}
          >
            {FONT_OPTIONS.map((f) => (
              <option key={f} value={f}>
                {f}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center justify-between">
          <span>字号</span>
          <input
            type="number"
            className={field + ' w-24'}
            value={s.fontSize}
            onChange={(e) => s.set({ fontSize: Number(e.target.value) })}
          />
        </label>
        <label className="flex items-center justify-between">
          <span>配色方案</span>
          <select
            className={field + ' w-64'}
            value={s.theme}
            onChange={(e) => s.set({ theme: e.target.value })}
          >
            {Object.entries(colorSchemes).map(([key, cs]) => (
              <option key={key} value={key}>
                {cs.label}
              </option>
            ))}
          </select>
        </label>
        <div className="flex items-center flex-wrap gap-2 pl-1">
          {Object.entries(colorSchemes).map(([key, cs]) => (
            <button
              key={key}
              title={cs.label}
              onClick={() => s.set({ theme: key })}
              className={
                'w-5 h-5 rounded border ' +
                (s.theme === key ? 'border-accent' : 'border-gray-700')
              }
              style={{ background: cs.theme.background }}
            >
              <span
                className="block w-full h-full rounded"
                style={{ background: cs.theme.foreground, opacity: 0.85 }}
              />
            </button>
          ))}
        </div>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={s.cursorBlink}
            onChange={(e) => s.set({ cursorBlink: e.target.checked })}
          />
          <span>光标闪烁</span>
        </label>
        <label className="flex items-center justify-between">
          <span>光标样式</span>
          <select
            className={field + ' w-32'}
            value={s.cursorStyle}
            onChange={(e) =>
              s.set({ cursorStyle: e.target.value as 'block' | 'bar' | 'underline' })
            }
          >
            <option value="block">块状</option>
            <option value="bar">竖条</option>
            <option value="underline">下划线</option>
          </select>
        </label>
        <label className="flex items-center justify-between">
          <span>滚动缓冲行数</span>
          <input
            type="number"
            className={field + ' w-32'}
            value={s.scrollback}
            onChange={(e) => s.set({ scrollback: Number(e.target.value) })}
          />
        </label>
      </section>

      {/* 默认连接 */}
      <section className="space-y-2">
        <div className="text-gray-400 text-xs uppercase tracking-wide">默认连接</div>
        <label className="flex items-center justify-between">
          <span>默认协议</span>
          <select
            className={field + ' w-40'}
            value={s.defaultProtocol}
            onChange={(e) => s.set({ defaultProtocol: e.target.value as ProtocolType })}
          >
            {PROTOCOL_OPTIONS.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center justify-between">
          <span>默认认证方式</span>
          <select
            className={field + ' w-40'}
            value={s.defaultAuthType}
            onChange={(e) => s.set({ defaultAuthType: e.target.value })}
          >
            {AUTH_OPTIONS.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
      </section>

      {/* 传输默认 */}
      <section className="space-y-2">
        <div className="text-gray-400 text-xs uppercase tracking-wide">传输默认</div>
        <label className="flex items-center justify-between">
          <span>默认传输协议</span>
          <select
            className={field + ' w-40'}
            value={s.defaultTransfer}
            onChange={(e) => s.set({ defaultTransfer: e.target.value as 'sftp' | 'ftp' })}
          >
            <option value="sftp">SFTP</option>
            <option value="ftp">FTP</option>
          </select>
        </label>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={s.recvAutoDownload}
            onChange={(e) => s.set({ recvAutoDownload: e.target.checked })}
          />
          <span>接收（recv）自动下载</span>
        </label>
      </section>

      {/* 安全 */}
      <section className="space-y-2">
        <div className="text-gray-400 text-xs uppercase tracking-wide">安全</div>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={s.strictHostKeyChecking}
            onChange={(e) => s.set({ strictHostKeyChecking: e.target.checked })}
          />
          <span>known_hosts 严格校验（StrictHostKeyChecking）</span>
        </label>
        <label className="flex items-center justify-between">
          <span>连接超时（秒）</span>
          <input
            type="number"
            className={field + ' w-24'}
            value={s.connectTimeoutSec}
            onChange={(e) => s.set({ connectTimeoutSec: Number(e.target.value) })}
          />
        </label>
      </section>
    </div>
  )
}
