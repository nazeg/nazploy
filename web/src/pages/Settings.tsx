import { useState } from 'react'
import pb from '../lib/pocketbase'
import { Lock, CheckCircle2, AlertCircle, Key, Shield } from 'lucide-react'

export default function Settings() {
  const user = pb.authStore.model
  const initial = user?.email ? user.email.charAt(0).toUpperCase() : 'N'

  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')

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

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-gray-900">Ayarlar & Güvenlik</h1>
        <p className="text-sm text-gray-500 mt-1">
          Yönetici hesabı güvenlik ayarlarınızı yapılandırın.
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

        {/* Change Password Card */}
        <div className="md:col-span-2 bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-5">
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
      </div>
    </div>
  )
}
