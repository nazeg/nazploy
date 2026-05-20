import { useEffect, useState } from 'react'
import pb from '../lib/pocketbase'

export default function NginxStatus() {
  const [running, setRunning] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    checkStatus()
  }, [])

  async function checkStatus() {
    try {
      const res: any = await pb.send('/api/dashboard/nginx/status', { method: 'GET' })
      setRunning(res.running)
    } catch {
      setRunning(false)
    } finally {
      setLoading(false)
    }
  }

  async function reloadNginx() {
    try {
      await pb.send('/api/dashboard/nginx/reload', { method: 'POST' })
      alert('Nginx başarıyla yeniden yüklendi!')
      checkStatus()
    } catch (err: any) {
      alert('Nginx reload başarısız: ' + (err?.message || ''))
    }
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Nginx Yönetimi</h1>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white rounded-xl shadow-sm p-6">
          <h2 className="font-semibold mb-4">Servis Durumu</h2>

          {loading ? (
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600" />
          ) : (
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <div className={`w-3 h-3 rounded-full ${running ? 'bg-green-500' : 'bg-red-500'}`} />
                <span className="font-medium">
                  Nginx {running ? 'Çalışıyor' : 'Çalışmıyor'}
                </span>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={reloadNginx}
                  className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
                >
                  Nginx'i Yeniden Yükle
                </button>
                <button
                  onClick={checkStatus}
                  className="px-4 py-2 border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  Durumu Kontrol Et
                </button>
              </div>
            </div>
          )}
        </div>

        <div className="bg-white rounded-xl shadow-sm p-6">
          <h2 className="font-semibold mb-4">Komutlar</h2>
          <div className="space-y-3 text-sm">
            <div className="p-3 bg-gray-50 rounded-lg">
              <code className="text-blue-600">nginx -t</code>
              <p className="text-gray-500 mt-1">Nginx konfigürasyonunu test eder</p>
            </div>
            <div className="p-3 bg-gray-50 rounded-lg">
              <code className="text-blue-600">nginx -s reload</code>
              <p className="text-gray-500 mt-1">Nginx'i kesintisiz yeniden başlatır</p>
            </div>
            <div className="p-3 bg-gray-50 rounded-lg">
              <code className="text-blue-600">systemctl status nginx</code>
              <p className="text-gray-500 mt-1">Servis durumunu gösterir</p>
            </div>
            <div className="p-3 bg-gray-50 rounded-lg">
              <code className="text-blue-600">journalctl -u nginx --no-pager -n 50</code>
              <p className="text-gray-500 mt-1">Son 50 Nginx log satırını gösterir</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}