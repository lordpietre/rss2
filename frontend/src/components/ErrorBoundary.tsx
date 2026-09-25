import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  message: string
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, message: '' }

  static getDerivedStateFromError(err: unknown): State {
    return { hasError: true, message: err instanceof Error ? err.message : String(err) }
  }

  componentDidCatch(err: unknown) {
    console.error('ErrorBoundary caught:', err)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen flex items-center justify-center p-6">
          <div className="max-w-md text-center">
            <h1 className="text-xl font-bold mb-2">Algo salió mal</h1>
            <p className="text-gray-500 mb-4">La vista no pudo renderizarse. Recarga o vuelve al inicio.</p>
            {this.state.message && (
              <pre className="text-xs bg-gray-100 dark:bg-gray-800 p-2 rounded mb-4 overflow-auto">{this.state.message}</pre>
            )}
            <div className="flex gap-2 justify-center">
              <button onClick={() => window.location.reload()} className="btn-primary">
                Recargar
              </button>
              <a href="/" className="btn-secondary">
                Ir al inicio
              </a>
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
