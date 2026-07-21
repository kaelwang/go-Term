import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'

interface Props {
  open: boolean
  title: string
  /** Confirmation body text shown above the buttons (omit for input-only dialogs). */
  message?: string
  /** When set, an input field is rendered and its value is passed to onConfirm. */
  inputLabel?: string
  inputDefault?: string
  /** Input type for the rendered field (e.g. 'password'). Defaults to 'text'. */
  inputType?: string
  inputPlaceholder?: string
  confirmText?: string
  cancelText?: string
  onConfirm: (value: string) => void
  onCancel: () => void
  /** Custom body content (e.g. multi-field forms). When provided it replaces
   *  the single input field; onConfirm still fires on the confirm button. */
  children?: ReactNode
}

// ModalDialog is a theme-aware replacement for the native window.prompt /
// window.confirm popups. It mirrors the look of ConnectForm: dark surface
// (bg-gray-900), subtle border, and the accent confirm button.
export default function ModalDialog({
  open,
  title,
  message,
  inputLabel,
  inputDefault,
  inputType = 'text',
  inputPlaceholder,
  confirmText = '确定',
  cancelText = '取消',
  onConfirm,
  onCancel,
  children,
}: Props) {
  const [value, setValue] = useState(inputDefault || '')

  useEffect(() => {
    if (open) setValue(inputDefault || '')
  }, [open, inputDefault])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={onCancel}
    >
      <div
        className="w-[360px] bg-gray-900 border border-gray-700 rounded-lg p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-base font-semibold text-gray-100 mb-3">{title}</h3>
        {message && <p className="text-sm text-gray-300 mb-3 whitespace-pre-wrap">{message}</p>}
        {children
          ? children
          : inputLabel && (
              <div className="mb-4">
                <label className="block text-xs text-gray-400 mb-1">{inputLabel}</label>
                <input
                  autoFocus
                  type={inputType}
                  placeholder={inputPlaceholder}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm text-gray-100 focus:outline-none focus:border-accent"
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') onConfirm(value)
                  }}
                />
              </div>
            )}
        <div className="flex justify-end gap-2">
          <button
            className="px-3 py-1 rounded text-sm bg-gray-700 text-gray-200 hover:bg-gray-600"
            onClick={onCancel}
          >
            {cancelText}
          </button>
          <button
            className="px-3 py-1 rounded text-sm bg-accent text-black hover:opacity-90"
            onClick={() => onConfirm(value)}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  )
}
