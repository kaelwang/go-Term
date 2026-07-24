import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'

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
  const confirmRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (open) {
      setValue(inputDefault || '')
      // When there is no text input to focus, make the confirm button the
      // default action so the Enter key confirms directly and it shows focus.
      if (!inputLabel && !children) confirmRef.current?.focus()
    }
  }, [open, inputDefault, inputLabel, children])

  // Keyboard handling for the dialog surface. The confirm button is auto-focused
  // above, so a focused BUTTON handles Enter natively; we only act here when the
  // focus is elsewhere (e.g. the card/backdrop) and skip inputs/buttons to avoid
  // double-triggering.
  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const tag = (e.target as HTMLElement).tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'BUTTON') return
    if (e.key === 'Enter') {
      e.preventDefault()
      onConfirm(value)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={onCancel}
      onKeyDown={handleKeyDown}
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
            ref={confirmRef}
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
