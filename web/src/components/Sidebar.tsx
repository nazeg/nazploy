import { NavLink } from 'react-router-dom'
import pb from '../lib/pocketbase'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '◻' },
  { to: '/sites', label: 'Siteler', icon: '◻' },
  { to: '/nginx', label: 'Nginx Durum', icon: '◻' },
]

export default function Sidebar() {
  const user = pb.authStore.model

  return (
    <aside className="w-64 bg-slate-900 text-white flex flex-col">
      <div className="p-5 border-b border-slate-700">
        <h1 className="text-xl font-bold tracking-tight">VPS Dashboard</h1>
        <p className="text-xs text-slate-400 mt-1">{user?.email}</p>
      </div>

      <nav className="flex-1 p-3 space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-300 hover:bg-slate-800 hover:text-white'
              }`
            }
          >
            <span className="text-lg">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
      </nav>

      <div className="p-3 border-t border-slate-700">
        <button
          onClick={() => pb.authStore.clear()}
          className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium text-slate-400 hover:bg-slate-800 hover:text-white w-full transition-colors"
        >
          Çıkış Yap
        </button>
      </div>
    </aside>
  )
}
