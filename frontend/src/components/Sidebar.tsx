import { Plug, Folder, Zap, KeyRound, Settings, ScrollText, User } from 'lucide-react'

interface Props {
  active: string
  onSelect: (v: string) => void
}

const ITEMS: { key: string; label: string; Icon: typeof Plug }[] = [
  { key: 'connect', label: '连接', Icon: Plug },
  { key: 'files', label: '文件', Icon: Folder },
  { key: 'quick', label: '快捷命令', Icon: Zap },
  { key: 'vault', label: '凭证', Icon: KeyRound },
  { key: 'settings', label: '设置', Icon: Settings },
  { key: 'logs', label: '日志', Icon: ScrollText },
  { key: 'users', label: '用户管理', Icon: User },
]

// Sidebar: primary navigation rail toggling the right-hand panel.
export default function Sidebar({ active, onSelect }: Props) {
  return (
    <div className="w-16 bg-gray-950 border-r border-gray-800 flex flex-col items-center py-3 gap-2">
      {ITEMS.map(({ key, label, Icon }) => (
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
          <Icon size={18} strokeWidth={1.75} />
          <span>{label}</span>
        </button>
      ))}
    </div>
  )
}
