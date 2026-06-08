import { useState, useEffect } from 'react'
import pb from '../lib/pocketbase'
import { Lock, CheckCircle2, AlertCircle, Key, Shield, ExternalLink, RefreshCw } from 'lucide-react'

export default function Settings() {
  const user = pb.authStore.model
  const initial = user?.email ? user.email.charAt(0).toUpperCase() : 'N'

  // Password state
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')

  // GitHub integration state
  const [githubToken, setGithubToken] = useState(user?.github_token || '')
  const [githubLoading, setGithubLoading] = useState(false)
  const [githubSuccess, setGithubSuccess] = useState('')
  const [githubError, setGithubError] = useState('')

  // GitHub App state
  const [appStatus, setAppStatus] = useState<{
    is_configured: boolean
    app_id?: string
    client_id?: string
    slug?: string
    webhook_url?: string
  } | null>(null)
  const [appLoading, setAppLoading] = useState(true)
  const [githubState, setGithubState] = useState('')

  // System Update state
  const [updateLoading, setUpdateLoading] = useState(false)
  const [updateSuccess, setUpdateSuccess] = useState('')
  const [updateError, setUpdateError] = useState('')
  const [countdown, setCountdown] = useState<number | null>(null)

  const handleSystemUpdate = async () => {
    if (!window.confirm('Nazploy\'u son sürüme güncellemek istediğinizden emin misiniz? Bu işlem sırasında kısa süreliğine panele erişilemeyebilir.')) {
      return
    }
    setUpdateLoading(true)
    setUpdateError('')
    setUpdateSuccess('')
    try {
      const res = await pb.send('/api/dashboard/system/update', { method: 'POST' })
      setUpdateSuccess(res.message || 'Güncelleme başarıyla başlatıldı!')
      setCountdown(30)
    } catch (err: any) {
      setUpdateError(err?.message || 'Güncelleme başlatılamadı.')
      setUpdateLoading(false)
    }
  }

  useEffect(() => {
    if (countdown === null) return
    if (countdown === 0) {
      window.location.reload()
      return
    }
    const timer = setTimeout(() => {
      setCountdown(countdown - 1)
    }, 1000)
    return () => clearTimeout(timer)
  }, [countdown])

  const fetchAppStatus = async () => {
    try {
      const res = await pb.send('/api/dashboard/github/app-status', { method: 'GET' })
      setAppStatus(res)
    } catch (err) {
      console.error('GitHub App status fetching failed:', err)
    } finally {
      setAppLoading(false)
    }
  }

  const fetchGithubState = async () => {
    try {
      const res = await pb.send('/api/dashboard/github/generate-state', { method: 'POST' })
      setGithubState(res.state)
    } catch (err) {
      console.error('GitHub App state generation failed:', err)
    }
  }

  useEffect(() => {
    fetchAppStatus()
    fetchGithubState()

    const params = new URLSearchParams(window.location.search)
    if (params.get('github_app_installed')) {
      setGithubSuccess('GitHub App başarıyla kuruldu ve yetkilendirildi!')
      window.history.replaceState({}, document.title, window.location.pathname)
    }
    const err = params.get('github_error')
    if (err) {
      setGithubError(`GitHub App kurulum hatası: ${err}`)
      window.history.replaceState({}, document.title, window.location.pathname)
    }
  }, [])

  const handleDisconnectApp = async () => {
    if (!window.confirm('GitHub App bağlantısını kesmek istediğinizden emin misiniz?')) return
    setGithubLoading(true)
    setGithubError('')
    setGithubSuccess('')
    try {
      await pb.send('/api/dashboard/github/disconnect', { method: 'POST' })
      setGithubSuccess('GitHub App bağlantısı başarıyla kesildi.')
      fetchAppStatus()
    } catch (err: any) {
      setGithubError(err?.message || 'Bağlantı kesilemedi.')
    } finally {
      setGithubLoading(false)
    }
  }

  // Dynamic manifest for GitHub App creation
  const origin = window.location.origin
  const hostname = window.location.hostname
  const manifest = {
    name: `Nazploy - ${hostname}`,
    url: origin,
    redirect_url: `${origin}/api/public/github/callback`,
    setup_url: `${origin}/settings?github_app_installed=true`,
    hook_attributes: {
      url: `${origin}/api/public/github/webhook`
    },
    public: false,
    default_permissions: {
      contents: "read",
      metadata: "read"
    },
    default_events: [
      "push"
    ]
  }
  const manifestString = JSON.stringify(manifest)

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')

    if (newPassword !== confirmPassword) {
      setError('Yeni şifreler eşleşmiyor.')
      return
    }

    setLoading(true)
    try {
      const recordId = user?.id
      if (!recordId) throw new Error('Kullanıcı oturum bilgisi bulunamadı.')

      await pb.collection('_superusers').update(recordId, {
        oldPassword: oldPassword,
        password: newPassword,
        passwordConfirm: confirmPassword,
      })

      setSuccess('Şifreniz başarıyla güncellendi.')
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      setError(err?.message || 'Şifre güncellenemedi. Lütfen mevcut şifrenizi kontrol edin.')
    } finally {
      setLoading(false)
    }
  }

  const handleGithubTokenChange = async (e: React.FormEvent) => {
    e.preventDefault()
    setGithubError('')
    setGithubSuccess('')

    setGithubLoading(true)
    try {
      const recordId = user?.id
      if (!recordId) throw new Error('Kullanıcı oturum bilgisi bulunamadı.')

      const updated = await pb.collection('_superusers').update(recordId, {
        github_token: githubToken.trim(),
      })

      // Sync local authStore session cache
      pb.authStore.save(pb.authStore.token, updated)

      setGithubSuccess('GitHub erişim belirteci başarıyla kaydedildi.')
    } catch (err: any) {
      setGithubError(err?.message || 'Kaydedilemedi. Lütfen yetkileri ve belirteci kontrol edin.')
    } finally {
      setGithubLoading(false)
    }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-gray-900">Ayarlar & Güvenlik</h1>
        <p className="text-sm text-gray-500 mt-1">
          Yönetici hesabı güvenlik ve entegrasyon ayarlarınızı yapılandırın.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Account Details Card */}
        <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-6">
          <div className="flex items-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-indigo-50 text-indigo-600 font-bold text-lg border border-indigo-100/55">
              {initial}
            </div>
            <div>
              <h2 className="font-bold text-gray-900">{user?.email?.split('@')[0]}</h2>
              <p className="text-xs text-gray-400 mt-0.5">Süper Kullanıcı</p>
            </div>
          </div>

          <div className="space-y-3.5 border-t border-gray-50 pt-4 text-xs text-gray-600">
            <div className="flex justify-between">
              <span className="text-gray-400">E-posta:</span>
              <span className="font-semibold text-gray-800">{user?.email}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Hesap Tipi:</span>
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-indigo-50 text-indigo-700 border border-indigo-100 uppercase">
                <Shield className="w-3 h-3" />
                Admin
              </span>
            </div>
          </div>
        </div>

        <div className="md:col-span-2 space-y-6">
          {/* Change Password Card */}
          <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-5">
            <div className="flex items-center gap-3 border-b border-gray-50 pb-4">
              <div className="p-2.5 bg-blue-50 text-blue-600 rounded-xl">
                <Key className="w-5 h-5" />
              </div>
              <div>
                <h2 className="font-bold text-gray-900">Şifre Değiştir</h2>
                <p className="text-xs text-gray-500 mt-0.5">Nazploy yönetici giriş şifresini güncelleyin</p>
              </div>
            </div>

            <form onSubmit={handlePasswordChange} className="space-y-4">
              {error && (
                <div className="bg-rose-50 text-rose-600 text-xs p-4 rounded-xl border border-rose-100 flex items-start gap-2.5">
                  <AlertCircle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
                  <span>{error}</span>
                </div>
              )}
              {success && (
                <div className="bg-emerald-50 text-emerald-600 text-xs p-4 rounded-xl border border-emerald-100 flex items-start gap-2.5">
                  <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0 mt-0.5" />
                  <span>{success}</span>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-gray-600 mb-1.5">Mevcut Şifre</label>
                <div className="relative">
                  <input
                    type="password"
                    value={oldPassword}
                    onChange={(e) => setOldPassword(e.target.value)}
                    placeholder="Mevcut şifrenizi girin"
                    className="w-full pl-9 pr-4 py-2.5 border border-gray-200 rounded-xl text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 bg-gray-50"
                    required
                  />
                  <Lock className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-gray-600 mb-1.5">Yeni Şifre</label>
                  <div className="relative">
                    <input
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      placeholder="Yeni şifre belirleyin"
                      className="w-full pl-9 pr-4 py-2.5 border border-gray-200 rounded-xl text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 bg-gray-50"
                      required
                    />
                    <Lock className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-gray-600 mb-1.5">Yeni Şifre Tekrar</label>
                  <div className="relative">
                    <input
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      placeholder="Yeni şifreyi onaylayın"
                      className="w-full pl-9 pr-4 py-2.5 border border-gray-200 rounded-xl text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 bg-gray-50"
                      required
                    />
                    <Lock className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
                  </div>
                </div>
              </div>

              <button
                type="submit"
                disabled={loading}
                className="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-xl py-2.5 px-5 text-xs font-bold transition-all shadow-sm shadow-blue-500/10 flex items-center justify-center gap-2"
              >
                <Key className="w-4 h-4" />
                {loading ? 'Şifre Güncelleniyor...' : 'Şifreyi Güncelle'}
              </button>
            </form>
          </div>

          {/* GitHub Integration Card */}
          <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-5">
            <div className="flex items-center gap-3 border-b border-gray-50 pb-4">
              <div className="p-2.5 bg-zinc-900 text-white rounded-xl">
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                </svg>
              </div>
              <div>
                <h2 className="font-bold text-gray-900">GitHub Entegrasyonu</h2>
                <p className="text-xs text-gray-500 mt-0.5">Private ve public repolarınıza şifresiz, güvenli erişim sağlayın</p>
              </div>
            </div>

            {githubError && (
              <div className="bg-rose-50 text-rose-600 text-xs p-4 rounded-xl border border-rose-100 flex items-start gap-2.5">
                <AlertCircle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
                <span>{githubError}</span>
              </div>
            )}
            {githubSuccess && (
              <div className="bg-emerald-50 text-emerald-600 text-xs p-4 rounded-xl border border-emerald-100 flex items-start gap-2.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0 mt-0.5" />
                <span>{githubSuccess}</span>
              </div>
            )}

            {/* TAB 1: GitHub App Manifest (Recommended) */}
            <div className="bg-gray-50/70 rounded-xl p-5 border border-gray-100 space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="inline-flex h-2 w-2 rounded-full bg-blue-500 animate-pulse"></span>
                  <h3 className="font-bold text-xs text-gray-800 uppercase tracking-wider">GitHub App Yöntemi (Önerilen)</h3>
                </div>
                {appStatus?.is_configured && (
                  <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-green-50 text-green-700 border border-green-100">
                    Aktif
                  </span>
                )}
              </div>

              {appLoading ? (
                <div className="text-xs text-gray-400">Yükleniyor...</div>
              ) : appStatus?.is_configured ? (
                <div className="space-y-4">
                  <p className="text-xs text-gray-600 leading-relaxed">
                    Nazploy GitHub App ile başarıyla entegre edildi. Repolarınızı güvenli bir şekilde listeleyebilir ve klonlayabilirsiniz.
                  </p>

                  <div className="grid grid-cols-2 gap-4 text-[10px] font-mono text-gray-500 border-t border-gray-200/60 pt-3">
                    <div>
                      <span className="block text-gray-400 uppercase text-[9px] mb-0.5">App ID</span>
                      <span className="font-semibold text-gray-800">{appStatus.app_id}</span>
                    </div>
                    <div>
                      <span className="block text-gray-400 uppercase text-[9px] mb-0.5">Client ID</span>
                      <span className="font-semibold text-gray-800">{appStatus.client_id}</span>
                    </div>
                    <div className="col-span-2 mt-2">
                      <span className="block text-gray-400 uppercase text-[9px] mb-0.5">Webhook URL</span>
                      <span className="font-semibold text-gray-800 select-all">{appStatus.webhook_url}</span>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 pt-2">
                    <a
                      href={`https://github.com/apps/${appStatus.slug}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 bg-white hover:bg-gray-50 border border-gray-200 text-gray-700 rounded-xl py-2 px-4 text-xs font-bold transition-all shadow-sm"
                    >
                      Uygulamayı Yönet
                      <ExternalLink className="w-3.5 h-3.5 text-gray-400" />
                    </a>
                    <button
                      type="button"
                      disabled={githubLoading}
                      onClick={handleDisconnectApp}
                      className="bg-rose-50 hover:bg-rose-100 text-rose-600 rounded-xl py-2 px-4 text-xs font-bold transition-all border border-rose-200/50"
                    >
                      Bağlantıyı Kes
                    </button>
                  </div>
                </div>
              ) : (
                <div className="space-y-4">
                  <p className="text-xs text-gray-500 leading-relaxed">
                    Tek tıkla sunucunuza özel bir GitHub App Manifest oluşturup yetkilendirin. Bu sayede manuel şifre veya PAT (Personal Access Token) girmenize gerek kalmaz, webhook'lar otomatik yapılandırılır.
                  </p>

                  {hostname === 'localhost' || hostname.match(/^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$/) ? (
                    <div className="bg-amber-50/70 border border-amber-100 text-amber-800 rounded-xl p-3.5 text-[11px] leading-relaxed flex items-start gap-2.5">
                      <AlertCircle className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
                      <div>
                        <strong>Yerel IP veya Localhost Bildirimi:</strong> Sunucunuza yerel bir IP ({hostname}) üzerinden ulaştığınız için GitHub otomatik webhook (anlık push deploy) paketlerini sunucunuza ulaştıramaz. Ancak, <strong>güvenli klonlama ve repo listeleme sorunsuz çalışacaktır.</strong> Tam otomatik push-deploy istiyorsanız ngrok veya Cloudflare Tunnels gibi bir tünel kullanabilirsiniz.
                      </div>
                    </div>
                  ) : null}

                  <form action={`https://github.com/settings/apps/new?state=${githubState}`} method="post" target="_self">
                    <input type="hidden" name="manifest" value={manifestString} />
                    <button
                      type="submit"
                      disabled={githubLoading || !githubState}
                      className="bg-blue-600 hover:bg-blue-700 text-white rounded-xl py-2.5 px-5 text-xs font-bold transition-all shadow-sm shadow-blue-500/10 flex items-center gap-2"
                    >
                      <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                      </svg>
                      GitHub App Entegrasyonunu Başlat
                    </button>
                  </form>
                </div>
              )}
            </div>

            {/* TAB 2: PAT Token (Alternative) */}
            <div className="bg-white rounded-xl p-5 border border-gray-100 space-y-4">
              <div className="flex items-center gap-2">
                <span className="inline-flex h-2 w-2 rounded-full bg-gray-400"></span>
                <h3 className="font-bold text-xs text-gray-500 uppercase tracking-wider">Alternatif Yöntem: Personal Access Token (PAT)</h3>
              </div>

              {!appStatus?.is_configured ? (
                <form onSubmit={handleGithubTokenChange} className="space-y-4">
                  <div>
                    <label className="block text-xs font-semibold text-gray-600 mb-1.5">GitHub Personal Access Token (PAT)</label>
                    <input
                      type="password"
                      value={githubToken}
                      onChange={(e) => setGithubToken(e.target.value)}
                      placeholder="ghp_..."
                      className="w-full px-3.5 py-2.5 border border-gray-200 rounded-xl text-xs focus:outline-none focus:ring-1 focus:ring-blue-500 bg-gray-50 font-mono"
                    />
                    <p className="text-[10px] text-gray-400 mt-1.5 leading-relaxed font-sans">
                      Alternatif olarak klasik PAT ekleyebilirsiniz. Gerekli yetkiler: <strong>repo (classic)</strong> veya <strong>Metadata: Read-only, Contents: Read-only (fine-grained)</strong>.
                    </p>
                  </div>

                  <button
                    type="submit"
                    disabled={githubLoading}
                    className="bg-zinc-900 hover:bg-zinc-800 disabled:opacity-50 text-white rounded-xl py-2.5 px-5 text-xs font-bold transition-all shadow-sm flex items-center justify-center gap-2"
                  >
                    {githubLoading ? 'Kaydediliyor...' : 'Manuel Belirteci Kaydet'}
                  </button>
                </form>
              ) : (
                <p className="text-xs text-gray-400">
                  GitHub App aktif olduğu için PAT kullanımı devredışıdır. App bağlantısını keserek tekrar PAT yöntemine dönebilirsiniz.
                </p>
              )}
            </div>
          </div>

          {/* System Update Card */}
          <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-5">
            <div className="flex items-center gap-3 border-b border-gray-50 pb-4">
              <div className="p-2.5 bg-indigo-50 text-indigo-600 rounded-xl">
                <RefreshCw className={`w-5 h-5 ${updateLoading ? 'animate-spin' : ''}`} />
              </div>
              <div>
                <h2 className="font-bold text-gray-900">Sistem Güncellemesi</h2>
                <p className="text-xs text-gray-500 mt-0.5">Nazploy panelini GitHub üzerindeki son sürüme güncelleyin</p>
              </div>
            </div>

            {updateError && (
              <div className="bg-rose-50 text-rose-600 text-xs p-4 rounded-xl border border-rose-100 flex items-start gap-2.5">
                <AlertCircle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
                <span>{updateError}</span>
              </div>
            )}
            {updateSuccess && (
              <div className="bg-emerald-50 text-emerald-600 text-xs p-4 rounded-xl border border-emerald-100 flex items-start gap-2.5">
                <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0 mt-0.5" />
                <div>
                  <span className="font-semibold block">{updateSuccess}</span>
                  {countdown !== null && (
                    <span className="text-[11px] text-emerald-700 mt-1 block">
                      Sunucu derlenip yeniden başlatılıyor. Sayfa {countdown} saniye içinde otomatik yenilenecek...
                    </span>
                  )}
                </div>
              </div>
            )}

            <div className="space-y-4">
              <p className="text-xs text-gray-500 leading-relaxed">
                Bu buton sunucuda arka planda <code>setup.sh</code> betiğini çalıştırır. Sunucudaki kodlar güncellenir, frontend ve backend baştan derlenir ve Nazploy servisi yeniden başlatılır.
              </p>
              <button
                type="button"
                disabled={updateLoading}
                onClick={handleSystemUpdate}
                className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white rounded-xl py-2.5 px-5 text-xs font-bold transition-all shadow-sm shadow-indigo-500/10 flex items-center justify-center gap-2"
              >
                <RefreshCw className={`w-4 h-4 ${updateLoading ? 'animate-spin' : ''}`} />
                {updateLoading ? 'Güncelleniyor...' : 'Şimdi Güncelle'}
              </button>
            </div>
          </div>

        </div>
      </div>
    </div>
  )
}
