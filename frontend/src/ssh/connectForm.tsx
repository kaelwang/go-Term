import { useEffect, useState } from 'react'
import type {
  ConnectionSpec,
  CredentialMeta,
  ProtocolType,
  SavedConnection,
} from '../types'
import { rest } from '../api/rest'
import { useConnectionStore } from '../store/connectionStore'
import type { SaveConnectionInput } from '../store/connectionStore'
import { useSettingStore } from '../store/settingStore'

interface Props {
  onConnect: (c: ConnectionSpec) => void
  onClose: () => void
  /** When set, the form opens in edit mode pre-filled from this connection. */
  initial?: SavedConnection | null
}

const DEFAULT_PORT: Record<ProtocolType, number> = {
  ssh: 22,
  telnet: 23,
  vnc: 5900,
  localshell: 0,
}

// ConnectForm is the modal dialog used to open a new terminal session. It also
// supports persisting the connection (saved to /api/connections, referencing a
// vault credential by id — B3) and reusing an existing vault credential. In
// edit mode (Props.initial set) it pre-fills from the saved connection and
// issues a PUT instead of a POST (B2).
export default function ConnectForm({ onConnect, onClose, initial }: Props) {
  const isEdit = !!(initial && initial.id)
  // groups feeds the "choose a group" dropdown (B1). If the store has not been
  // populated yet, a mount effect triggers a refresh as a fallback.
  const groups = useConnectionStore((s) => s.groups)

  const settings = useSettingStore.getState()

  const [protocol, setProtocol] = useState<ProtocolType>(settings.defaultProtocol || 'ssh')
  const [host, setHost] = useState('')
  const [port, setPort] = useState(22)
  const [username, setUsername] = useState('')
  const [authType, setAuthType] = useState(settings.defaultAuthType || 'password')
  const [password, setPassword] = useState('')
  const [privateKey, setPrivateKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [transfer, setTransfer] = useState<'sftp' | 'ftp'>('sftp')
  const [strict, setStrict] = useState(settings.strictHostKeyChecking || false)
  const [command, setCommand] = useState('')
  const [testMsg, setTestMsg] = useState('')
  const [saveMsg, setSaveMsg] = useState('')
  const [busy, setBusy] = useState(false)
  // ssh_config_host references a Host alias in the server's ~/.ssh/config.
  const [sshConfigHost, setSshConfigHost] = useState('')
  const [sshHosts, setSshHosts] = useState<string[]>([])

  // Vault integration: list of saved credentials + the one selected for reuse.
  const [credList, setCredList] = useState<CredentialMeta[]>([])
  const [selectedCredId, setSelectedCredId] = useState<number | null>(null)
  // B1: target group when saving; null = 未分组.
  const [groupId, setGroupId] = useState<number | null>(null)

  useEffect(() => {
    rest
      .sshConfigHosts()
      .then((r) => {
        if (r.code === 0 && r.data?.hosts) setSshHosts(r.data.hosts)
      })
      .catch(() => {
        /* no aliases or auth not ready; manual entry still works */
      })
    rest
      .listCredentials()
      .then((r) => {
        if (r.code === 0) setCredList(r.data || [])
      })
      .catch(() => {
        /* auth not ready / vault unavailable; dropdown stays empty */
      })
  }, [])

  // B1: ensure the group dropdown has data; the sidebar keeps its own local
  // copy, so the store may not be populated when the form opens standalone.
  useEffect(() => {
    if (useConnectionStore.getState().groups.length === 0) {
      void useConnectionStore.getState().load()
    }
  }, [])

  // B2: pre-fill the form from an existing saved connection when editing.
  useEffect(() => {
    if (!initial || !initial.id) return
    const proto = (initial.protocol as ProtocolType) || 'ssh'
    setProtocol(proto)
    setHost(initial.host || '')
    setPort(initial.port || DEFAULT_PORT[proto])
    setUsername(initial.username || '')
    setAuthType(initial.auth_type || 'password')
    setGroupId(initial.group_id ?? null)
    setSshConfigHost(initial.ssh_config_host || '')
    setSelectedCredId(initial.credential_id ?? null)
    // Pull the plaintext secret from the vault so credential fields are
    // editable; a reused reference keeps the same id.
    if (initial.credential_id != null) {
      rest
        .getCredentialSecret(initial.credential_id)
        .then((r) => {
          if (r.code === 0 && r.data) {
            const sec = r.data
            setPassword(sec.password || '')
            setPrivateKey(sec.private_key || '')
            setPassphrase(sec.passphrase || '')
            if (sec.private_key) setAuthType('publickey')
          }
        })
        .catch(() => {
          /* secret unavailable; fields stay empty */
        })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial?.id])

  const compose = (): ConnectionSpec => {
    const cred: ConnectionSpec['credential'] = {
      type: authType,
      username,
      password,
      private_key: privateKey || undefined,
      passphrase: passphrase || undefined,
    }
    const p = Number(port) || DEFAULT_PORT[protocol]
    const conn: ConnectionSpec = {
      id: 's' + Date.now(),
      protocol,
      host,
      port: p,
      credential: cred,
      initial_cols: 80,
      initial_rows: 24,
      strict_host_key_checking: strict,
      command: command || undefined,
      transfer: protocol === 'ssh' ? transfer : 'sftp',
      ssh_config_host: sshConfigHost || undefined,
    }
    return conn
  }

  const onProtocolChange = (p: ProtocolType) => {
    setProtocol(p)
    setPort(DEFAULT_PORT[p])
    setSshConfigHost('')
  }

  // Selecting/typing an alias no longer wipes manually entered host/port/
  // username — "manual input wins". The backend's ResolveSSHConfig only
  // fills a field from the alias when that field is left empty, so keeping the
  // user's values here gives the intended "manual overrides alias" behavior.
  const onSshConfigHostChange = (v: string) => {
    setSshConfigHost(v)
  }

  // Reuse a vault credential: decrypt it to prefill the form fields (for
  // display) and remember its id so saving references it instead of creating a
  // duplicate. The connection itself still carries the inline credential when
  // used immediately.
  const onSelectCred = async (idStr: string) => {
    if (idStr === '') {
      setSelectedCredId(null)
      return
    }
    const id = Number(idStr)
    setSelectedCredId(id)
    const r = await rest.getCredentialSecret(id)
    if (r.code === 0 && r.data) {
      const sec = r.data
      setUsername(sec.username || '')
      if (sec.private_key) {
        setAuthType('publickey')
        setPrivateKey(sec.private_key)
        setPassphrase(sec.passphrase || '')
        setPassword('')
      } else {
        setAuthType('password')
        setPassword(sec.password || '')
        setPrivateKey('')
        setPassphrase(sec.passphrase || '')
      }
    }
  }

  const submit = () => {
    onConnect(compose())
    onClose()
  }

  const test = async () => {
    setBusy(true)
    setTestMsg('测试中...')
    try {
      const r = await rest.testTerminal(compose())
      setTestMsg(r.code === 0 ? '✅ 连接成功' : '❌ ' + r.message)
    } catch (e) {
      setTestMsg('❌ ' + String(e))
    } finally {
      setBusy(false)
    }
  }

  // Persist the current form as a saved connection. Secrets never land on the
  // connections row: either an existing vault credential is referenced by id,
  // or a new one is created from the inline fields (B3).
  const save = async () => {
    setSaveMsg('保存中...')
    const p = Number(port) || DEFAULT_PORT[protocol]
    const alias = sshConfigHost.trim()
    const baseName = alias
      ? alias
      : host
        ? `${protocol}://${username || ''}@${host}:${p}`
        : `${protocol} 连接`
    // Name resolution:
    // - New connection: auto-generate (alias wins, then host-based fallback).
    // - Edit: when an alias is present, use it as the name so editing the alias
    //   updates the displayed connection name. When the alias is empty but the
    //   saved connection previously had one, regenerate from host. Otherwise
    //   keep the existing saved name, so unrelated edits (e.g. port) don't
    //   clobber a user-set custom name.
    let name: string
    if (initial?.id) {
      if (alias) {
        name = alias
      } else if (initial.ssh_config_host) {
        name = baseName
      } else {
        name = initial.name || baseName
      }
    } else {
      name = baseName
    }
    const input: SaveConnectionInput = {
      name,
      protocol,
      host,
      port: p,
      username,
      authType,
      credentialId: selectedCredId,
      password,
      privateKey,
      passphrase,
      groupId,
      // Transmit the (possibly empty) ssh_config_host so the backend
      // read-modify-write reflects the user's edit; the field is pre-filled
      // from the saved connection in edit mode, so the original value is
      // preserved unless the user deliberately changes it.
      sshConfigHost: sshConfigHost,
    }
    let id: number | null
    if (initial?.id) {
      id = await useConnectionStore.getState().updateConnection({ id: initial.id, ...input })
    } else {
      id = await useConnectionStore.getState().saveConnection(input)
    }
    if (id != null) {
      setSaveMsg('✅ 已保存')
      onClose()
    } else {
      setSaveMsg('❌ 保存失败')
    }
  }

  const field = 'w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm'
  const label = 'block text-xs text-gray-400 mb-1'
  const btn = 'px-3 py-1 rounded text-sm'

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/60">
      <div className="w-[520px] max-h-[90vh] overflow-auto bg-gray-900 border border-gray-700 rounded-lg p-5 shadow-2xl">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-100">{isEdit ? '编辑连接' : '新建连接'}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-200">✕</button>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={label}>协议</label>
            <select className={field} value={protocol} onChange={(e) => onProtocolChange(e.target.value as ProtocolType)}>
              <option value="ssh">SSH</option>
              <option value="telnet">Telnet</option>
              <option value="vnc">VNC</option>
              <option value="localshell">本地终端</option>
            </select>
          </div>
          <div>
            <label className={label}>端口</label>
            <input className={field} type="number" value={port} onChange={(e) => setPort(Number(e.target.value))} />
          </div>
          {protocol === 'ssh' && (
            <div className="col-span-2">
              <label className={label}>SSH 配置别名（可选，手输或选择服务器 ~/.ssh/config 的 Host）</label>
              <input
                className={field}
                list="ssh-hosts-list"
                value={sshConfigHost}
                onChange={(e) => onSshConfigHostChange(e.target.value)}
                placeholder="如 my-server"
              />
              <datalist id="ssh-hosts-list">
                {sshHosts.map((h) => (
                  <option key={h} value={h} />
                ))}
              </datalist>
            </div>
          )}
          <div className="col-span-2">
            <label className={label}>主机 / IP{protocol === 'localshell' ? '（本地终端可留空）' : ''}</label>
            <input className={field} placeholder="例如 192.168.1.10" value={host} onChange={(e) => setHost(e.target.value)} disabled={protocol === 'localshell'} />
          </div>
          {/* B1: choose a group when saving (null = 未分组). */}
          <div className="col-span-2">
            <label className={label}>分组（可选）</label>
            <select
              className={field}
              value={groupId ?? ''}
              onChange={(e) => setGroupId(e.target.value === '' ? null : Number(e.target.value))}
            >
              <option value="">未分组</option>
              {groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={label}>用户名</label>
            <input className={field} value={username} onChange={(e) => setUsername(e.target.value)} />
          </div>
          <div>
            <label className={label}>认证方式</label>
            <select className={field} value={authType} onChange={(e) => setAuthType(e.target.value)} disabled={protocol !== 'ssh'}>
              <option value="password">密码</option>
              <option value="publickey">公钥</option>
              <option value="keyboard-interactive">键盘交互 / 2FA</option>
              <option value="agent">SSH Agent</option>
            </select>
          </div>
          {protocol !== 'localshell' && (
            <div className="col-span-2">
              <label className={label}>从凭证库选择（可选，自动填充并引用）</label>
              <select className={field} value={selectedCredId ?? ''} onChange={(e) => void onSelectCred(e.target.value)}>
                <option value="">不使用 / 手动填写</option>
                {credList.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}（{c.type === 'private_key' ? '私钥' : '密码'}）
                  </option>
                ))}
              </select>
            </div>
          )}
          {authType === 'password' && (
            <div className="col-span-2">
              <label className={label}>密码</label>
              <input className={field} type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
          )}
          {authType === 'publickey' && (
            <>
              <div className="col-span-2">
                <label className={label}>私钥 (PEM)</label>
                <textarea className={field + ' h-20'} value={privateKey} onChange={(e) => setPrivateKey(e.target.value)} />
              </div>
              <div className="col-span-2">
                <label className={label}>私钥口令（可选）</label>
                <input className={field} type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
              </div>
            </>
          )}
          {protocol === 'ssh' && (
            <div className="col-span-2">
              <label className={label}>文件传输协议</label>
              <select className={field} value={transfer} onChange={(e) => setTransfer(e.target.value as 'sftp' | 'ftp')}>
                <option value="sftp">SFTP</option>
                <option value="ftp">FTP</option>
              </select>
            </div>
          )}
          <div className="col-span-2">
            <label className={label}>启动命令（可选，例如 sudo -i）</label>
            <input className={field} value={command} onChange={(e) => setCommand(e.target.value)} />
          </div>
          {protocol === 'ssh' && (
            <div className="col-span-2 flex items-center gap-2">
              <input type="checkbox" checked={strict} onChange={(e) => setStrict(e.target.checked)} />
              <span className="text-sm text-gray-300">严格校验 known_hosts</span>
            </div>
          )}
        </div>

        <div className="flex items-center gap-2 mt-4">
          <button className={btn + ' bg-accent text-black'} onClick={submit}>连接</button>
          <button className={btn + ' bg-gray-700 text-gray-200'} onClick={test} disabled={busy}>
            {busy ? '测试中…' : '测试连接'}
          </button>
          <button className={btn + ' bg-gray-700 text-gray-200'} onClick={() => void save()}>
            保存
          </button>
          {testMsg && <span className="text-xs text-gray-300">{testMsg}</span>}
          {saveMsg && <span className="text-xs text-gray-300">{saveMsg}</span>}
        </div>
      </div>
    </div>
  )
}
