import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Site, Database, CreateDatabaseRequest } from '../types'

export default function SiteDetail() {
  const { id } = useParams()
  const navigate = useNavigate()

  const [site, setSite] = useState<Site | null>(null)
  const [databases, setDatabases] = useState<Database[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Database form
  const [showDbForm, setShowDbForm] = useState(false)
  const [dbName, setDbName] = useState('')
  const [dbEmail, setDbEmail] = useState('')
  const [dbLoading, setDbLoading] = useState(false)

  // Git deploy
  const [gitDeploying, setGitDeploying] = useState(false)

  // Logs state
  const [logType, setLogType] = useState<'nginx_access' | 'nginx_error' | 'service' | 'ssl'>('nginx_access')
  const [logs, setLogs] = useState('')
  const [logsLoading, setLogsLoading] = useState(false)
  const [liveLogs, setLiveLogs] = useState(false)

  async function loadLogs() {
    if (!id) return
    setLogsLoading(true)
    try {
      const res = await pb.send(`/api/dashboard/sites/${id}/logs?type=${logType}`, {
        method: 'GET',
      })
      setLogs(res.logs)
    } catch {
      setLogs('Loglar yüklenemedi.')
    } finally {
      setLogsLoading(false)
    }
  }

  async function clearLogs() {
    if (!id || !window.confirm("Bu log dosyasını temizlemek istediğinize emin misiniz?")) return
    try {
      await pb.send(`/api/dashboard/sites/${id}/logs/clear?type=${logType}`, {
        method: 'POST',
      })
      setLogs('Log dosyası temizlendi.')
    } catch (err: any) {
      alert('Hata: Log dosyası temizlenemedi. ' + (err?.message || ''))
    }
  }

  useEffect(() => {
    loadLogs()
  }, [id, logType])

  useEffect(() => {
    if (!liveLogs) return
    const interval = setInterval(loadLogs, 3000)
    return () => clearInterval(interval)
  }, [id, logType, liveLogs])

  useEffect(() => {
    const el = document.getElementById('log-viewer-box')
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  }, [logs])

  useEffect(() => {
    loadSite()
    loadDatabases()
  }, [id])

  async function loadSite() {
    try {
      const site = await pb.collection('sites').getOne<Site>(id!)
      setSite(site)
    } catch {
      setError('Site bulunamadı')
    } finally {
      setLoading(false)
    }
  }

  async function loadDatabases() {
    try {
      const records = await pb.send(`/api/dashboard/sites/${id}/databases`, {
        method: 'GET',
      })
      setDatabases(records as Database[])
    } catch {
      // ignore
    }
  }

  async function toggleSSL() {
    if (!site) return

    // Check if domain is an IP address
    const ipPattern = /^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$/
    if (site.ssl_status !== 'active' && ipPattern.test(site.domain)) {
      alert("Hata: IP adresleri (örneğin 10.2.42.87) veya yerel adresler için Let's Encrypt SSL sertifikası üretilemez. Lütfen sitenizin ayarlarından gerçek bir alan adı (domain) tanımlayın.")
      return
    }

    try {
      if (site.ssl_status === 'active') {
        await pb.send(`/api/dashboard/sites/${id}/ssl/disable`, { method: 'POST' })
      } else {
        await pb.send(`/api/dashboard/sites/${id}/ssl/enable`, { method: 'POST' })
      }
      loadSite()
    } catch (err: any) {
      alert('SSL işlemi başarısız: ' + (err?.message || ''))
    }
  }

  async function deploy() {
    try {
      await pb.send(`/api/dashboard/sites/${id}/deploy`, { method: 'POST' })
      alert('Site başarıyla deploy edildi!')
    } catch (err: any) {
      alert('Deploy başarısız: ' + (err?.message || ''))
    }
  }

  async function gitDeploy() {
    if (!site?.git_repo) return
    setGitDeploying(true)
    try {
      await pb.send(`/api/dashboard/sites/${id}/git-deploy`, { method: 'POST' })
      alert('GitHub deploy başlatıldı! Arka planda klonlanıp build ediliyor.')
    } catch (err: any) {
      alert('GitHub deploy başarısız: ' + (err?.message || ''))
    } finally {
      setGitDeploying(false)
    }
  }

  async function toggleStatus() {
    if (!site) return
    const newStatus = site.status === 'active' ? 'paused' : 'active'
    try {
      const updated = await pb.send<Site>(`/api/dashboard/sites/${id}`, {
        method: 'PATCH',
        body: { status: newStatus },
      })
      setSite(updated)
    } catch (err: any) {
      alert('Durum güncellenirken hata oluştu: ' + (err?.message || ''))
    }
  }

  async function handleCreateDb(e: React.FormEvent) {
    e.preventDefault()
    setDbLoading(true)
    try {
      await pb.send(`/api/dashboard/sites/${id}/databases`, {
        method: 'POST',
        body: { name: dbName, admin_email: dbEmail } as CreateDatabaseRequest,
      })
      setShowDbForm(false)
      setDbName('')
      setDbEmail('')
      loadDatabases()
    } catch (err: any) {
      alert('Veritabanı oluşturulamadı: ' + (err?.message || ''))
    } finally {
      setDbLoading(false)
    }
  }

  async function deleteDatabase(db: Database) {
    if (!confirm(`"${db.name}" veritabanını silmek istediğinize emin misiniz?`)) return
    try {
      await pb.send(`/api/dashboard/sites/${id}/databases/${db.id}`, {
        method: 'DELETE',
      })
      loadDatabases()
    } catch {
      alert('Silme başarısız')
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    )
  }

  if (error || !site) {
    return (
      <div className="text-center py-12">
        <p className="text-red-600 mb-4">{error || 'Site bulunamadı'}</p>
        <Link to="/sites" className="text-blue-600 hover:underline">Sitelere Dön</Link>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link to="/sites" className="text-gray-400 hover:text-gray-600">&larr; Siteler</Link>
        <h1 className="text-2xl font-bold">{site.name}</h1>
        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
          site.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'
        }`}>
          {site.status === 'active' ? 'Aktif' : 'Duraklatıldı'}
        </span>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Site Details */}
        <div className="lg:col-span-2 space-y-6">

          {/* Info Card */}
          <div className="bg-white rounded-xl shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold">Site Bilgileri</h2>
              <Link to={`/sites/${id}/edit`} className="text-sm text-blue-600 hover:underline">
                Düzenle
              </Link>
            </div>
            <dl className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <dt className="text-gray-500">Domain</dt>
                <dd className="font-medium mt-1">{site.domain}</dd>
              </div>
              <div>
                <dt className="text-gray-500">Port (Nginx / Backend)</dt>
                <dd className="font-medium mt-1">
                  {site.port}
                  {site.site_type === 'pocketbase' && site.proxy_url && (
                    <span className="text-xs text-gray-400 block mt-0.5">
                      Backend: {site.proxy_url.split(':').pop()}
                    </span>
                  )}
                </dd>
              </div>
              <div>
                <dt className="text-gray-500">Site Türü</dt>
                <dd className="font-medium mt-1 capitalize">
                  {site.site_type === 'pocketbase' ? 'PocketBase Backend' : site.site_type}
                </dd>
              </div>
              {site.site_type === 'proxy' && (
                <div>
                  <dt className="text-gray-500">Proxy URL</dt>
                  <dd className="font-medium mt-1">{site.proxy_url}</dd>
                </div>
              )}
              {site.site_type === 'pocketbase' && (
                <>
                  <div>
                    <dt className="text-gray-500">PocketBase Admin Email</dt>
                    <dd className="font-medium mt-1">{site.admin_email}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">PocketBase Admin Şifresi</dt>
                    <dd className="font-mono text-xs mt-1 bg-gray-50 p-1 rounded border inline-block">
                      {site.admin_password}
                    </dd>
                  </div>
                </>
              )}
              <div>
                <dt className="text-gray-500">Kök Dizin (Statik Dosyalar)</dt>
                <dd className="font-mono text-xs mt-1">{site.root_dir}</dd>
              </div>
              {site.notes && (
                <div className="col-span-2">
                  <dt className="text-gray-500">Notlar</dt>
                  <dd className="mt-1">{site.notes}</dd>
                </div>
              )}
              {site.git_repo && (
                <div className="col-span-2">
                  <dt className="text-gray-500 flex items-center gap-1.5">
                    <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                    </svg>
                    GitHub Repo
                  </dt>
                  <dd className="font-mono text-xs mt-1">
                    <a
                      href={site.git_repo}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-600 hover:underline"
                    >
                      {site.git_repo}
                    </a>
                  </dd>
                </div>
              )}
            </dl>
          </div>

          {/* SSL Card */}
          <div className="bg-white rounded-xl shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold">SSL / Let's Encrypt</h2>
              <div className="flex gap-2">
                {site.ssl_status === 'error' && (
                  <button
                    onClick={async () => {
                      try {
                        await pb.send(`/api/dashboard/sites/${id}/ssl/disable`, { method: 'POST' })
                        loadSite()
                      } catch (err: any) {
                        alert('Sıfırlama işlemi başarısız: ' + (err?.message || ''))
                      }
                    }}
                    className="px-3 py-1.5 rounded-lg text-sm font-medium bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
                  >
                    Sıfırla
                  </button>
                )}
                <button
                  onClick={toggleSSL}
                  className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                    site.ssl_status === 'active'
                      ? 'bg-red-50 text-red-600 hover:bg-red-100'
                      : 'bg-green-50 text-green-600 hover:bg-green-100'
                  }`}
                >
                  {site.ssl_status === 'active' ? 'SSL\'yi Kaldır' : 'SSL Ekle'}
                </button>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                site.ssl_status === 'active' ? 'bg-green-100 text-green-700' :
                site.ssl_status === 'pending' ? 'bg-yellow-100 text-yellow-700' :
                site.ssl_status === 'error' ? 'bg-red-100 text-red-700' :
                'bg-gray-100 text-gray-600'
              }`}>
                {site.ssl_status === 'active' ? 'Aktif' :
                 site.ssl_status === 'pending' ? 'İşleniyor...' :
                 site.ssl_status === 'error' ? 'Hata' : 'Yok'}
              </span>
              {site.ssl_expiry && (
                <span className="text-sm text-gray-500">
                  Geçerlilik: {new Date(site.ssl_expiry).toLocaleDateString('tr-TR')}
                </span>
              )}
            </div>
          </div>

          {/* Databases Card */}
          {site.site_type !== 'pocketbase' && (
            <div className="bg-white rounded-xl shadow-sm p-6">
              <div className="flex items-center justify-between mb-4">
                <h2 className="font-semibold">Veritabanları</h2>
                <button
                  onClick={() => setShowDbForm(!showDbForm)}
                  className="bg-blue-600 text-white px-3 py-1.5 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
                >
                  + Veritabanı Ekle
                </button>
              </div>

              {showDbForm && (
                <form onSubmit={handleCreateDb} className="mb-4 p-4 bg-gray-50 rounded-lg space-y-3">
                  <div>
                    <label className="block text-xs font-medium text-gray-600 mb-1">Veritabanı Adı</label>
                    <input
                      type="text"
                      value={dbName}
                      onChange={(e) => setDbName(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="ör. blog-db"
                      required
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-gray-600 mb-1">Admin Email</label>
                    <input
                      type="email"
                      value={dbEmail}
                      onChange={(e) => setDbEmail(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="admin@site.com"
                      required
                    />
                  </div>
                  <div className="flex gap-2">
                    <button type="submit" disabled={dbLoading}
                      className="bg-blue-600 text-white px-3 py-1.5 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
                    >
                      {dbLoading ? 'Oluşturuluyor...' : 'Oluştur'}
                    </button>
                    <button type="button" onClick={() => setShowDbForm(false)}
                      className="px-3 py-1.5 border rounded-lg text-sm text-gray-600 hover:bg-gray-100"
                    >
                      İptal
                    </button>
                  </div>
                </form>
              )}

              {databases.length === 0 ? (
                <p className="text-sm text-gray-500">Henüz bir veritabanı eklenmemiş.</p>
              ) : (
                <div className="space-y-2">
                  {databases.map((db) => (
                    <div key={db.id} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                      <div>
                        <p className="font-medium text-sm">{db.name}</p>
                        <p className="text-xs text-gray-500">Port: {db.port} | {db.admin_email}</p>
                      </div>
                      <div className="flex items-center gap-3">
                        <span className="text-xs text-gray-400">{db.db_type}</span>
                        <a
                          href={`http://${site.domain}:${db.port}/_/`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-blue-600 hover:text-blue-800 text-sm font-medium"
                        >
                          Yönet
                        </a>
                        <button
                          onClick={() => deleteDatabase(db)}
                          className="text-red-500 hover:text-red-700 text-sm"
                        >
                          Sil
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Log İzleyici Card */}
          <div className="bg-white rounded-xl shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-gray-700">Log İzleyici</h2>
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-1 text-xs text-gray-500 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={liveLogs}
                    onChange={(e) => setLiveLogs(e.target.checked)}
                    className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  />
                  Canlı Akış
                </label>
                {logType !== 'service' && (
                  <button
                    onClick={clearLogs}
                    className="text-xs text-red-600 hover:underline"
                  >
                    Temizle
                  </button>
                )}
                <button
                  onClick={loadLogs}
                  disabled={logsLoading}
                  className="text-xs text-blue-600 hover:underline"
                >
                  Yenile
                </button>
              </div>
            </div>

            <div className="flex border-b border-gray-200 mb-4 overflow-x-auto">
              <button
                onClick={() => setLogType('nginx_access')}
                className={`py-2 px-4 border-b-2 text-sm font-medium whitespace-nowrap transition-colors ${
                  logType === 'nginx_access'
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                Nginx Access
              </button>
              <button
                onClick={() => setLogType('nginx_error')}
                className={`py-2 px-4 border-b-2 text-sm font-medium whitespace-nowrap transition-colors ${
                  logType === 'nginx_error'
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                Nginx Error
              </button>
              <button
                onClick={() => setLogType('ssl')}
                className={`py-2 px-4 border-b-2 text-sm font-medium whitespace-nowrap transition-colors ${
                  logType === 'ssl'
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                SSL (Let's Encrypt) Logları
              </button>
              {site.site_type === 'pocketbase' && (
                <button
                  onClick={() => setLogType('service')}
                  className={`py-2 px-4 border-b-2 text-sm font-medium whitespace-nowrap transition-colors ${
                    logType === 'service'
                      ? 'border-blue-500 text-blue-600'
                      : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                  }`}
                >
                  Servis Logları
                </button>
              )}
            </div>

            <div
              id="log-viewer-box"
              className="bg-gray-950 text-gray-200 rounded-lg p-4 font-mono text-xs overflow-y-auto h-80 whitespace-pre-wrap leading-relaxed shadow-inner border border-gray-800"
            >
              {logsLoading && logs === '' ? (
                <p className="text-gray-500 animate-pulse">Yükleniyor...</p>
              ) : (
                logs
              )}
            </div>
          </div>
        </div>

        {/* Sidebar Actions */}
        <div className="space-y-4">
          <div className="bg-white rounded-xl shadow-sm p-5">
            <h3 className="font-semibold mb-3">Aksiyonlar</h3>
            <div className="space-y-2">
              <button
                onClick={toggleStatus}
                className={`w-full px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                  site.status === 'active'
                    ? 'bg-yellow-50 text-yellow-700 hover:bg-yellow-100'
                    : 'bg-green-600 text-white hover:bg-green-700'
                }`}
              >
                {site.status === 'active' ? 'Siteyi Durdur (Pasif Yap)' : 'Siteyi Başlat (Aktif Yap)'}
              </button>
              <button
                onClick={deploy}
                disabled={site.status === 'paused'}
                className="w-full bg-green-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-green-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Deploy Et (Nginx Reload)
              </button>
              {site.git_repo && (
                <button
                  onClick={gitDeploy}
                  disabled={site.status === 'paused' || gitDeploying}
                  className="w-full flex items-center justify-center gap-2 bg-gray-900 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                  </svg>
                  {gitDeploying ? 'Deploy Ediliyor...' : "GitHub'dan Güncelle"}
                </button>
              )}
              {site.site_type === 'pocketbase' && (
                <a
                  href={`http://${site.domain}:${site.port}/_/`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`block w-full text-center bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors ${
                    site.status === 'paused' ? 'pointer-events-none opacity-50' : ''
                  }`}
                >
                  PocketBase Admin Panel
                </a>
              )}
              <a
                href={`http://${site.domain}:${site.port}`}
                target="_blank"
                rel="noopener noreferrer"
                className={`block w-full text-center border border-gray-300 px-4 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors ${
                  site.status === 'paused' ? 'pointer-events-none opacity-50' : ''
                }`}
              >
                Siteyi Aç
              </a>
            </div>
          </div>

          <div className="bg-white rounded-xl shadow-sm p-5">
            <h3 className="font-semibold mb-2">Hızlı Bilgiler</h3>
            <ul className="text-sm text-gray-600 space-y-2">
              <li><span className="text-gray-400">Oluşturulma:</span><br />{new Date(site.created).toLocaleDateString('tr-TR')}</li>
              <li><span className="text-gray-400">Güncellenme:</span><br />{new Date(site.updated).toLocaleDateString('tr-TR')}</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}
