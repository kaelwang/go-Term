import { useEffect, useState } from 'react'
import { Folder, File as FileIcon } from 'lucide-react'
import type { ConnectionSpec, FileEntry } from '../types'
import { ErrorCode } from '../types'
import {
  listDir,
  makeDir,
  removePath,
  renamePath,
  downloadUrl,
  uploadFile,
  FileApiError,
} from './fileApi'

interface Props {
  connection: ConnectionSpec
  onOpenFile: (path: string) => void
}

/** Translate a raw error into a user-friendly Chinese message. */
function translateError(err: unknown): string {
  if (err instanceof FileApiError) {
    switch (err.code) {
      case ErrorCode.AuthFail:
        return '认证失败：请重新登录后再试'
      case ErrorCode.PermissionDenied:
        return '权限不足：当前用户无此操作权限'
      case ErrorCode.ConnFail:
        return `连接失败：${err.message || '无法连接到远程服务器'}`
      case ErrorCode.TransferFail:
        return `文件操作失败：${err.message || '请检查远程服务是否支持 SFTP'}`
      case ErrorCode.BadParam:
        return `参数错误：${err.message || '请求参数不正确'}`
      default:
        return err.message || '未知错误'
    }
  }
  if (err instanceof Error) return err.message
  return String(err)
}

function formatSize(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1024 / 1024).toFixed(1) + ' MB'
}

// FilePanel browses a remote directory tree with the usual file-manager
// operations (list / upload / download / mkdir / rename / remove).
export default function FilePanel({ connection, onOpenFile }: Props) {
  const [path, setPath] = useState('/')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const t = connection.transfer || 'sftp'

  const load = (p: string) => {
    setLoading(true)
    setError('')
    listDir(connection, p, t)
      .then((e) => {
        setEntries(
          [...e].sort((a, b) => {
            if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
            return a.name.localeCompare(b.name)
          }),
        )
        setPath(p)
      })
      .catch((err) => setError(translateError(err)))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load('/')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connection.id])

  const parts = path.split('/').filter(Boolean)
  const crumbs = ['/']
  parts.forEach((p, i) => crumbs.push('/' + parts.slice(0, i + 1).join('/')))

  const goUp = () => {
    const parent = path === '/' ? '/' : '/' + parts.slice(0, -1).join('/')
    load(parent === '' ? '/' : parent)
  }

  const onUpload = (file: File | null) => {
    if (!file) return
    uploadFile(connection, path, file, t)
      .then(() => load(path))
      .catch((e) => setError(translateError(e)))
  }

  const onDownload = (e: FileEntry) => {
    const a = document.createElement('a')
    a.href = downloadUrl(connection, e.path, t)
    a.download = e.name
    a.click()
  }

  const onMkdir = () => {
    const name = prompt('新建目录名')
    if (!name) return
    const np = path === '/' ? '/' + name : path + '/' + name
    makeDir(connection, np, t)
      .then(() => load(path))
      .catch((e) => setError(translateError(e)))
  }

  const onRename = (e: FileEntry) => {
    const name = prompt('重命名为', e.name)
    if (!name) return
    const np = path === '/' ? '/' + name : path + '/' + name
    renamePath(connection, e.path, np, t)
      .then(() => load(path))
      .catch((err) => setError(translateError(err)))
  }

  const onRemove = (e: FileEntry) => {
    if (!confirm(`确认删除 ${e.name} ?`)) return
    removePath(connection, e.path, t)
      .then(() => load(path))
      .catch((err) => setError(translateError(err)))
  }

  return (
    <div className="flex flex-col h-full bg-gray-900 text-gray-200">
      <div className="flex items-center gap-2 p-2 border-b border-gray-700">
        <button className="px-2 py-1 text-xs bg-gray-700 rounded" onClick={goUp}>
          ↑ 上级
        </button>
        <div className="flex-1 flex items-center gap-1 overflow-x-auto text-xs">
          {crumbs.map((c, i) => (
            <span key={i}>
              <button className="hover:text-accent" onClick={() => load(c)}>
                {i === 0 ? '根' : c.split('/').pop()}
              </button>
              {i < crumbs.length - 1 && <span className="text-gray-500"> / </span>}
            </span>
          ))}
        </div>
        <label className="px-2 py-1 text-xs bg-gray-700 rounded cursor-pointer">
          上传
          <input
            type="file"
            className="hidden"
            onChange={(e) => onUpload(e.target.files?.[0] || null)}
          />
        </label>
        <button className="px-2 py-1 text-xs bg-gray-700 rounded" onClick={onMkdir}>
          新建目录
        </button>
      </div>

      {error && <div className="px-2 py-1 text-xs text-red-400 bg-red-900/40">{error}</div>}
      {loading && <div className="px-2 py-1 text-xs text-gray-400">加载中…</div>}

      <div className="flex-1 overflow-auto thin-scroll">
        <table className="w-full text-sm">
          <thead className="text-gray-400 text-xs bg-gray-800">
            <tr>
              <th className="text-left px-2 py-1">名称</th>
              <th className="text-right px-2 py-1 w-24">大小</th>
              <th className="text-left px-2 py-1 w-32">操作</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e) => (
              <tr key={e.path} className="border-b border-gray-800 hover:bg-gray-800/60">
                <td className="px-2 py-1">
                  <span className={e.is_dir ? 'text-accent' : e.is_symlink ? 'text-yellow-300' : ''}>
                    {e.is_dir ? (
                      <Folder size={14} className="inline align-text-bottom" />
                    ) : (
                      <FileIcon size={14} className="inline align-text-bottom" />
                    )}{' '}
                    {e.name}
                  </span>
                </td>
                <td className="px-2 py-1 text-right text-gray-400">{e.is_dir ? '—' : formatSize(e.size)}</td>
                <td className="px-2 py-1 whitespace-nowrap">
                  {e.is_dir ? (
                    <button className="text-xs text-accent hover:underline" onClick={() => load(e.path)}>
                      进入
                    </button>
                  ) : (
                    <>
                      <button className="text-xs text-accent hover:underline mr-2" onClick={() => onOpenFile(e.path)}>
                        编辑
                      </button>
                      <button className="text-xs text-gray-300 hover:underline mr-2" onClick={() => onDownload(e)}>
                        下载
                      </button>
                    </>
                  )}
                  {!e.is_dir && (
                    <button className="text-xs text-gray-300 hover:underline mr-2" onClick={() => onRename(e)}>
                      重命名
                    </button>
                  )}
                  <button className="text-xs text-red-400 hover:underline" onClick={() => onRemove(e)}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
