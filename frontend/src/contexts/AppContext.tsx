/**
 * App Context - Estado global de la aplicación RSS2
 * 
 * Maneja:
 * - Estado de autenticación
 * - Configuración del servidor (local vs cloud)
 * - Información del plan y límites
 */

import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react'
import { apiClient, serverConfig, ServerMode, User, SubscriptionStatus, UsageStats } from '../services/apiClient'

// =============================================================================
// TIPOS
// =============================================================================

interface AppState {
  // Servidor
  serverMode: ServerMode
  apiUrl: string
  isLocalServerAvailable: boolean
  isCheckingServer: boolean
  
  // Auth
  isAuthenticated: boolean
  user: User | null
  isLoading: boolean
  
  // Subscription (cloud only)
  subscription: SubscriptionStatus | null
  usage: UsageStats | null
  
  // Error
  error: string | null
}

interface AppContextType extends AppState {
  // Server
  setMode: (mode: ServerMode) => void
  checkLocalServer: () => Promise<boolean>
  updateLocalServerUrl: (url: string, port: number) => void
  
  // Auth
  login: (email: string, password: string) => Promise<void>
  register: (email: string, username: string, password: string) => Promise<void>
  logout: () => void
  refreshUser: () => Promise<void>
  
  // Subscription
  refreshSubscription: () => Promise<void>
  refreshUsage: () => Promise<void>
  
  // Utils
  clearError: () => void
}

// =============================================================================
// CONTEXT
// =============================================================================

const AppContext = createContext<AppContextType | null>(null)

export function useApp(): AppContextType {
  const context = useContext(AppContext)
  if (!context) {
    throw new Error('useApp must be used within AppProvider')
  }
  return context
}

// =============================================================================
// PROVIDER
// =============================================================================

interface AppProviderProps {
  children: ReactNode
}

