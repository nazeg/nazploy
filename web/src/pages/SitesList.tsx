import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Site } from '../types'

export default function SitesList() {
  const [sites, setSites] = useState<Site[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    loadSites()
  }, [])

  async function loadSites() {
    setLoading(true)
    try {
      const records = await pb.collection('sites').getFullList<Site>({
        sort: '-created',
      })
      setSites(records)
    } catch (err) {
      setError('Siteler yüklenemedi')
    } finally {
      setLoading(false)
    }
  }

  async function deleteSite(site: Site) {
    if (!confirm(`"${site.name}" sitesini silmek istediğinize emin misiniz?`)) return

    try {
      await pb.send(`/api/dashboard/sites/${site.id}`, { method: 'DELETE' })
      setSites((prev) => prev.filter((s) => s.id !== site.id))
    } catch {
      alert('Silme işlemi başarısız')
    }
  }

  const statusColors = {
    active: 'bg-green-100 text-green-700',
    paused: 'bg-yellow-100 text-yellow-700',
  }

  const sslColors = {
    none: 'bg-gray-100 text-gray-600',
    pending: 'bg-yellow-100 text-yellow-700',
    active: 'bg-green-100 text-green-700',
    error: 'bg-red-100 text-red-700',
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Siteler</h1>
          <p className="text-sm text-gray-500 mt-1">Tüm web sitelerini yönet</p>
        </div>
        <Link
          to="/sites/new"
          className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
        >
          + Yeni Site
        </Link>
      </div>

      {error && (
        <div className="bg-red-50 text-red-600 text-sm p-3 rounded-lg mb-4">{error}</div>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
        </div>
      ) : sites.length === 0 ? (
        <div className="bg-white rounded-xl shadow-sm p-12 text-center">
          <p className="text-gray-500 mb-4">Henüz bir site eklenmemiş.</p>
          <Link
            to="/sites/new"
            className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
          >
            İlk Siteyi Ekle
          </Link>
        </div>
      ) : (
        <div className="bg-white rounded-xl shadow-sm overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
                <th className="text-left p-4 text-sm font-medium text-gray-500">İsim</th>
                <th className="text-left p-4 text-sm font-medium text-gray-500">Domain</th>
                <th className="text-left p-4 text-sm font-medium text-gray-500">Port</th>
                <th className="text-left p-4 text-sm font-medium text-gray-500">Tür</th>
                <th className="text-left p-4 text-sm font-medium text-gray-500">SSL</th>
                <th className="text-left p-4 text-sm font-medium text-gray-500">Durum</th>
                <th className="text-right p-4 text-sm font-medium text-gray-500">İşlem</th>
              </tr>
            </thead>
            <tbody>
              {sites.map((site) => (
                <tr key={site.id} className="border-b border-gray-50 hover:bg-gray-50">
                  <td className="p-4">
                    <Link to={`/sites/${site.id}`} className="font-medium text-blue-600 hover:text-blue-800">
                      {site.name}
                    </Link>
                  </td>
                  <td className="p-4 text-sm text-gray-600">{site.domain}</td>
                  <td className="p-4 text-sm text-gray-600">{site.port}</td>
                  <td className="p-4 text-sm">
                    <span className="capitalize">{site.site_type}</span>
                  </td>
                  <td className="p-4">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${sslColors[site.ssl_status]}`}>
                      {site.ssl_status === 'active' ? 'Aktif' : site.ssl_status === 'pending' ? 'Bekliyor' : site.ssl_status === 'error' ? 'Hata' : 'Yok'}
                    </span>
                  </td>
                  <td className="p-4">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${statusColors[site.status]}`}>
                      {site.status === 'active' ? 'Aktif' : 'Duraklatıldı'}
                    </span>
                  </td>
                  <td className="p-4 text-right">
                    <button
                      onClick={() => deleteSite(site)}
                      className="text-red-500 hover:text-red-700 text-sm"
                    >
                      Sil
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
