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
            </dl>
          </div>

          {/* SSL Card */}
          <div className="bg-white rounded-xl shadow-sm p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold">SSL / Let's Encrypt</h2>
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
        </div>

        {/* Sidebar Actions */}
        <div className="space-y-4">
          <div className="bg-white rounded-xl shadow-sm p-5">
            <h3 className="font-semibold mb-3">Aksiyonlar</h3>
            <div className="space-y-2">
              <button
                onClick={deploy}
                className="w-full bg-green-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-green-700 transition-colors"
              >
                Deploy Et (Nginx Reload)
              </button>
              {site.site_type === 'pocketbase' && (
                <a
                  href={`http://${site.domain}:${site.port}/_/`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="block w-full text-center bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
                >
                  PocketBase Admin Panel
                </a>
              )}
              <a
                href={`http://${site.domain}:${site.port}`}
                target="_blank"
                rel="noopener noreferrer"
                className="block w-full text-center border border-gray-300 px-4 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
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
