import { NavLink } from 'react-router-dom'
import pb from '../lib/pocketbase'
import { LayoutDashboard, Globe, Server, LogOut, Terminal, Settings } from 'lucide-react'

const navItems = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/sites', label: 'Siteler', icon: Globe },
  { to: '/nginx', label: 'Nginx Durum', icon: Server },
  { to: '/settings', label: 'Ayarlar', icon: Settings },
]

export default function Sidebar() {
  const user = pb.authStore.model
  const initial = user?.email ? user.email.charAt(0).toUpperCase() : 'N'

  return (
    <aside className="w-64 bg-zinc-950 text-zinc-100 flex flex-col border-r border-zinc-900">
      {/* Brand Header */}
      <div className="p-6 border-b border-zinc-900/60 flex items-center gap-3">
        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-tr from-indigo-600 to-violet-600 shadow-md shadow-indigo-500/20">
          <Terminal className="h-[18px] w-[18px] text-white" />
        </div>
        <div className="flex flex-col">
          <span className="text-sm font-bold tracking-tight bg-gradient-to-r from-white via-zinc-100 to-zinc-400 bg-clip-text text-transparent">
            Nazploy
          </span>
          <span className="text-[10px] text-zinc-500 uppercase tracking-wider font-semibold">
            VPS Deployer
          </span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-6 space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon
          return (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `group flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 border-l-2 ${
                  isActive
                    ? 'bg-indigo-500/10 text-indigo-400 border-indigo-500'
                    : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-900/50 border-transparent hover:translate-x-0.5'
                }`
              }
            >
              {({ isActive }) => (
                <>
                  <Icon
                    className={`h-[18px] w-[18px] transition-transform duration-200 group-hover:scale-105 ${
                      isActive ? 'text-indigo-400' : 'text-zinc-400 group-hover:text-zinc-300'
                    }`}
                  />
                  <span>{item.label}</span>
                </>
              )}
            </NavLink>
          )
        })}
      </nav>

      {/* Footer Profile / Logout */}
      <div className="p-4 border-t border-zinc-900/60 bg-zinc-950/40 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-zinc-900 text-zinc-200 font-semibold text-xs border border-zinc-800/80">
            {initial}
          </div>
          <div className="flex flex-col min-w-0">
            <span className="text-xs font-medium text-zinc-300 truncate">
              {user?.email?.split('@')[0]}
            </span>
            <span className="text-[10px] text-zinc-500 truncate">
              {user?.email}
            </span>
          </div>
        </div>
        <button
          onClick={() => pb.authStore.clear()}
          title="Çıkış Yap"
          className="p-2 rounded-lg text-zinc-500 hover:text-rose-400 hover:bg-rose-500/10 transition-all duration-200 cursor-pointer"
        >
          <LogOut className="h-[18px] w-[18px]" />
        </button>
      </div>
    </aside>
  )
}