export function AppProvider({ children }: AppProviderProps) {
  // Estado
  const [serverMode, setServerModeState] = useState<ServerMode>('cloud')
  const [apiUrl, setApiUrl] = useState<string>('')
  const [isLocalServerAvailable, setIsLocalServerAvailable] = useState(false)
  const [isCheckingServer, setIsCheckingServer] = useState(false)
  
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  
  const [subscription, setSubscription] = useState<SubscriptionStatus | null>(null)
  const [usage, setUsage] = useState<UsageStats | null>(null)
  
  const [error, setError] = useState<string | null>(null)

  // Inicialización
  useEffect(() => {
    const init = async () => {
      // Cargar configuración del servidor
      const config = serverConfig.getConfig()
      setServerModeState(config.mode)
      setApiUrl(serverConfig.getApiUrl())
      
      // Verificar si hay token guardado
      const token = localStorage.getItem('token')
      if (token) {
        try {
          const userData = await apiClient.getCurrentUser()
          setUser(userData)
          setIsAuthenticated(true)
          
          // Si es cloud, cargar suscripción
          if (config.mode === 'cloud') {
            await loadSubscription()
          }
        } catch (err) {
          // Token inválido
          localStorage.removeItem('token')
          localStorage.removeItem('user')
        }
      }
      
      setIsLoading(false)
    }
    
    init()
  }, [])

  // Verificar servidor local cuando cambia el modo
  useEffect(() => {
    if (serverMode === 'local') {
      checkLocalServer()
    }
  }, [serverMode])

  // =============================================================================
  // HELPERS
  // =============================================================================

  const loadSubscription = async () => {
    try {
      const sub = await apiClient.getSubscriptionStatus()
      setSubscription(sub)
    } catch (err) {
      console.error('Error loading subscription:', err)
    }
  }

  const loadUsage = async () => {
    try {
      const usageData = await apiClient.getUsage()
      setUsage(usageData)
    } catch (err) {
      console.error('Error loading usage:', err)
    }
  }

  // =============================================================================
  // ACCIONES
  // =============================================================================

  const setMode = useCallback((mode: ServerMode) => {
    serverConfig.setMode(mode)
    setServerModeState(mode)
    setApiUrl(serverConfig.getApiUrl())
    
    // Limpiar auth al cambiar de modo
    setIsAuthenticated(false)
    setUser(null)
    setSubscription(null)
    setUsage(null)
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }, [])

  const checkLocalServer = useCallback(async (): Promise<boolean> => {
    setIsCheckingServer(true)
    try {
      const available = await serverConfig.checkLocalServer()
      setIsLocalServerAvailable(available)
      return available
    } finally {
      setIsCheckingServer(false)
    }
  }, [])

  const updateLocalServerUrl = useCallback((url: string, port: number) => {
    serverConfig.setLocalUrl(url, port)
    setApiUrl(serverConfig.getApiUrl())
    // Revalidar
    if (serverMode === 'local') {
      checkLocalServer()
    }
  }, [serverMode, checkLocalServer])

  const login = useCallback(async (email: string, password: string) => {
    setError(null)
    setIsLoading(true)
    try {
      const response = await apiClient.login(email, password)
      localStorage.setItem('token', response.token)
      localStorage.setItem('user', JSON.stringify(response.user))
      setUser(response.user)
      setIsAuthenticated(true)
      
      // Si es cloud, cargar suscripción
      if (serverConfig.getMode() === 'cloud') {
        await loadSubscription()
        await loadUsage()
      }
    } catch (err: any) {
      if (err.response?.status === 401) {
        setError('Credenciales inválidas')
      } else if (err.message === 'NO_CONNECTION') {
        setError('No se puede conectar al servidor. Verifica la conexión.')
      } else {
        setError('Error al iniciar sesión')
      }
      throw err
    } finally {
      setIsLoading(false)
    }
  }, [serverMode])

  const register = useCallback(async (email: string, username: string, password: string) => {
    setError(null)
    setIsLoading(true)
    try {
      const response = await apiClient.register(email, username, password)
      localStorage.setItem('token', response.token)
      localStorage.setItem('user', JSON.stringify(response.user))
      setUser(response.user)
      setIsAuthenticated(true)
      
      // Si es cloud, cargar suscripción
      if (serverConfig.getMode() === 'cloud') {
        await loadSubscription()
        await loadUsage()
      }
    } catch (err: any) {
      if (err.response?.status === 409) {
        setError('El email o usuario ya existe')
      } else if (err.message === 'NO_CONNECTION') {
        setError('No se puede conectar al servidor. Verifica la conexión.')
      } else {
        setError('Error al registrar')
      }
      throw err
    } finally {
      setIsLoading(false)
    }
  }, [serverMode])

  const logout = useCallback(() => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    setUser(null)
    setIsAuthenticated(false)
    setSubscription(null)
    setUsage(null)
  }, [])

  const refreshUser = useCallback(async () => {
    try {
      const userData = await apiClient.getCurrentUser()
      setUser(userData)
      localStorage.setItem('user', JSON.stringify(userData))
    } catch (err) {
      console.error('Error refreshing user:', err)
    }
  }, [])

  const refreshSubscription = useCallback(async () => {
    if (serverConfig.getMode() === 'cloud') {
      await loadSubscription()
    }
  }, [serverMode])

  const refreshUsage = useCallback(async () => {
    if (serverConfig.getMode() === 'cloud') {
      await loadUsage()
    }
  }, [serverMode])

  const clearError = useCallback(() => {
    setError(null)
  }, [])

  // =============================================================================
  // VALUE
  // =============================================================================

  const value: AppContextType = {
    // State
    serverMode,
    apiUrl,
    isLocalServerAvailable,
    isCheckingServer,
    isAuthenticated,
    user,
    isLoading,
    subscription,
    usage,
    error,
    
    // Actions
    setMode,
    checkLocalServer,
    updateLocalServerUrl,
    login,
    register,
    logout,
    refreshUser,
    refreshSubscription,
    refreshUsage,
    clearError,
  }

  return (
    <AppContext.Provider value={value}>
      {children}
    </AppContext.Provider>
  )
}
