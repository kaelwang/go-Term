interface Props {
  active: string
  onSelect: (v: string) => void
}

const ITEMS: [string, string, string][] = [
  ['connect', '连接', '🔌'],
  ['files', '文件', '📂'],
  ['quick', '快捷命令', '⚡'],
  ['vault', '凭证', '🔐'],
  ['settings', '设置', '⚙️'],
  ['logs', '日志', '📜'],
  ['users', '用户管理', '👤'],
]

// Sidebar: primary navigation rail toggling the right-hand panel.
export default function Sidebar({ active, onSelect }: Props) {
  return (
    <div className="w-16 bg-gray-950 border-r border-gray-800 flex flex-col items-center py-3 gap-2">
      {ITEMS.map(([key, label, icon]) => (
        <button
          key={key}
          title={label}
          onClick={() => onSelect(key)}
          className={
            'w-12 h-12 rounded flex flex-col items-center justify-center text-[10px] gap-0.5 ' +
            (active === key
              ? 'bg-accent text-black'
              : 'text-gray-400 hover:bg-gray-800')
          }
        >
          <span className="text-base leading-none">{icon}</span>
          <span>{label}</span>
        </button>
      ))}
    </div>
  )
}
