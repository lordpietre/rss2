/**
 * OnboardingPage - Selección de modo de conexión
 * 
 * Permite al usuario elegir entre:
 * - Modo Cloud: Conectar al servidor central (suscripción)
 * - Modo Local: Desplegar servidor en el teléfono
 */

import React, { useState, useEffect } from 'react'
import { useApp } from '../contexts/AppContext'
import { Cloud, Smartphone, Server, CheckCircle, XCircle, Loader } from 'lucide-react'

export function OnboardingPage() {
  const { 
    serverMode, 
    setMode, 
    isLocalServerAvailable, 
    isCheckingServer,
    checkLocalServer,
    apiUrl 
  } = useApp()

  const [showAdvanced, setShowAdvanced] = useState(false)
  const [localUrl, setLocalUrl] = useState('http://localhost')
  const [localPort, setLocalPort] = useState(8080)

  // Verificar servidor local al montar
  useEffect(() => {
    checkLocalServer()
  }, [])

  const handleModeSelect = (mode: 'cloud' | 'local') => {
    setMode(mode)
  }

  const handleLocalUrlSave = () => {
    // Guardar URL personalizada y re-checkear
    setLocalUrl(localUrl)
    checkLocalServer()
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex items-center justify-center p-4">
      <div className="max-w-2xl w-full">
        {/* Header */}
        <div className="text-center mb-8">
          <h1 className="text-4xl font-bold text-gray-900 mb-2">RSS2</h1>
          <p className="text-gray-600">Tu agregador de noticias inteligente</p>
        </div>

        {/* Mode Selection Cards */}
        <div className="grid md:grid-cols-2 gap-6 mb-8">
          {/* Cloud Mode */}
          <button
            onClick={() => handleModeSelect('cloud')}
            className={`p-6 rounded-2xl border-2 transition-all text-left ${
              serverMode === 'cloud'
                ? 'border-indigo-500 bg-indigo-50 shadow-lg'
                : 'border-gray-200 bg-white hover:border-indigo-300'
            }`}
          >
            <div className="flex items-center mb-4">
              <div className={`p-3 rounded-xl ${
                serverMode === 'cloud' ? 'bg-indigo-500 text-white' : 'bg-gray-100 text-gray-600'
              }`}>
                <Cloud className="w-8 h-8" />
              </div>
              {serverMode === 'cloud' && (
                <CheckCircle className="w-6 h-6 text-indigo-500 ml-auto" />
              )}
            </div>
            <h2 className="text-xl font-semibold text-gray-900 mb-2">Modo Cloud</h2>
            <p className="text-gray-600 text-sm mb-4">
              Conecta a nuestro servidor central. Ideal para acceso desde cualquier lugar.
            </p>
            <div className="space-y-2 text-sm">
              <div className="flex items-center text-gray-500">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2" />
                Sincronización automática
              </div>
              <div className="flex items-center text-gray-500">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2" />
                Suscripción premium
              </div>
              <div className="flex items-center text-gray-500">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2" />
                Traducción automática
              </div>
            </div>
          </button>

          {/* Local Mode */}
          <button
            onClick={() => handleModeSelect('local')}
            className={`p-6 rounded-2xl border-2 transition-all text-left ${
              serverMode === 'local'
                ? 'border-green-500 bg-green-50 shadow-lg'
                : 'border-gray-200 bg-white hover:border-green-300'
            }`}
          >
            <div className="flex items-center mb-4">
              <div className={`p-3 rounded-xl ${
                serverMode === 'local' ? 'bg-green-500 text-white' : 'bg-gray-100 text-gray-600'
              }`}>
                <Smartphone className="w-8 h-8" />
              </div>
              {serverMode === 'local' && (
                <CheckCircle className="w-6 h-6 text-green-500 ml-auto" />
              )}
            </div>
            <h2 className="text-xl font-semibold text-gray-900 mb-2">Modo Local</h2>
            <p className="text-gray-600 text-sm mb-4">
              Despliega el servidor en tu teléfono. Tus datos permanecen en tu dispositivo.
            </p>
            <div className="space-y-2 text-sm">
              <div className="flex items-center text-gray-500">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2" />
                100% privado
              </div>
              <div className="flex items-center text-gray-500">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2" />
                Sin internet requerida
              </div>
              <div className="flex items-center text-gray-500">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2" />
                Compra única
              </div>
            </div>
          </button>
        </div>

        {/* Server Status for Local Mode */}
        {serverMode === 'local' && (
          <div className="bg-white rounded-xl p-6 border border-gray-200 mb-6">
            <div className="flex items-center mb-4">
              <Server className="w-5 h-5 text-gray-600 mr-2" />
              <h3 className="font-semibold text-gray-900">Estado del Servidor Local</h3>
            </div>
            
            <div className="flex items-center mb-4">
              {isCheckingServer ? (
                <>
                  <Loader className="w-5 h-5 text-blue-500 animate-spin mr-2" />
                  <span className="text-gray-600">Verificando...</span>
                </>
              ) : isLocalServerAvailable ? (
                <>
                  <CheckCircle className="w-5 h-5 text-green-500 mr-2" />
                  <span className="text-green-600 font-medium">Servidor detectado en {apiUrl}</span>
                </>
              ) : (
                <>
                  <XCircle className="w-5 h-5 text-red-500 mr-2" />
                  <span className="text-red-600">Servidor no encontrado</span>
                </>
              )}
            </div>

            <button
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="text-sm text-indigo-600 hover:text-indigo-800"
            >
              {showAdvanced ? 'Ocultar' : 'Configurar'} servidor manual
            </button>

            {showAdvanced && (
              <div className="mt-4 pt-4 border-t border-gray-200">
                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      URL
                    </label>
                    <input
                      type="text"
                      value={localUrl}
                      onChange={(e) => setLocalUrl(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                      placeholder="http://localhost"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Puerto
                    </label>
                    <input
                      type="number"
                      value={localPort}
                      onChange={(e) => setLocalPort(parseInt(e.target.value) || 8080)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                      placeholder="8080"
                    />
                  </div>
                  <div className="flex items-end">
                    <button
                      onClick={handleLocalUrlSave}
                      className="w-full px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm hover:bg-indigo-700"
                    >
                      Verificar
                    </button>
                  </div>
                </div>
                <p className="mt-2 text-xs text-gray-500">
                  Asegúrate de que el servidor RSS2 esté ejecutándose en tu teléfono.
                </p>
              </div>
            )}
          </div>
        )}

        {/* Continue Button */}
        <button
          onClick={() => {
            // Navegar a login (aquí sería con router, por ahora solo cambiamos el modo)
            window.location.href = '/login'
          }}
          className="w-full py-4 bg-indigo-600 text-white font-semibold rounded-xl hover:bg-indigo-700 transition-colors"
        >
          Continuar
        </button>

        {/* Info Footer */}
        <p className="text-center text-gray-500 text-sm mt-6">
          {serverMode === 'cloud' 
            ? 'El modo cloud requiere una suscripción activa.'
            : 'El modo local requiere tener RSS2 Server instalado en tu dispositivo.'}
        </p>
      </div>
    </div>
  )
}
