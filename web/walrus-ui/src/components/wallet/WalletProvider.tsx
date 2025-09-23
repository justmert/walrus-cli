import { createContext, useContext, useState } from 'react'
import type { ReactNode } from 'react'

interface WalletError {
  message: string
  code?: string
  details?: string
}

interface WalletContextType {
  error: WalletError | null
  setError: (error: WalletError | null) => void
  clearError: () => void
}

const WalletContext = createContext<WalletContextType | undefined>(undefined)

export function WalletErrorProvider({ children }: { children: ReactNode }) {
  const [error, setError] = useState<WalletError | null>(null)

  const clearError = () => setError(null)

  return (
    <WalletContext.Provider value={{ error, setError, clearError }}>
      {children}
    </WalletContext.Provider>
  )
}

export function useWalletError() {
  const context = useContext(WalletContext)
  if (context === undefined) {
    throw new Error('useWalletError must be used within a WalletErrorProvider')
  }
  return context
}