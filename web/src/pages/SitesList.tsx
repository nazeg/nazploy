import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Site } from '../types'
import {
  LayoutGrid,
  List,
  Globe,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Play,
  Pause,
  Trash2,
  ExternalLink,
  ArrowRight,
  GitBranch,
  Terminal,
  Activity
} from 'lucide-react'

export default function SitesList() {
  const [sites, setSites] = useState<Site[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [viewMode, setViewMode] = useState<'grid' | 'list'>(() => {
    return (localStorage.getItem('sites_view_mode') as 'grid' | 'list') || 'grid'
  })

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
    active: 'bg-green-100 text-green-700 border-green-200',
    paused: 'bg-yellow-100 text-yellow-700 border-yellow-200',
  }

  async function toggleStatus(site: Site) {
    const newStatus = site.status === 'active' ? 'paused' : 'active'
    try {
      const updated = await pb.send<Site>(`/api/dashboard/sites/${site.id}`, {
        method: 'PATCH',
        body: { status: newStatus },
      })
      setSites((prev) => prev.map((s) => (s.id === site.id ? { ...s, status: updated.status } : s)))
    } catch (err: any) {
      alert('Hata: Durum değiştirilemedi. ' + (err?.message || ''))
    }
  }

  const sslColors = {
    none: 'bg-gray-100 text-gray-600 border-gray-200',
    pending: 'bg-yellow-100 text-yellow-700 border-yellow-200',
    active: 'bg-green-100 text-green-700 border-green-200',
    error: 'bg-red-100 text-red-700 border-red-200',
  }

  const toggleViewMode = (mode: 'grid' | 'list') => {
    setViewMode(mode)
    localStorage.setItem('sites_view_mode', mode)
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">Siteler</h1>
          <p className="text-sm text-gray-500 mt-1">Tüm web sitelerini yönetin ve izleyin</p>
        </div>
        <div className="flex items-center gap-3">
          {/* View Toggle Switcher */}
          <div className="flex items-center bg-gray-100 p-1 rounded-xl border border-gray-200">
            <button
              onClick={() => toggleViewMode('grid')}
              className={`p-2 rounded-lg transition-all duration-200 ${
                viewMode === 'grid'
                  ? 'bg-white text-blue-600 shadow-sm'
                  : 'text-gray-500 hover:text-gray-900'
              }`}
              title="Kutu Görünümü"
            >
              <LayoutGrid className="w-4 h-4" />
            </button>
            <button
              onClick={() => toggleViewMode('list')}
              className={`p-2 rounded-lg transition-all duration-200 ${
                viewMode === 'list'
                  ? 'bg-white text-blue-600 shadow-sm'
                  : 'text-gray-500 hover:text-gray-900'
              }`}
              title="Liste Görünümü"
            >
              <List className="w-4 h-4" />
            </button>
          </div>

          <Link
            to="/sites/new"
            className="bg-blue-600 text-white px-4 py-2 rounded-xl text-sm font-semibold hover:bg-blue-700 transition-colors shadow-sm shadow-blue-500/10 flex items-center gap-1.5"
          >
            + Yeni Site
          </Link>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 text-red-600 text-sm p-4 rounded-xl border border-red-100">{error}</div>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <div className="relative flex items-center justify-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
            <Activity className="absolute w-5 h-5 text-blue-600 animate-pulse" />
          </div>
        </div>
      ) : sites.length === 0 ? (
        <div className="bg-white rounded-2xl border border-gray-100 p-12 text-center shadow-sm">
          <Globe className="w-12 h-12 text-gray-300 mx-auto mb-4" />
          <p className="text-gray-500 font-medium mb-4">Henüz bir site eklenmemiş.</p>
          <Link
            to="/sites/new"
            className="bg-blue-600 text-white px-5 py-2.5 rounded-xl text-sm font-semibold hover:bg-blue-700 transition-colors inline-flex items-center gap-1.5 shadow-sm shadow-blue-500/10"
          >
            İlk Siteyi Ekle
          </Link>
        </div>
      ) : viewMode === 'grid' ? (
        /* Cards/Boxes Grid View */
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {sites.map((site) => (
            <div
              key={site.id}
              className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm hover:shadow-md hover:-translate-y-1 transition-all duration-300 flex flex-col justify-between group relative overflow-hidden"
            >
              {/* Type Accent Color Bar */}
              <div className={`absolute top-0 left-0 right-0 h-1 transition-colors ${
                site.status === 'active'
                  ? site.site_type === 'pocketbase'
                    ? 'bg-purple-500'
                    : site.site_type === 'proxy'
                    ? 'bg-amber-500'
                    : 'bg-blue-500'
                  : 'bg-gray-300'
              }`} />

              <div>
                {/* Top Header Section */}
                <div className="flex items-start justify-between mb-4">
                  <div className="space-y-1.5">
                    <Link
                      to={`/sites/${site.id}`}
                      className="text-lg font-bold text-gray-900 group-hover:text-blue-600 transition-colors flex items-center gap-1.5"
                    >
                      {site.name}
                      <ArrowRight className="w-4 h-4 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all duration-200 text-blue-600" />
                    </Link>
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-bold uppercase tracking-wider border ${
                        site.site_type === 'pocketbase'
                          ? 'bg-purple-50 text-purple-700 border-purple-100'
                          : site.site_type === 'proxy'
                          ? 'bg-amber-50 text-amber-700 border-amber-100'
                          : 'bg-blue-50 text-blue-700 border-blue-100'
                      }`}>
                        {site.site_type}
                      </span>
                      {site.git_repo && (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium bg-gray-50 text-gray-600 border border-gray-100">
                          <GitBranch className="w-3 h-3 text-gray-400" />
                          {site.git_branch || 'main'}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Status Indicator */}
                  <div className="flex items-center gap-2 bg-gray-50 border border-gray-100 px-2.5 py-1 rounded-full">
                    <span className="relative flex h-2 w-2">
                      {site.status === 'active' && (
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                      )}
                      <span className={`relative inline-flex rounded-full h-2 w-2 ${
                        site.status === 'active' ? 'bg-emerald-500' : 'bg-amber-500'
                      }`}></span>
                    </span>
                    <span className="text-[11px] text-gray-600 font-semibold">
                      {site.status === 'active' ? 'Aktif' : 'Durduruldu'}
                    </span>
                  </div>
                </div>

                {/* Connection & Port details */}
                <div className="space-y-2.5 my-5 border-t border-b border-gray-50 py-4">
                  {/* Domain */}
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-400 flex items-center gap-1.5">
                      <Globe className="w-4 h-4 text-gray-300" />
                      Domain
                    </span>
                    <a
                      href={`http://${site.domain}${site.port ? `:${site.port}` : ''}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-gray-700 hover:text-blue-600 font-medium flex items-center gap-1 transition-colors"
                    >
                      {site.domain}
                      <ExternalLink className="w-3.5 h-3.5 opacity-60" />
                    </a>
                  </div>

                  {/* Port */}
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-400 flex items-center gap-1.5">
                      <Terminal className="w-4 h-4 text-gray-300" />
                      Port
                    </span>
                    <span className="font-mono bg-gray-50 px-2 py-0.5 rounded border border-gray-100 text-gray-700 text-xs font-semibold">
                      {site.port}
                    </span>
                  </div>

                  {/* SSL Status */}
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-400 flex items-center gap-1.5">
                      <Shield className="w-4 h-4 text-gray-300" />
                      SSL
                    </span>
                    <span className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold border ${sslColors[site.ssl_status]}`}>
                      {site.ssl_status === 'active' ? (
                        <>
                          <ShieldCheck className="w-3.5 h-3.5 text-emerald-500" />
                          Aktif
                        </>
                      ) : site.ssl_status === 'pending' ? (
                        <>
                          <Shield className="w-3.5 h-3.5 text-amber-500 animate-pulse" />
                          Bekliyor
                        </>
                      ) : site.ssl_status === 'error' ? (
                        <>
                          <ShieldAlert className="w-3.5 h-3.5 text-red-500" />
                          Hata
                        </>
                      ) : (
                        'Yok'
                      )}
                    </span>
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex items-center justify-between gap-3 pt-2">
                <button
                  onClick={() => toggleStatus(site)}
                  className={`flex-1 flex items-center justify-center gap-1.5 text-xs font-bold px-3 py-2 rounded-xl transition-all duration-200 border ${
                    site.status === 'active'
                      ? 'bg-amber-50 text-amber-700 hover:bg-amber-100 border-amber-200'
                      : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100 border-emerald-200'
                  }`}
                >
                  {site.status === 'active' ? (
                    <>
                      <Pause className="w-3.5 h-3.5" />
                      Durdur
                    </>
                  ) : (
                    <>
                      <Play className="w-3.5 h-3.5" />
                      Başlat
                    </>
                  )}
                </button>

                <div className="flex items-center gap-2">
                  <Link
                    to={`/sites/${site.id}`}
                    className="text-xs font-semibold text-gray-700 hover:text-gray-900 bg-gray-50 hover:bg-gray-100 border border-gray-200 px-3.5 py-2 rounded-xl transition-colors"
                  >
                    Yönet
                  </Link>
                  <button
                    onClick={() => deleteSite(site)}
                    className="p-2 text-gray-400 hover:text-red-600 rounded-xl hover:bg-red-50 border border-transparent hover:border-red-100 transition-all duration-200"
                    title="Sil"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        /* Traditional Table List View */
        <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-gray-100 bg-gray-50/50">
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400">İsim</th>
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400">Domain</th>
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400">Port</th>
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400">Tür</th>
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400">SSL</th>
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400">Durum</th>
                  <th className="p-4 text-xs font-bold uppercase tracking-wider text-gray-400 text-right">İşlem</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {sites.map((site) => (
                  <tr key={site.id} className="hover:bg-gray-50/60 transition-colors">
                    <td className="p-4">
                      <Link to={`/sites/${site.id}`} className="font-semibold text-blue-600 hover:text-blue-800 flex items-center gap-1">
                        {site.name}
                        <ArrowRight className="w-3.5 h-3.5 opacity-0 hover:opacity-100 hover:translate-x-0.5 transition-all text-blue-600" />
                      </Link>
                    </td>
                    <td className="p-4 text-sm text-gray-600 font-medium">{site.domain}</td>
                    <td className="p-4 text-sm text-gray-600 font-mono">{site.port}</td>
                    <td className="p-4 text-sm">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold capitalize ${
                        site.site_type === 'pocketbase' ? 'text-purple-600' : site.site_type === 'proxy' ? 'text-amber-600' : 'text-blue-600'
                      }`}>
                        {site.site_type}
                      </span>
                    </td>
                    <td className="p-4">
                      <span className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold border ${sslColors[site.ssl_status]}`}>
                        {site.ssl_status === 'active' ? (
                          <>
                            <ShieldCheck className="w-3 h-3 text-emerald-500" />
                            Aktif
                          </>
                        ) : site.ssl_status === 'pending' ? (
                          <>
                            <Shield className="w-3 h-3 text-amber-500 animate-pulse" />
                            Bekliyor
                          </>
                        ) : site.ssl_status === 'error' ? (
                          <>
                            <ShieldAlert className="w-3 h-3 text-red-500" />
                            Hata
                          </>
                        ) : (
                          'Yok'
                        )}
                      </span>
                    </td>
                    <td className="p-4">
                      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold border ${statusColors[site.status]}`}>
                        <span className={`w-1.5 h-1.5 rounded-full ${site.status === 'active' ? 'bg-emerald-500' : 'bg-amber-500'}`} />
                        {site.status === 'active' ? 'Aktif' : 'Durduruldu'}
                      </span>
                    </td>
                    <td className="p-4 text-right">
                      <div className="flex justify-end items-center gap-2">
                        <button
                          onClick={() => toggleStatus(site)}
                          className={`text-xs font-bold px-3 py-1.5 rounded-lg border transition-colors ${
                            site.status === 'active'
                              ? 'bg-amber-50 text-amber-700 hover:bg-amber-100 border-amber-200'
                              : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100 border-emerald-200'
                          }`}
                        >
                          {site.status === 'active' ? 'Durdur' : 'Başlat'}
                        </button>
                        <Link
                          to={`/sites/${site.id}`}
                          className="text-xs font-semibold text-gray-700 hover:text-gray-900 bg-gray-50 hover:bg-gray-100 border border-gray-200 px-3 py-1.5 rounded-lg transition-colors"
                        >
                          Yönet
                        </Link>
                        <button
                          onClick={() => deleteSite(site)}
                          className="p-1.5 text-gray-400 hover:text-red-600 rounded-lg hover:bg-red-50 transition-colors"
                          title="Sil"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

