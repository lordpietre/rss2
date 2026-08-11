import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiService, api } from '../services/api'

export function Login() {
  const navigate = useNavigate()
  const [isLogin, setIsLogin] = useState(true)
  const [form, setForm] = useState({ email: '', password: '', username: '' })
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [successMessage, setSuccessMessage] = useState('')
  const [sessionExpired] = useState<boolean>(
    () => new URLSearchParams(window.location.search).get('expired') === '1',
  )

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccessMessage('')
    
    if (!isLogin && form.password !== confirmPassword) {
      setError('Las contraseñas no coinciden')
      return
    }
    
    try {
      if (isLogin) {
        const { token, user } = await apiService.login(form.email, form.password)
        localStorage.setItem('token', token)
        localStorage.setItem('user', JSON.stringify(user))
      } else {
        const result = await apiService.register(form.email, form.username, form.password)
        localStorage.setItem('token', result.token)
        localStorage.setItem('user', JSON.stringify(result.user))
        
        if (result.is_first_user) {
          setSuccessMessage('🎉 ¡Felicidades! Has sido registrado como ADMINISTRADOR del sistema.')
          setTimeout(() => navigate('/'), 3000)
          return
        }
      }
      navigate('/')
    } catch (err: any) {
      setError(err.response?.data?.error || 'Error al iniciar sesión')
    }
  }

  return (
    <div className="max-w-md mx-auto">
      <div className="card p-8">
        <h1 className="text-2xl font-bold text-center mb-2">
          {isLogin ? 'Iniciar Sesión' : 'Registrarse'}
        </h1>
        
        {successMessage && (
          <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4 text-center">
            {successMessage}
          </div>
        )}
        
        {sessionExpired && !successMessage && (
          <div className="bg-yellow-100 border border-yellow-400 text-yellow-700 px-4 py-3 rounded mb-4 text-center">
            Tu sesión ha expirado. Inicia sesión de nuevo.
          </div>
        )}
        
        <form onSubmit={handleSubmit} className="space-y-4">
          {!isLogin && (
            <>
              <input
                type="text"
                placeholder="Nombre de usuario"
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                className="input"
                required
              />
            </>
          )}
          <input
            type="email"
            placeholder="Email"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            className="input"
            required
          />
          <input
            type="password"
            placeholder="Contraseña"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
            className="input"
            required
          />
          
          {!isLogin && (
            <input
              type="password"
              placeholder="Confirmar contraseña"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="input"
              required
            />
          )}
          
          {error && <p className="text-red-600 text-sm">{error}</p>}
          
          <button type="submit" className="btn-primary w-full">
            {isLogin ? 'Iniciar Sesión' : 'Registrarse'}
          </button>
        </form>
        
        <p className="text-center mt-4 text-gray-600">
          {isLogin ? '¿No tienes cuenta? ' : '¿Ya tienes cuenta? '}
          <button onClick={() => { setIsLogin(!isLogin); setError(''); setConfirmPassword(''); }} className="text-primary-600 hover:underline">
            {isLogin ? 'Regístrate' : 'Inicia sesión'}
          </button>
        </p>
      </div>
    </div>
  )
}
