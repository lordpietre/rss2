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
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">Account</h1>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        <div className="md:col-span-2">
          <form onSubmit={handleUpdate} className="card p-6">
            <div className="flex items-center gap-3 mb-6">
              <div className="h-16 w-16 rounded-full bg-primary-100 flex items-center justify-center">
                <User className="h-8 w-8 text-primary-600" />
              </div>
              <div>
                <h2 className="text-xl font-semibold">{user.username}</h2>
                <p className="text-gray-500 text-sm">{user.email}</p>
              </div>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
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
                <label className="block text-sm font-medium text-gray-700 mb-1">
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
                <label className="block text-sm font-medium text-gray-700 mb-1">
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
                <div className="p-3 bg-green-50 text-green-700 rounded-lg">
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
          <div className="card p-6">
            <h3 className="font-semibold mb-4">Account Info</h3>
            <div className="space-y-3 text-sm">
              <div className="flex justify-between">
                <span className="text-gray-500">User ID:</span>
                <span className="font-medium">{user.id}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Role:</span>
                <span className="font-medium">{user.is_admin ? 'Admin' : 'User'}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Member since:</span>
                <span className="font-medium">
                  {new Date(user.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>

            <button
              onClick={handleLogout}
              className="btn-danger w-full mt-6 flex items-center justify-center gap-2"
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
