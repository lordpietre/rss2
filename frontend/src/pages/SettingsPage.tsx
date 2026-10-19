/**
 * SettingsPage - Configuración de servidor y cuenta
 */

import React, { useState, useEffect } from 'react'
import { useApp } from '../contexts/AppContext'
import { 
  Cloud, Smartphone, Server, User, CreditCard, CheckCircle, 
  XCircle, Loader, ChevronRight, LogOut 
} from 'lucide-react'
import { Link } from 'react-router-dom'

export function SettingsPage() {
  const { 
    serverMode, 
    setMode,
    apiUrl,
    isLocalServerAvailable,
    isCheckingServer,
    checkLocalServer,
    updateLocalServerUrl,
    user,
    subscription,
    usage,
    isAuthenticated,
    logout
  } = useApp()

  const [localUrl, setLocalUrl] = useState('http://localhost')
  const [localPort, setLocalPort] = useState(8080)
  const [showUpgradeModal, setShowUpgradeModal] = useState(false)

  useEffect(() => {
    if (serverMode === 'local') {
      checkLocalServer()
    }
  }, [])

  const handleLocalUrlSave = () => {
    updateLocalServerUrl(localUrl, localPort)
  }

  const handleLogout = () => {
    logout()
    window.location.href = '/login'
  }

  const getPlanBadgeColor = (plan: string) => {
    switch (plan) {
      case 'premium': return 'bg-purple-100 text-purple-800'
      case 'pro': return 'bg-blue-100 text-blue-800'
      default: return 'bg-gray-100 text-gray-800'
    }
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white shadow-sm">
        <div className="max-w-2xl mx-auto px-4 py-4">
          <h1 className="text-xl font-bold text-gray-900">Configuración</h1>
        </div>
      </div>

      <div className="max-w-2xl mx-auto px-4 py-6 space-y-6">
        
        {/* Server Mode Section */}
        <section className="bg-white rounded-xl shadow-sm overflow-hidden">
          <div className="px-4 py-3 bg-gray-50 border-b border-gray-200">
            <h2 className="font-semibold text-gray-900 flex items-center">
              <Server className="w-5 h-5 mr-2" />
              Servidor
            </h2>
          </div>
          
          <div className="p-4">
            {/* Mode Toggle */}
            <div className="flex rounded-lg bg-gray-100 p-1 mb-4">
              <button
                onClick={() => setMode('cloud')}
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
                onClick={() => setMode('local')}
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

            {/* Current Server Info */}
            <div className="text-sm text-gray-600 mb-4">
              Conectado a: <span className="font-mono text-gray-900">{apiUrl}</span>
            </div>

            {/* Local Server Config */}
            {serverMode === 'local' && (
              <div className="border-t border-gray-200 pt-4 mt-4">
                <div className="flex items-center justify-between mb-3">
                  <span className="text-sm font-medium text-gray-700">
                    Estado del servidor
                  </span>
                  {isCheckingServer ? (
                    <Loader className="w-4 h-4 text-blue-500 animate-spin" />
                  ) : isLocalServerAvailable ? (
                    <span className="flex items-center text-green-600 text-sm">
                      <CheckCircle className="w-4 h-4 mr-1" />
                      Activo
                    </span>
                  ) : (
                    <span className="flex items-center text-red-600 text-sm">
                      <XCircle className="w-4 h-4 mr-1" />
                      No disponible
                    </span>
                  )}
                </div>

                <div className="grid grid-cols-3 gap-3">
                  <div className="col-span-2">
                    <label className="block text-xs font-medium text-gray-500 mb-1">
                      URL del servidor
                    </label>
                    <input
                      type="text"
                      value={localUrl}
                      onChange={(e) => setLocalUrl(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-gray-500 mb-1">
                      Puerto
                    </label>
                    <input
                      type="number"
                      value={localPort}
                      onChange={(e) => setLocalPort(parseInt(e.target.value) || 8080)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                    />
                  </div>
                </div>
                <button
                  onClick={handleLocalUrlSave}
                  className="mt-3 w-full py-2 bg-gray-100 text-gray-700 rounded-lg text-sm hover:bg-gray-200"
                >
                  Actualizar conexión
                </button>
              </div>
            )}
          </div>
        </section>

        {/* Account Section */}
        {isAuthenticated && (
          <section className="bg-white rounded-xl shadow-sm overflow-hidden">
            <div className="px-4 py-3 bg-gray-50 border-b border-gray-200">
              <h2 className="font-semibold text-gray-900 flex items-center">
                <User className="w-5 h-5 mr-2" />
                Cuenta
              </h2>
            </div>
            
            <div className="p-4">
              <div className="flex items-center justify-between mb-4">
                <div>
                  <p className="font-medium text-gray-900">{user?.username}</p>
                  <p className="text-sm text-gray-500">{user?.email}</p>
                </div>
                <span className={`px-2 py-1 rounded-full text-xs font-medium ${getPlanBadgeColor(user?.plan || 'free')}`}>
                  {user?.plan || 'free'}
                </span>
              </div>

              {/* Subscription Info (Cloud only) */}
              {serverMode === 'cloud' && subscription && (
                <div className="border-t border-gray-200 pt-4 mt-4">
                  <h3 className="text-sm font-medium text-gray-700 mb-3 flex items-center">
                    <CreditCard className="w-4 h-4 mr-2" />
                    Suscripción
                  </h3>
                  
                  <div className="bg-gray-50 rounded-lg p-3 mb-3">
                    <div className="flex justify-between items-center mb-2">
                      <span className="text-sm text-gray-600">Plan</span>
                      <span className="font-medium text-gray-900 capitalize">{subscription.plan}</span>
                    </div>
                    <div className="flex justify-between items-center mb-2">
                      <span className="text-sm text-gray-600">Estado</span>
                      <span className={`text-sm font-medium ${
                        subscription.status === 'active' ? 'text-green-600' : 'text-gray-600'
                      }`}>
                        {subscription.status}
                      </span>
                    </div>
                    {subscription.ends_at && (
                      <div className="flex justify-between items-center">
                        <span className="text-sm text-gray-600">Renovación</span>
                        <span className="text-sm text-gray-900">
                          {new Date(subscription.ends_at).toLocaleDateString()}
                        </span>
                      </div>
                    )}
                  </div>

                  {/* Usage Stats */}
                  {usage && (
                    <div className="space-y-2 mb-3">
                      <div className="flex justify-between text-sm">
                        <span className="text-gray-600">Feeds usados</span>
                        <span className="text-gray-900">
                          {usage.usage.feeds_used} / {usage.limits.feeds === -1 ? '∞' : usage.limits.feeds}
                        </span>
                      </div>
                      <div className="flex justify-between text-sm">
                        <span className="text-gray-600">Traducciones hoy</span>
                        <span className="text-gray-900">
                          {usage.usage.translations_today} / {usage.limits.translations_per_day === -1 ? '∞' : usage.limits.translations_per_day}
                        </span>
                      </div>
                      <div className="flex justify-between text-sm">
                        <span className="text-gray-600">Búsqueda</span>
                        <span className={usage.limits.search_enabled ? 'text-green-600' : 'text-red-600'}>
                          {usage.limits.search_enabled ? 'Disponible' : 'No disponible'}
                        </span>
                      </div>
                    </div>
                  )}

                  {subscription.needs_upgrade && (
                    <button
                      onClick={() => setShowUpgradeModal(true)}
                      className="w-full py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700"
                    >
                      Mejorar a Pro
                    </button>
                  )}
                </div>
              )}

              <button
                onClick={handleLogout}
                className="w-full mt-4 py-2 border border-red-300 text-red-600 rounded-lg text-sm font-medium hover:bg-red-50 flex items-center justify-center"
              >
                <LogOut className="w-4 h-4 mr-2" />
                Cerrar Sesión
              </button>
            </div>
          </section>
        )}

        {/* App Info */}
        <section className="bg-white rounded-xl shadow-sm p-4">
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">Versión de la App</span>
            <span className="text-gray-900">1.0.0</span>
          </div>
        </section>
      </div>

      {/* Upgrade Modal */}
      {showUpgradeModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl max-w-sm w-full p-6">
            <h3 className="text-lg font-bold text-gray-900 mb-2">
              Mejorar tu Plan
            </h3>
            <p className="text-gray-600 text-sm mb-4">
              Desbloquea más fuentes, traducciones y funciones premium.
            </p>
            
            <div className="space-y-3 mb-6">
              <div className="flex items-center">
                <CheckCircle className="w-5 h-5 text-green-500 mr-3" />
                <span className="text-sm text-gray-700">50 fuentes RSS</span>
              </div>
              <div className="flex items-center">
                <CheckCircle className="w-5 h-5 text-green-500 mr-3" />
                <span className="text-sm text-gray-700">100 traducciones/día</span>
              </div>
              <div className="flex items-center">
                <CheckCircle className="w-5 h-5 text-green-500 mr-3" />
                <span className="text-sm text-gray-700">Búsqueda semántica</span>
              </div>
              <div className="flex items-center">
                <CheckCircle className="w-5 h-5 text-green-500 mr-3" />
                <span className="text-sm text-gray-700">10 alertas</span>
              </div>
            </div>

            <div className="text-center mb-4">
              <span className="text-3xl font-bold text-gray-900">$4.99</span>
              <span className="text-gray-500">/mes</span>
            </div>

            <button
              onClick={() => setShowUpgradeModal(false)}
              className="w-full py-3 bg-indigo-600 text-white rounded-xl font-medium hover:bg-indigo-700 mb-3"
            >
              Suscribirse
            </button>
            <button
              onClick={() => setShowUpgradeModal(false)}
              className="w-full py-2 text-gray-600 text-sm hover:text-gray-800"
            >
              Quizás más tarde
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
