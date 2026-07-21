import React, { PointerEvent as ReactPointerEvent, useRef, useState } from 'react'

interface Props {
  orientation: 'horizontal' | 'vertical'
  children: [React.ReactNode, React.ReactNode]
}

// SplitPane renders two children with a draggable divider. Horizontal =
// side-by-side; vertical = stacked. The divider position is a 0..1 ratio.
export default function SplitPane({ orientation, children }: Props) {
  const [ratio, setRatio] = useState(0.5)
  const ref = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const isH = orientation === 'horizontal'

  const onPointerMove = (e: PointerEvent) => {
    if (!dragging.current || !ref.current) return
    const rect = ref.current.getBoundingClientRect()
    const r = isH
      ? (e.clientX - rect.left) / rect.width
      : (e.clientY - rect.top) / rect.height
    setRatio(Math.min(0.85, Math.max(0.15, r)))
  }
  const stop = () => {
    dragging.current = false
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerup', stop)
  }
  const onDown = (e: ReactPointerEvent) => {
    e.preventDefault()
    dragging.current = true
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', stop)
  }

  const firstStyle: React.CSSProperties = isH
    ? { flexBasis: `${ratio * 100}%` }
    : { flexBasis: `${ratio * 100}%` }
  const dividerStyle: React.CSSProperties = isH
    ? { width: 4, cursor: 'col-resize' }
    : { height: 4, cursor: 'row-resize' }

  return (
    <div
      ref={ref}
      className="flex w-full h-full"
      style={{ flexDirection: isH ? 'row' : 'column' }}
    >
      <div
        className="h-full overflow-hidden"
        style={{ ...firstStyle, flexGrow: 0, flexShrink: 0 }}
      >
        {children[0]}
      </div>
      <div
        onPointerDown={onDown}
        className="bg-gray-600 hover:bg-accent"
        style={dividerStyle}
      />
      <div className="flex-1 h-full overflow-hidden">{children[1]}</div>
    </div>
  )
}
