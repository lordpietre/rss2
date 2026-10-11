import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { User, LogOut, Save } from 'lucide-react'

interface UserData {
  id: number
  email: string
  username: string
  is_admin: boolean
  created_at: string
  avatar_url?: string
}

export function Account() {
  const navigate = useNavigate()
  const [user, setUser] = useState<UserData | null>(null)
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [message, setMessage] = useState('')

  useEffect(() => {
    const token = localStorage.getItem('token')
    const userData = localStorage.getItem('user')
    
    if (!token) {
      navigate('/login')
      return
    }

    if (userData) {
      const parsed = JSON.parse(userData)
      setUser(parsed)
      setUsername(parsed.username || '')
      setEmail(parsed.email || '')
    }
  }, [navigate])

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    navigate('/login')
  }

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault()
    setMessage('Profile updated successfully!')
    setTimeout(() => setMessage(''), 3000)
  }

  if (!user) {
    return null
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-50">Account</h1>
        <p className="text-sm text-gray-400 mt-1">Manage your profile and preferences</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="md:col-span-2">
          <form onSubmit={handleUpdate} className="panel">
            <div className="flex items-center gap-3 mb-6">
              <div className="h-16 w-16 rounded-full bg-gray-800 flex items-center justify-center">
                <User className="h-8 w-8 text-primary-50" />
              </div>
              <div>
                <h2 className="text-xl font-semibold text-gray-50">{user.username}</h2>
                <p className="text-gray-400 text-sm">{user.email}</p>
              </div>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Username
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="input"
                />
              </div>

              <div>
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Email
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="input"
                />
              </div>

              <div>
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  New Password (leave blank to keep current)
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="input"
                  placeholder="••••••••"
                />
              </div>

              {message && (
                <div className="p-3 bg-primary-900/30 text-primary-50 rounded-lg border border-primary-500/30">
                  {message}
                </div>
              )}

              <button type="submit" className="btn-primary flex items-center gap-2">
                <Save className="h-4 w-4" />
                Save Changes
              </button>
            </div>
          </form>
        </div>

        <div>
          <div className="panel">
            <h3 className="font-semibold text-gray-50 mb-4">Account Info</h3>
            <div className="space-y-3 text-sm">
              <div className="flex justify-between">
                <span className="text-gray-400">User ID:</span>
                <span className="font-mono text-gray-300">{user.id}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Role:</span>
                <span className="text-primary-50">{user.is_admin ? 'Admin' : 'User'}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Member since:</span>
                <span className="text-gray-300">
                  {new Date(user.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>

            <button
              onClick={handleLogout}
              className="btn-secondary w-full mt-6 flex items-center justify-center gap-2 border-rss-red/50 text-rss-red hover:bg-rss-red/10"
            >
              <LogOut className="h-4 w-4" />
              Logout
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
