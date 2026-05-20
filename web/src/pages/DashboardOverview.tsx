import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Stats } from '../types'

export default function DashboardOverview() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadStats()
  }, [])

  async function loadStats() {
    try {
      const res = await pb.send('/api/dashboard/stats', { method: 'GET' })
      setStats(res as Stats)
    } catch {
      // fallback
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    )
  }

  const cards = [
    {
      label: 'Toplam Site',
      value: stats?.total_sites ?? 0,
      color: 'bg-blue-500',
      link: '/sites',
    },
    {
      label: 'Aktif Siteler',
      value: stats?.active_sites ?? 0,
      color: 'bg-green-500',
      link: '/sites',
    },
    {
      label: 'SSL Aktif',
      value: stats?.ssl_active_count ?? 0,
      color: 'bg-purple-500',
      link: '/sites',
    },
    {
      label: 'Veritabanları',
      value: stats?.total_databases ?? 0,
      color: 'bg-orange-500',
      link: '/sites',
    },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Dashboard</h1>
          <p className="text-sm text-gray-500 mt-1">VPS sitelerine genel bakış</p>
        </div>
        <Link
          to="/sites/new"
          className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
        >
          + Yeni Site
        </Link>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {cards.map((card) => (
          <Link key={card.label} to={card.link} className="bg-white rounded-xl shadow-sm p-5 hover:shadow-md transition-shadow">
            <div className="flex items-center gap-3">
              <div className={`w-10 h-10 ${card.color} rounded-lg flex items-center justify-center text-white font-bold text-lg`}>
                {card.value}
              </div>
              <div>
                <p className="text-sm text-gray-500">{card.label}</p>
                <p className="text-2xl font-bold">{card.value}</p>
              </div>
            </div>
          </Link>
        ))}
      </div>

      <div className="bg-white rounded-xl shadow-sm p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-semibold">Nginx Durumu</h2>
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
            stats?.nginx_running ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
          }`}>
            {stats?.nginx_running ? 'Çalışıyor' : 'Kapalı'}
          </span>
        </div>
        <p className="text-sm text-gray-500">
          {stats?.nginx_running
            ? 'Nginx aktif ve sitelere hizmet veriyor.'
            : 'Nginx çalışmıyor. Lütfen Nginx durum sayfasını kontrol edin.'}
        </p>
      </div>
    </div>
  )
}
