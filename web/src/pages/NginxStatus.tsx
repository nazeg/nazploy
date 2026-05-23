import { useEffect, useState } from 'react'
import pb from '../lib/pocketbase'
import {
  Server,
  RefreshCw,
  Terminal,
  CheckCircle2,
  AlertCircle,
  Copy,
  Check,
  FileText,
  Settings,
  ShieldCheck,
  ShieldAlert,
  ChevronDown,
  ChevronUp,
  Activity
} from 'lucide-react'

interface NginxStatusData {
  running: boolean
  status_output: string
  config_test: string
}

export default function NginxStatus() {
  const [status, setStatus] = useState<NginxStatusData | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [reloading, setReloading] = useState(false)
  const [showStatusDetails, setShowStatusDetails] = useState(false)
  
  // Logs states
  const [activeTab, setActiveTab] = useState<'nginx' | 'nazploy'>('nginx')
  const [logs, setLogs] = useState<string>('')
  const [logsLoading, setLogsLoading] = useState(false)
  const [lineCount, setLineCount] = useState<number>(100)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    loadAll()
  }, [activeTab, lineCount])

  async function loadAll() {
    setLoading(true)
    await Promise.all([checkStatus(), fetchLogs()])
    setLoading(false)
  }

  async function handleRefresh() {
    setRefreshing(true)
    await Promise.all([checkStatus(), fetchLogs()])
    setRefreshing(false)
  }

  async function checkStatus() {
    try {
      const res = await pb.send<NginxStatusData>('/api/dashboard/nginx/status', { method: 'GET' })
      setStatus(res)
    } catch {
      setStatus({
        running: false,
        status_output: 'Hata: Servis durum bilgisi alınamadı.',
        config_test: 'Hata: Konfigürasyon testi yapılamadı.'
      })
    }
  }

  async function fetchLogs() {
    setLogsLoading(true)
    try {
      const res = await pb.send<{ logs: string }>(
        `/api/dashboard/nginx/logs?service=${activeTab}&lines=${lineCount}`,
        { method: 'GET' }
      )
      setLogs(res.logs || 'Kayıt bulunamadı.')
    } catch {
      setLogs('Hata: Sistem logları yüklenemedi.')
    } finally {
      setLogsLoading(false)
    }
  }

  async function reloadNginx() {
    setReloading(true)
    try {
      await pb.send('/api/dashboard/nginx/reload', { method: 'POST' })
      alert('Nginx başarıyla yeniden yüklendi!')
      await checkStatus()
    } catch (err: any) {
      alert('Nginx reload başarısız: ' + (err?.message || ''))
    } finally {
      setReloading(false)
    }
  }

  const handleCopyLogs = () => {
    navigator.clipboard.writeText(logs)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const isConfigOk = status?.config_test.toLowerCase().includes('successful') || 
                     status?.config_test.toLowerCase().includes('syntax is ok')

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">Nginx & Servis Yönetimi</h1>
          <p className="text-sm text-gray-500 mt-1">
            Nginx web sunucusu durumunu izleyin, konfigürasyonu test edin ve sistem loglarını kontrol edin.
          </p>
        </div>
        <button
          onClick={handleRefresh}
          disabled={loading || refreshing}
          className="bg-white border border-gray-200 text-gray-700 px-4 py-2 rounded-xl text-sm font-semibold hover:bg-gray-50 transition-colors flex items-center justify-center gap-2 shadow-sm"
        >
          <RefreshCw className={`w-4 h-4 ${(loading || refreshing) ? 'animate-spin text-blue-600' : 'text-gray-500'}`} />
          Güncelle
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Nginx Servis Durumu */}
        <div className="lg:col-span-2 space-y-6">
          <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="p-3 bg-blue-50 text-blue-600 rounded-xl">
                  <Server className="w-6 h-6" />
                </div>
                <div>
                  <h2 className="font-bold text-gray-900">Nginx Web Sunucusu</h2>
                  <p className="text-xs text-gray-500 mt-0.5">Sistem Nginx Servisi Durumu</p>
                </div>
              </div>

              <div className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-bold border ${
                status?.running
                  ? 'bg-emerald-50 text-emerald-700 border-emerald-100'
                  : 'bg-rose-50 text-rose-700 border-rose-100'
              }`}>
                <span className="relative flex h-2 w-2">
                  {status?.running && (
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                  )}
                  <span className={`relative inline-flex rounded-full h-2 w-2 ${
                    status?.running ? 'bg-emerald-500' : 'bg-rose-500'
                  }`}></span>
                </span>
                {status?.running ? 'ÇALIŞIYOR' : 'DURDURULDU'}
              </div>
            </div>

            {/* Config Test Banner */}
            {status && (
              <div className={`p-4 rounded-xl border flex items-start gap-3 ${
                isConfigOk
                  ? 'bg-emerald-50/50 text-emerald-800 border-emerald-100'
                  : 'bg-rose-50/50 text-rose-800 border-rose-100'
              }`}>
                {isConfigOk ? (
                  <ShieldCheck className="w-5 h-5 text-emerald-600 shrink-0 mt-0.5" />
                ) : (
                  <ShieldAlert className="w-5 h-5 text-rose-600 shrink-0 mt-0.5" />
                )}
                <div>
                  <p className="text-sm font-semibold">
                    {isConfigOk ? 'Konfigürasyon Testi Başarılı' : 'Konfigürasyon Testi Başarısız'}
                  </p>
                  <p className="text-xs mt-1 text-gray-600 font-mono break-all whitespace-pre-line leading-relaxed">
                    {status.config_test}
                  </p>
                </div>
              </div>
            )}

            {/* Action Buttons */}
            <div className="flex flex-wrap items-center gap-3 border-t border-gray-50 pt-6">
              <button
                onClick={reloadNginx}
                disabled={reloading}
                className="bg-blue-600 text-white px-5 py-2.5 rounded-xl text-sm font-semibold hover:bg-blue-700 transition-colors shadow-sm shadow-blue-500/10 flex items-center justify-center gap-2"
              >
                <Activity className={`w-4 h-4 ${reloading ? 'animate-pulse' : ''}`} />
                {reloading ? 'Yeniden Yükleniyor...' : "Nginx'i Yeniden Yükle"}
              </button>

              <button
                onClick={() => setShowStatusDetails(!showStatusDetails)}
                className="bg-gray-50 hover:bg-gray-100 border border-gray-200 text-gray-700 px-4 py-2.5 rounded-xl text-sm font-semibold transition-colors flex items-center justify-center gap-2 ml-auto"
              >
                {showStatusDetails ? (
                  <>
                    Detayları Gizle
                    <ChevronUp className="w-4 h-4 text-gray-500" />
                  </>
                ) : (
                  <>
                    systemctl status Detayları
                    <ChevronDown className="w-4 h-4 text-gray-500" />
                  </>
                )}
              </button>
            </div>

            {/* Collapsible Details */}
            {showStatusDetails && status && (
              <div className="border-t border-gray-100 pt-4 animate-fadeIn">
                <div className="flex items-center gap-1.5 text-xs text-gray-500 mb-2">
                  <Terminal className="w-3.5 h-3.5" />
                  <span>systemctl status nginx output:</span>
                </div>
                <pre className="bg-zinc-950 text-zinc-300 font-mono text-[11px] p-4 rounded-xl overflow-x-auto max-h-72 border border-zinc-800 leading-relaxed">
                  {status.status_output}
                </pre>
              </div>
            )}
          </div>
        </div>

        {/* Nginx Komutları Rehberi */}
        <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-3 mb-6">
              <div className="p-3 bg-amber-50 text-amber-600 rounded-xl">
                <Settings className="w-6 h-6" />
              </div>
              <div>
                <h2 className="font-bold text-gray-900">Nginx Hızlı Komutlar</h2>
                <p className="text-xs text-gray-500 mt-0.5">Yönetim ve Hata Ayıklama</p>
              </div>
            </div>

            <div className="space-y-4">
              <div className="p-3 bg-gray-50 rounded-xl border border-gray-100">
                <code className="text-xs font-bold text-blue-600 block">nginx -t</code>
                <p className="text-xs text-gray-500 mt-1">Konfigürasyon dosyalarını test eder.</p>
              </div>
              <div className="p-3 bg-gray-50 rounded-xl border border-gray-100">
                <code className="text-xs font-bold text-blue-600 block">systemctl reload nginx</code>
                <p className="text-xs text-gray-500 mt-1">Bağlantıları kesmeden ayarları uygular.</p>
              </div>
              <div className="p-3 bg-gray-50 rounded-xl border border-gray-100">
                <code className="text-xs font-bold text-blue-600 block">systemctl status nginx</code>
                <p className="text-xs text-gray-500 mt-1">Servis çalışma detaylarını gösterir.</p>
              </div>
            </div>
          </div>

          <div className="border-t border-gray-50 pt-6 mt-6">
            <span className="text-[10px] uppercase font-bold text-gray-400 tracking-wider">Sunucu Bilgisi</span>
            <div className="flex items-center justify-between text-xs text-gray-600 mt-2">
              <span>Nginx Versiyonu:</span>
              <span className="font-mono bg-gray-100 px-2 py-0.5 rounded text-gray-700">nginx/latest</span>
            </div>
          </div>
        </div>
      </div>

      {/* Servis & Uygulama Logları */}
      <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-gray-50 pb-6 mb-6">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-indigo-50 text-indigo-600 rounded-xl">
              <FileText className="w-6 h-6" />
            </div>
            <div>
              <h2 className="font-bold text-gray-900">Sistem Logları</h2>
              <p className="text-xs text-gray-500 mt-0.5">Logları anlık izleyin (journalctl)</p>
            </div>
          </div>

          <div className="flex items-center flex-wrap gap-3">
            {/* Service Toggle */}
            <div className="flex bg-gray-100 p-1 rounded-xl border border-gray-200">
              <button
                onClick={() => setActiveTab('nginx')}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                  activeTab === 'nginx'
                    ? 'bg-white text-blue-600 shadow-sm'
                    : 'text-gray-500 hover:text-gray-900'
                }`}
              >
                Nginx Servisi
              </button>
              <button
                onClick={() => setActiveTab('nazploy')}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                  activeTab === 'nazploy'
                    ? 'bg-white text-blue-600 shadow-sm'
                    : 'text-gray-500 hover:text-gray-900'
                }`}
              >
                Nazploy Dashboard
              </button>
            </div>

            {/* Line Count Select */}
            <select
              value={lineCount}
              onChange={(e) => setLineCount(Number(e.target.value))}
              className="bg-white border border-gray-200 text-gray-700 text-xs font-semibold px-3 py-1.5 rounded-xl outline-none cursor-pointer focus:ring-1 focus:ring-blue-500"
            >
              <option value={50}>50 Satır</option>
              <option value={100}>100 Satır</option>
              <option value={200}>200 Satır</option>
              <option value={500}>500 Satır</option>
            </select>

            {/* Copy Logs Button */}
            <button
              onClick={handleCopyLogs}
              disabled={logsLoading || !logs}
              className="bg-gray-50 hover:bg-gray-100 border border-gray-200 text-gray-700 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-colors flex items-center gap-1.5"
            >
              {copied ? (
                <>
                  <Check className="w-3.5 h-3.5 text-emerald-500" />
                  Kopyalandı
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5 text-gray-500" />
                  Logları Kopyala
                </>
              )}
            </button>
          </div>
        </div>

        {/* Log content window */}
        <div className="relative">
          {logsLoading && (
            <div className="absolute inset-0 bg-zinc-950/40 rounded-xl backdrop-blur-[1px] flex items-center justify-center">
              <div className="flex items-center gap-2 bg-zinc-900 border border-zinc-800 text-zinc-300 px-4 py-2 rounded-xl text-xs font-semibold shadow-xl">
                <RefreshCw className="w-3.5 h-3.5 animate-spin text-blue-500" />
                Loglar Yükleniyor...
              </div>
            </div>
          )}

          <pre className="bg-zinc-950 text-zinc-300 font-mono text-xs p-5 rounded-xl overflow-x-auto max-h-[480px] border border-zinc-800 shadow-inner leading-relaxed">
            {logs}
          </pre>
        </div>
      </div>
    </div>
  )
}