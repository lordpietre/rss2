import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiService, api } from '../services/api'
import { User, Mail, Lock, Cpu, ChevronRight, ChevronLeft, Check, Sparkles, AlertCircle } from 'lucide-react'

interface WelcomeWizardProps {
  onComplete: () => void
}

export function WelcomeWizard({ onComplete }: WelcomeWizardProps) {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const [form, setForm] = useState({
    username: '',
    email: '',
    password: '',
    confirmPassword: '',
    preferCpu: true,
    workerCount: 2
  })
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [startingWorkers, setStartingWorkers] = useState(false)
  const totalSteps = 3

  const handleNext = () => {
    setError('')
    
    if (step === 1) {
      if (!form.username.trim() || !form.email.trim()) {
        setError('Por favor completa todos los campos')
        return
      }
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
        setError('Por favor ingresa un correo válido')
        return
      }
    }
    
    if (step === 2) {
      if (!form.password || form.password.length < 6) {
        setError('La contraseña debe tener al menos 6 caracteres')
        return
      }
      if (form.password !== form.confirmPassword) {
        setError('Las contraseñas no coinciden')
        return
      }
    }
    
    if (step < totalSteps) {
      setStep(step + 1)
    }
  }

  const handleBack = () => {
    setError('')
    if (step > 1) {
      setStep(step - 1)
    }
  }

  const handleSubmit = async () => {
    setError('')
    setLoading(true)
    setStartingWorkers(true)
    
    try {
      // 1. Registrar usuario
      const result = await apiService.register(form.email, form.username, form.password)
      localStorage.setItem('token', result.token)
      localStorage.setItem('user', JSON.stringify(result.user))
      
      // 2. Configurar workers
      await api.post('/admin/workers/config', {
        type: form.preferCpu ? 'cpu' : 'gpu',
        workers: form.workerCount
      })
      
      // 3. Iniciar workers automáticamente
      try {
        await api.post('/admin/workers/start', {
          type: form.preferCpu ? 'cpu' : 'gpu',
          workers: form.workerCount
        })
      } catch (workerErr) {
        console.warn('No se pudieron iniciar los workers:', workerErr)
      }
      
      onComplete()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Error al crear la cuenta')
      setLoading(false)
      setStartingWorkers(false)
    }
  }

  const renderStep = () => {
    switch (step) {
      case 1:
        return (
          <div className="space-y-4">
            <div className="text-center mb-6">
              <div className="w-16 h-16 bg-gradient-to-br from-primary-500 to-primary-700 rounded-full flex items-center justify-center mx-auto mb-4">
                <User className="h-8 w-8 text-white" />
              </div>
              <h2 className="text-xl font-bold">Bienvenido a RSS2</h2>
              <p className="text-gray-500 text-sm mt-1">Cuéntanos sobre ti</p>
            </div>
            
            <div>
              <label className="block text-sm font-medium mb-1">Nombre de usuario</label>
              <input
                type="text"
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                className="input w-full"
                placeholder="Tu nombre"
                autoFocus
              />
            </div>
            
            <div>
              <label className="block text-sm font-medium mb-1">Correo electrónico</label>
              <input
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                className="input w-full"
                placeholder="tu@email.com"
              />
            </div>
          </div>
        )
      
      case 2:
        return (
          <div className="space-y-4">
            <div className="text-center mb-6">
              <div className="w-16 h-16 bg-gradient-to-br from-purple-500 to-purple-700 rounded-full flex items-center justify-center mx-auto mb-4">
                <Lock className="h-8 w-8 text-white" />
              </div>
              <h2 className="text-xl font-bold">Protege tu cuenta</h2>
              <p className="text-gray-500 text-sm mt-1">Crea una contraseña segura</p>
            </div>
            
            <div>
              <label className="block text-sm font-medium mb-1">Contraseña</label>
              <input
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                className="input w-full"
                placeholder="Mínimo 6 caracteres"
              />
            </div>
            
            <div>
              <label className="block text-sm font-medium mb-1">Confirmar contraseña</label>
              <input
                type="password"
                value={form.confirmPassword}
                onChange={(e) => setForm({ ...form, confirmPassword: e.target.value })}
                className="input w-full"
                placeholder="Repite tu contraseña"
              />
            </div>
            
            <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3 flex items-start gap-2">
              <AlertCircle className="h-5 w-5 text-yellow-600 flex-shrink-0 mt-0.5" />
              <p className="text-sm text-yellow-800 dark:text-yellow-200">
                Como primer usuario, tu cuenta tendrá <strong>derechos de administrador</strong>
              </p>
            </div>
          </div>
        )
      
      case 3:
        return (
          <div className="space-y-4">
            <div className="text-center mb-6">
              <div className="w-16 h-16 bg-gradient-to-br from-green-500 to-green-700 rounded-full flex items-center justify-center mx-auto mb-4">
                <Cpu className="h-8 w-8 text-white" />
              </div>
              <h2 className="text-xl font-bold">Configuración de traducción</h2>
              <p className="text-gray-500 text-sm mt-1">¿Qué prefieres para procesar las traducciones?</p>
            </div>
            
            <div className="grid grid-cols-2 gap-4">
              <button
                type="button"
                onClick={() => setForm({ ...form, preferCpu: true })}
                className={`p-4 rounded-lg border-2 transition-all text-left ${
                  form.preferCpu 
                    ? 'border-green-500 bg-green-50 dark:bg-green-900/20' 
                    : 'border-gray-200 dark:border-gray-700 hover:border-green-300'
                }`}
              >
                <Cpu className="h-8 w-8 text-green-600 mb-2" />
                <h3 className="font-semibold">CPU</h3>
                <p className="text-xs text-gray-500 mt-1">
                  Más lento pero funciona en cualquier equipo
                </p>
              </button>
              
              <button
                type="button"
                onClick={() => setForm({ ...form, preferCpu: false })}
                className={`p-4 rounded-lg border-2 transition-all text-left ${
                  !form.preferCpu 
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20' 
                    : 'border-gray-200 dark:border-gray-700 hover:border-blue-300'
                }`}
              >
                <svg className="h-8 w-8 text-blue-600 mb-2" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                </svg>
                <h3 className="font-semibold">GPU</h3>
                <p className="text-xs text-gray-500 mt-1">
                  Mucho más rápido, necesita tarjeta NVIDIA
                </p>
              </button>
            </div>

            <div className="mt-6">
              <label className="block text-sm font-medium mb-2 text-center">
                Número de workers de traducción
              </label>
              <div className="flex flex-wrap justify-center gap-2">
                {[1, 2, 3, 4, 5, 6, 7, 8].map((num) => (
                  <button
                    key={num}
                    type="button"
                    onClick={() => setForm({ ...form, workerCount: num })}
                    className={`w-10 h-10 rounded-lg font-medium transition-all ${
                      form.workerCount === num
                        ? 'bg-primary-600 text-white'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                    }`}
                  >
                    {num}
                  </button>
                ))}
              </div>
              <p className="text-xs text-gray-500 text-center mt-2">
                Recomendado: 2-4 workers para CPU, 1-2 para GPU
              </p>
            </div>
            
            <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3 mt-4">
              <p className="text-sm text-blue-800 dark:text-blue-200">
                <strong>Resumen:</strong> {form.preferCpu ? 'CPU' : 'GPU'} con {form.workerCount} worker{form.workerCount > 1 ? 's' : ''}
              </p>
            </div>
          </div>
        )
      
      default:
        return null
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-50 via-white to-purple-50 dark:from-gray-900 dark:via-gray-800 dark:to-gray-900 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-primary-500 to-purple-600 rounded-2xl shadow-lg mb-4">
            <Sparkles className="h-8 w-8 text-white" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white">RSS2</h1>
          <p className="text-gray-500 dark:text-gray-400">Configuración inicial</p>
        </div>

        <div className="card p-6">
          <div className="flex items-center justify-between mb-6">
            {[1, 2, 3].map((s) => (
              <div key={s} className="flex items-center">
                <div className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-colors ${
                  step >= s 
                    ? 'bg-primary-600 text-white' 
                    : 'bg-gray-200 dark:bg-gray-700 text-gray-500'
                }`}>
                  {step > s ? <Check className="h-5 w-5" /> : s}
                </div>
                {s < 3 && (
                  <div className={`w-12 h-1 mx-1 rounded ${
                    step > s ? 'bg-primary-600' : 'bg-gray-200 dark:bg-gray-700'
                  }`} />
                )}
              </div>
            ))}
          </div>

          {renderStep()}

          {error && (
            <div className="mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
              <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
            </div>
          )}

          <div className="flex gap-3 mt-6">
            {step > 1 && (
              <button
                type="button"
                onClick={handleBack}
                className="btn-secondary flex items-center justify-center gap-2"
              >
                <ChevronLeft className="h-4 w-4" />
                Atrás
              </button>
            )}
            
            {step < totalSteps ? (
              <button
                type="button"
                onClick={handleNext}
                className="btn-primary flex-1 flex items-center justify-center gap-2"
              >
                Siguiente
                <ChevronRight className="h-4 w-4" />
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={loading}
                className="btn-primary flex-1 flex items-center justify-center gap-2"
              >
                {loading || startingWorkers ? (
                  <>
                    <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent" />
                    Configurando...
                  </>
                ) : (
                  <>
                    <Check className="h-4 w-4" />
                    Completar y iniciar
                  </>
                )}
              </button>
            )}
          </div>
        </div>

        <p className="text-center text-xs text-gray-400 mt-4">
          Paso {step} de {totalSteps}
        </p>
      </div>
    </div>
  )
}
