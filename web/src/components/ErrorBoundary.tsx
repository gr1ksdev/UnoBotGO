import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
 children: ReactNode
 fallback?: ReactNode
}

interface State {
 hasError: boolean
 error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
 constructor(props: Props) {
  super(props)
  this.state = { hasError: false, error: null }
 }

 static getDerivedStateFromError(error: Error): State {
  return { hasError: true, error }
 }

 componentDidCatch(error: Error, errorInfo: ErrorInfo) {
  console.error('ErrorBoundary caught:', error, errorInfo)
 }

 render() {
  if (this.state.hasError) {
   if (this.props.fallback) return this.props.fallback
   return (
    <main className="page" style={{ padding: '24px', textAlign: 'center' }}>
     <h1>Ocorreu um erro</h1>
     <p className="notice" role="alert">
      {this.state.error?.message || 'Erro inesperado na mesa.'}
     </p>
     <button
      className="primary"
      onClick={() => {
       this.setState({ hasError: false, error: null })
       window.location.reload()
      }}
     >
      Recarregar
     </button>
    </main>
   )
  }
  return this.props.children
 }
}
