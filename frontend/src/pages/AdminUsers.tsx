import { useState, useEffect } from 'react'
import { api } from '../services/api'
import { Users, Shield, ShieldOff, Crown } from 'lucide-react'

interface User {
  id: number
  email: string
  username: string
  is_admin: boolean
  created_at: string
}

export function AdminUsers() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [currentUserId, setCurrentUserId] = useState<number | null>(null)

  useEffect(() => {
    const userData = localStorage.getItem('user')
    if (userData) {
      try {
        const user = JSON.parse(userData)
        setCurrentUserId(user.id)
      } catch {}
    }
    fetchUsers()
  }, [])

  const fetchUsers = async () => {
    setLoading(true)
    try {
      const res = await api.get('/admin/users')
      setUsers(res.data.users)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const handlePromote = async (userId: number) => {
    if (!confirm('¿Promover este usuario a administrador?')) return
    try {
      await api.post(`/admin/users/${userId}/promote`)
      fetchUsers()
    } catch (err) {
      console.error(err)
      alert('Error al promover usuario')
    }
  }

  const handleDemote = async (userId: number) => {
    if (!confirm('¿Quitar permisos de administrador a este usuario?')) return
    try {
      await api.post(`/admin/users/${userId}/demote`)
      fetchUsers()
    } catch (err) {
      console.error(err)
      alert('Error al quitar permisos')
    }
  }

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString('es', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    })
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <Users className="h-6 w-6" />
          Gestión de Usuarios
        </h1>
      </div>

      {loading ? (
        <div className="text-center py-8">Cargando...</div>
      ) : users.length === 0 ? (
        <div className="text-center py-8 text-gray-500">No hay usuarios</div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left">Usuario</th>
                <th className="px-4 py-3 text-left">Email</th>
                <th className="px-4 py-3 text-left">Rol</th>
                <th className="px-4 py-3 text-left">Fecha Registro</th>
                <th className="px-4 py-3 text-right">Acciones</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} className="border-t dark:border-gray-700">
                  <td className="px-4 py-3 font-medium">
                    {user.username}
                    {user.id === currentUserId && (
                      <span className="ml-2 text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded">Tú</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-gray-500">{user.email}</td>
                  <td className="px-4 py-3">
                    {user.is_admin ? (
                      <span className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs bg-purple-100 text-purple-800">
                        <Crown className="h-3 w-3" />
                        Admin
                      </span>
                    ) : (
                      <span className="px-2 py-1 rounded text-xs bg-gray-100 text-gray-800">
                        Usuario
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-gray-500 text-sm">
                    {formatDate(user.created_at)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {user.id !== currentUserId && (
                      user.is_admin ? (
                        <button
                          onClick={() => handleDemote(user.id)}
                          className="text-red-600 hover:text-red-800 text-sm flex items-center gap-1 ml-auto"
                        >
                          <ShieldOff className="h-4 w-4" />
                          Quitar Admin
                        </button>
                      ) : (
                        <button
                          onClick={() => handlePromote(user.id)}
                          className="text-green-600 hover:text-green-800 text-sm flex items-center gap-1 ml-auto"
                        >
                          <Shield className="h-4 w-4" />
                          Hacer Admin
                        </button>
                      )
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
