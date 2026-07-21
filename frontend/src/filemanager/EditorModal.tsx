import { useEffect, useState } from 'react'
import type { ConnectionSpec } from '../types'
import { downloadUrl, uploadFile } from './fileApi'

interface Props {
  connection: ConnectionSpec
  path: string
  onClose: () => void
}

// EditorModal fetches a remote text file, allows inline editing, and saves
// it back (overwriting) via the upload endpoint.
export default function EditorModal({ connection, path, onClose }: Props) {
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const t = connection.transfer || 'sftp'
  const name = path.split('/').pop() || 'file'

  useEffect(() => {
    setLoading(true)
    const url = downloadUrl(connection, path, t)
    fetch(url)
      .then((r) => {
        if (!r.ok) throw new Error('下载失败 ' + r.status)
        return r.text()
      })
      .then((txt) => setText(txt))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path])

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      const blob = new Blob([text], { type: 'text/plain' })
      const file = new File([blob], name)
      const dir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '/'
      await uploadFile(connection, dir, file, t)
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-[760px] max-h-[90vh] flex flex-col bg-gray-900 border border-gray-700 rounded-lg shadow-2xl">
        <div className="flex items-center justify-between px-4 py-2 border-b border-gray-700">
          <span className="text-sm text-gray-200">编辑：{path}</span>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-200">✕</button>
        </div>
        {loading ? (
          <div className="p-6 text-sm text-gray-400">加载中…</div>
        ) : (
          <textarea
            className="flex-1 w-full h-[60vh] bg-gray-950 text-gray-100 p-3 font-mono text-sm outline-none resize-none"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        )}
        {error && <div className="px-4 py-2 text-xs text-red-400 bg-red-900/40">{error}</div>}
        <div className="flex items-center justify-end gap-2 px-4 py-2 border-t border-gray-700">
          <button onClick={onClose} className="px-3 py-1 text-sm bg-gray-700 rounded">取消</button>
          <button
            onClick={save}
            disabled={saving}
            className="px-3 py-1 text-sm bg-accent text-black rounded disabled:opacity-50"
          >
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  )
}
