import { useEffect, useState } from 'react'
import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom'
import pb from './lib/pocketbase'
import LoginPage from './pages/LoginPage'
import DashboardOverview from './pages/DashboardOverview'
import SitesList from './pages/SitesList'
import SiteDetail from './pages/SiteDetail'
import SiteForm from './pages/SiteForm'
import Sidebar from './components/Sidebar'
import NginxStatus from './pages/NginxStatus'
import Settings from './pages/Settings'

function App() {
  const [authenticated, setAuthenticated] = useState(pb.authStore.isValid)
  const navigate = useNavigate()
  const location = useLocation()

  useEffect(() => {
    const unsubscribe = pb.authStore.onChange((token) => {
      setAuthenticated(!!token)
      if (!token) navigate('/login')
    })
    return unsubscribe
  }, [navigate])

  if (!authenticated) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage onLogin={() => setAuthenticated(true)} />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-y-auto bg-gray-50">
        <div className="p-6 max-w-7xl mx-auto">
          <Routes>
            <Route path="/" element={<DashboardOverview />} />
            <Route path="/sites" element={<SitesList />} />
            <Route path="/sites/new" element={<SiteForm />} />
            <Route path="/sites/:id" element={<SiteDetail />} />
            <Route path="/sites/:id/edit" element={<SiteForm />} />
            <Route path="/nginx" element={<NginxStatus />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </div>
      </main>
    </div>
  )
}

export default App
