import { FormEvent, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Site } from '../types'

export default function SiteForm() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isEdit = !!id

  const [name, setName] = useState('')
  const [domain, setDomain] = useState(() => {
    return !id ? window.location.hostname : ''
  })
  const [port, setPort] = useState<number | ''>('')
  const [siteType, setSiteType] = useState<'static' | 'proxy' | 'pocketbase'>('static')
  const [proxyUrl, setProxyUrl] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [notes, setNotes] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Git deploy state
  const [useGitDeploy, setUseGitDeploy] = useState(false)
  const [gitRepo, setGitRepo] = useState('')
  const [gitBranch, setGitBranch] = useState('')
  const [buildCmd, setBuildCmd] = useState('')
  const [outputDir, setOutputDir] = useState('')

  useEffect(() => {
    if (isEdit) {
      loadSite()
    }
  }, [id])

  async function loadSite() {
    try {
      const site = await pb.collection('sites').getOne<Site>(id!)
      setName(site.name)
      setDomain(site.domain)
      setPort(site.port)
      setSiteType(site.site_type)
      setProxyUrl(site.proxy_url || '')
      setAdminEmail(site.admin_email || '')
      setNotes(site.notes || '')
      if (site.git_repo) {
        setUseGitDeploy(true)
        setGitRepo(site.git_repo)
        setGitBranch(site.git_branch || '')
        setBuildCmd(site.build_cmd || '')
        setOutputDir(site.output_dir || '')
      }
    } catch {
      setError('Site yüklenemedi')
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const body: any = {
        name,
        domain,
        port: port ? Number(port) : undefined,
        site_type: siteType,
        proxy_url: siteType === 'proxy' ? proxyUrl : '',
        notes,
      }

      // Attach git deploy fields
      if (useGitDeploy && gitRepo && siteType !== 'proxy') {
        body.git_repo = gitRepo
        body.git_branch = gitBranch
        body.build_cmd = buildCmd
        body.output_dir = outputDir
      } else {
        body.git_repo = ''
        body.git_branch = ''
        body.build_cmd = ''
        body.output_dir = ''
      }

      if (isEdit) {
        await pb.send(`/api/dashboard/sites/${id}`, {
          method: 'PATCH',
          body,
        })
      } else {
        await pb.send('/api/dashboard/sites', {
          method: 'POST',
          body: {
            ...body,
            admin_email: siteType === 'pocketbase' ? adminEmail : '',
          },
        })
      }
      navigate('/sites')
    } catch (err: any) {
      setError(err?.message || 'Site kaydedilemedi')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">
        {isEdit ? 'Siteyi Düzenle' : 'Yeni Site Ekle'}
      </h1>

      <form onSubmit={handleSubmit} className="bg-white rounded-xl shadow-sm p-6 space-y-5">
        {error && (
          <div className="bg-red-50 text-red-600 text-sm p-3 rounded-lg">{error}</div>
        )}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Site Adı</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="ör. Blog, E-Ticaret, Portföy"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Domain / IP</label>
          <input
            type="text"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="ör. example.com veya 10.2.42.87"
            required
          />
          <p className="text-xs text-gray-400 mt-1">
            Sitenize ait domain adresi veya IP. (Not: IP adresleri için Let's Encrypt SSL desteklenmemektedir).
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Port</label>
          <input
            type="number"
            value={port}
            onChange={(e) => setPort(e.target.value === '' ? '' : Number(e.target.value))}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder={isEdit ? "ör. 10000" : "Boş bırakılırsa otomatik atanacaktır"}
          />
          <p className="text-xs text-gray-400 mt-1">
            Nginx / Backend dinleme portu. Boş bırakırsanız sistem otomatik boş bir port (10000-20000 arası) atar.
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">Site Türü</label>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => setSiteType('static')}
              disabled={isEdit}
              className={`flex-1 p-3 rounded-lg border-2 text-center transition-colors ${
                siteType === 'static'
                  ? 'border-blue-500 bg-blue-50 text-blue-700'
                  : 'border-gray-200 hover:border-gray-300'
              } ${isEdit ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
              <div className="font-medium">Statik Site</div>
              <div className="text-xs mt-1 text-gray-500">HTML/CSS/JS dosyaları</div>
            </button>
            <button
              type="button"
              onClick={() => setSiteType('proxy')}
              disabled={isEdit}
              className={`flex-1 p-3 rounded-lg border-2 text-center transition-colors ${
                siteType === 'proxy'
                  ? 'border-blue-500 bg-blue-50 text-blue-700'
                  : 'border-gray-200 hover:border-gray-300'
              } ${isEdit ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
              <div className="font-medium">Proxy</div>
              <div className="text-xs mt-1 text-gray-500">Başka bir porta yönlendir</div>
            </button>
            <button
              type="button"
              onClick={() => setSiteType('pocketbase')}
              disabled={isEdit}
              className={`flex-1 p-3 rounded-lg border-2 text-center transition-colors ${
                siteType === 'pocketbase'
                  ? 'border-blue-500 bg-blue-50 text-blue-700'
                  : 'border-gray-200 hover:border-gray-300'
              } ${isEdit ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
              <div className="font-medium">PocketBase Backend</div>
              <div className="text-xs mt-1 text-gray-500">Veritabanı + Statik Site</div>
            </button>
          </div>
        </div>

        {siteType === 'proxy' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Proxy URL</label>
            <input
              type="text"
              value={proxyUrl}
              onChange={(e) => setProxyUrl(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="ör. http://localhost:3000"
            />
            <p className="text-xs text-gray-400 mt-1">İsteğin yönlendirileceği adres (ör. bir Node.js uygulaması)</p>
          </div>
        )}

        {siteType === 'pocketbase' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Admin E-posta</label>
            <input
              type="email"
              value={adminEmail}
              onChange={(e) => setAdminEmail(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="ör. admin@siteniz.com"
              required
              disabled={isEdit}
            />
            <p className="text-xs text-gray-400 mt-1">
              {isEdit ? 'PocketBase admin e-posta adresi değiştirilemez.' : 'Yeni PocketBase instance\'ınız için oluşturulacak admin hesabı e-postası. Şifre otomatik üretilecektir.'}
            </p>
          </div>
        )}

        {/* ── GitHub Deploy Section ── */}
        {siteType !== 'proxy' && (
          <div className="border border-gray-200 rounded-xl overflow-hidden transition-all">
            <button
              type="button"
              onClick={() => setUseGitDeploy(!useGitDeploy)}
              className={`w-full flex items-center justify-between p-4 text-left transition-colors ${
                useGitDeploy ? 'bg-gray-900 text-white' : 'bg-gray-50 hover:bg-gray-100 text-gray-700'
              }`}
            >
              <div className="flex items-center gap-3">
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                </svg>
                <div>
                  <div className="font-medium text-sm">GitHub'dan Deploy Et</div>
                  <div className={`text-xs mt-0.5 ${useGitDeploy ? 'text-gray-400' : 'text-gray-500'}`}>
                    GitHub reposu klonlanır, otomatik build edilir ve yayınlanır
                  </div>
                </div>
              </div>
              <div className={`w-10 h-6 rounded-full relative transition-colors ${
                useGitDeploy ? 'bg-green-500' : 'bg-gray-300'
              }`}>
                <div className={`absolute top-1 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                  useGitDeploy ? 'translate-x-5' : 'translate-x-1'
                }`} />
              </div>
            </button>

            {useGitDeploy && (
              <div className="p-4 space-y-4 border-t border-gray-200 bg-white">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    GitHub Repo URL <span className="text-red-500">*</span>
                  </label>
                  <div className="relative">
                    <input
                      type="url"
                      value={gitRepo}
                      onChange={(e) => setGitRepo(e.target.value)}
                      className="w-full pl-10 pr-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                      placeholder="https://github.com/kullanici/proje"
                      required={useGitDeploy}
                    />
                    <svg className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                    </svg>
                  </div>
                  <p className="text-xs text-gray-400 mt-1">Sadece public repolar desteklenmektedir.</p>
                </div>

                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Branch</label>
                    <input
                      type="text"
                      value={gitBranch}
                      onChange={(e) => setGitBranch(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                      placeholder="main"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Build Komutu</label>
                    <input
                      type="text"
                      value={buildCmd}
                      onChange={(e) => setBuildCmd(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                      placeholder="Otomatik"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Output Dizini</label>
                    <input
                      type="text"
                      value={outputDir}
                      onChange={(e) => setOutputDir(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                      placeholder="Otomatik"
                    />
                  </div>
                </div>
                <p className="text-xs text-gray-400 -mt-2">
                  Boş bırakırsanız framework otomatik tespit edilir (Vite, Next.js, Astro vb.)
                </p>
              </div>
            )}
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Notlar (isteğe bağlı)</label>
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            rows={3}
            placeholder="Site hakkında notlar..."
          />
        </div>

        <div className="flex gap-3 pt-2">
          <button
            type="submit"
            disabled={loading}
            className="bg-blue-600 text-white px-6 py-2.5 rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 transition-colors"
          >
            {loading ? 'Kaydediliyor...' : isEdit ? 'Güncelle' : 'Siteyi Oluştur'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/sites')}
            className="px-6 py-2.5 border border-gray-300 rounded-lg font-medium text-gray-700 hover:bg-gray-50 transition-colors"
          >
            İptal
          </button>
        </div>
      </form>
    </div>
  )
}

