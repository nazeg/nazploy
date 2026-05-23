import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Stats } from '../types'

export default function DashboardOverview() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadStats()
    const interval = setInterval(loadStats, 5000)
    return () => clearInterval(interval)
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

      {/* Sistem Durumu / Kaynak Kullanımı */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
        {/* CPU */}
        <div className="bg-white rounded-xl shadow-sm p-5">
          <div className="flex justify-between items-center mb-3">
            <span className="text-sm font-semibold text-gray-700">İşlemci (CPU)</span>
            <span className={`text-sm font-bold ${
              (stats?.metrics?.cpu_percent ?? 0) > 90 ? 'text-red-600' : (stats?.metrics?.cpu_percent ?? 0) > 70 ? 'text-yellow-600' : 'text-green-600'
            }`}>
              {stats?.metrics?.cpu_percent?.toFixed(1) ?? '0.0'}%
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className={`h-2 rounded-full transition-all duration-500 ${
                (stats?.metrics?.cpu_percent ?? 0) > 90 ? 'bg-red-500' : (stats?.metrics?.cpu_percent ?? 0) > 70 ? 'bg-yellow-500' : 'bg-green-500'
              }`}
              style={{ width: `${Math.min(100, stats?.metrics?.cpu_percent ?? 0)}%` }}
            />
          </div>
        </div>

        {/* Memory */}
        <div className="bg-white rounded-xl shadow-sm p-5">
          <div className="flex justify-between items-center mb-3">
            <span className="text-sm font-semibold text-gray-700">Bellek (RAM)</span>
            <span className={`text-sm font-bold ${
              (stats?.metrics?.ram_percent ?? 0) > 90 ? 'text-red-600' : (stats?.metrics?.ram_percent ?? 0) > 70 ? 'text-yellow-600' : 'text-green-600'
            }`}>
              {stats?.metrics?.ram_percent?.toFixed(1) ?? '0.0'}%
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2 mb-2">
            <div
              className={`h-2 rounded-full transition-all duration-500 ${
                (stats?.metrics?.ram_percent ?? 0) > 90 ? 'bg-red-500' : (stats?.metrics?.ram_percent ?? 0) > 70 ? 'bg-yellow-500' : 'bg-green-500'
              }`}
              style={{ width: `${Math.min(100, stats?.metrics?.ram_percent ?? 0)}%` }}
            />
          </div>
          <span className="text-xs text-gray-500">
            {stats?.metrics?.ram_used_mb ? (stats.metrics.ram_used_mb / 1024).toFixed(2) : '0.00'} GB / {stats?.metrics?.ram_total_mb ? (stats.metrics.ram_total_mb / 1024).toFixed(2) : '0.00'} GB Kullanılıyor
          </span>
        </div>

        {/* Disk */}
        <div className="bg-white rounded-xl shadow-sm p-5">
          <div className="flex justify-between items-center mb-3">
            <span className="text-sm font-semibold text-gray-700">Disk Depolama</span>
            <span className={`text-sm font-bold ${
              (stats?.metrics?.disk_percent ?? 0) > 90 ? 'text-red-600' : (stats?.metrics?.disk_percent ?? 0) > 70 ? 'text-yellow-600' : 'text-green-600'
            }`}>
              {stats?.metrics?.disk_percent?.toFixed(1) ?? '0.0'}%
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2 mb-2">
            <div
              className={`h-2 rounded-full transition-all duration-500 ${
                (stats?.metrics?.disk_percent ?? 0) > 90 ? 'bg-red-500' : (stats?.metrics?.disk_percent ?? 0) > 70 ? 'bg-yellow-500' : 'bg-green-500'
              }`}
              style={{ width: `${Math.min(100, stats?.metrics?.disk_percent ?? 0)}%` }}
            />
          </div>
          <span className="text-xs text-gray-500">
            {stats?.metrics?.disk_used_gb?.toFixed(2) ?? '0.00'} GB / {stats?.metrics?.disk_total_gb?.toFixed(2) ?? '0.00'} GB Kullanılıyor
          </span>
        </div>
      </div>

      <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h2 className="font-bold text-gray-900">Nginx Durumu</h2>
            <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-bold border ${
              stats?.nginx_running
                ? 'bg-emerald-50 text-emerald-700 border-emerald-100'
                : 'bg-rose-50 text-rose-700 border-rose-100'
            }`}>
              {stats?.nginx_running ? 'Çalışıyor' : 'Kapalı'}
            </span>
          </div>
          <p className="text-sm text-gray-500 leading-relaxed">
            {stats?.nginx_running
              ? 'Nginx aktif ve sitelere hizmet veriyor. Tüm proxy yönlendirmeleri aktif.'
              : 'Nginx çalışmıyor. Sitelerinize erişim sağlanamayabilir.'}
          </p>
        </div>
        <Link to="/nginx" className="bg-gray-50 hover:bg-gray-100 border border-gray-200 text-gray-700 px-4 py-2.5 rounded-xl text-sm font-semibold transition-colors flex items-center gap-2 shrink-0 justify-center">
          Nginx Yönetimi &rarr;
        </Link>
      </div>
    </div>
  )
}
