/**
 * LoginPage - Autenticación con soporte para Cloud y Local
 */

import React, { useState } from 'react'
import { useApp } from '../contexts/AppContext'
import { Cloud, Smartphone, Loader, AlertCircle } from 'lucide-react'
import { Link } from 'react-router-dom'

export function LoginPage() {
  const { 
    serverMode, 
    setMode,
    login, 
    register, 
    isLoading, 
    error, 
    clearError,
    apiUrl,
    subscription
  } = useApp()

  const [isRegister, setIsRegister] = useState(false)
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLocalError(null)
    clearError()

    try {
      if (isRegister) {
        await register(email, username, password)
      } else {
        await login(email, password)
      }
      // Redirect to home on success
      window.location.href = '/'
    } catch (err: any) {
      setLocalError(err.message || 'Error de conexión')
    }
  }

  const switchMode = (mode: 'cloud' | 'local') => {
    setMode(mode)
    setLocalError(null)
    clearError()
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col justify-center py-12 px-4 sm:px-6 lg:px-8">
      {/* Server Mode Toggle */}
      <div className="sm:mx-auto sm:w-full sm:max-w-md mb-8">
        <div className="flex rounded-lg bg-gray-200 p-1">
          <button
            onClick={() => switchMode('cloud')}
            className={`flex-1 flex items-center justify-center py-2 px-4 rounded-md text-sm font-medium transition-colors ${
              serverMode === 'cloud'
                ? 'bg-white text-indigo-600 shadow'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            <Cloud className="w-4 h-4 mr-2" />
            Cloud
          </button>
          <button
            onClick={() => switchMode('local')}
            className={`flex-1 flex items-center justify-center py-2 px-4 rounded-md text-sm font-medium transition-colors ${
              serverMode === 'local'
                ? 'bg-white text-green-600 shadow'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            <Smartphone className="w-4 h-4 mr-2" />
            Local
          </button>
        </div>
        <p className="text-center text-xs text-gray-500 mt-2">
          Conectando a: {apiUrl}
        </p>
      </div>

      {/* Form Container */}
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <div className="bg-white py-8 px-4 shadow sm:rounded-lg sm:px-10">
          {/* Header */}
          <div className="mb-6 text-center">
            <h2 className="text-2xl font-bold text-gray-900">
              {isRegister ? 'Crear Cuenta' : 'Iniciar Sesión'}
            </h2>
            <p className="text-gray-600 text-sm mt-1">
              {serverMode === 'cloud' 
                ? subscription?.needs_upgrade 
                  ? 'Upgrade tu plan para desbloquear funciones'
                  : `Plan ${subscription?.plan || 'free'} - ${subscription?.status || 'inactive'}`
                : 'Modo Local'}
            </p>
          </div>

          {/* Error Alert */}
          {(localError || error) && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg flex items-center text-red-700 text-sm">
              <AlertCircle className="w-5 h-5 mr-2 flex-shrink-0" />
              {localError || error}
            </div>
          )}

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            {isRegister && (
              <div>
                <label className="block text-sm font-medium text-gray-700">
                  Usuario
                </label>
                <input
                  type="text"
                  required
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500"
                  placeholder="tu_nombre"
                />
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-gray-700">
                Email
              </label>
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500"
                placeholder="tu@email.com"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700">
                Contraseña
              </label>
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500"
                placeholder="••••••••"
                minLength={6}
              />
            </div>

            <button
              type="submit"
              disabled={isLoading}
              className="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
            >
              {isLoading ? (
                <Loader className="w-5 h-5 animate-spin" />
              ) : isRegister ? (
                'Crear Cuenta'
              ) : (
                'Iniciar Sesión'
              )}
            </button>
          </form>

          {/* Toggle Register/Login */}
          <div className="mt-6 text-center">
            <button
              onClick={() => {
                setIsRegister(!isRegister)
                setLocalError(null)
                clearError()
              }}
              className="text-sm text-indigo-600 hover:text-indigo-500"
            >
              {isRegister 
                ? '¿Ya tienes cuenta? Inicia sesión' 
                : '¿No tienes cuenta? Regístrate'}
            </button>
          </div>

          {/* Back to mode selection */}
          <div className="mt-4 text-center">
            <Link to="/onboarding" className="text-sm text-gray-500 hover:text-gray-700">
              ← Cambiar modo de conexión
            </Link>
          </div>
        </div>

        {/* Subscription Info for Cloud Mode */}
        {serverMode === 'cloud' && !isRegister && (
          <div className="mt-6 bg-indigo-50 rounded-lg p-4 text-center">
            <p className="text-sm text-indigo-700">
              ¿No tienes cuenta? Regístrate gratis con 5 fuentes.
            </p>
            <p className="text-xs text-indigo-600 mt-1">
              Upgrade después para desbloquear más funciones.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
