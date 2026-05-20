import { FormEvent, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import pb from '../lib/pocketbase'
import type { Site } from '../types'

export default function SiteForm() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isEdit = !!id

  const [name, setName] = useState('')
  const [domain, setDomain] = useState('')
  const [port, setPort] = useState<number | ''>('')
  const [siteType, setSiteType] = useState<'static' | 'proxy' | 'pocketbase'>('static')
  const [proxyUrl, setProxyUrl] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [notes, setNotes] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

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
