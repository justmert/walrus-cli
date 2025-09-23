import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'

interface S3Credentials {
  accessKeyId: string
  secretAccessKey: string
  region: string
  sessionToken?: string
}

interface S3ContextType {
  credentials: S3Credentials
  setCredentials: (creds: S3Credentials) => void
  isS3Connected: boolean
  setIsS3Connected: (connected: boolean) => void
  s3TransferredFiles: any[]
  addS3TransferredFile: (file: any) => void
}

const S3Context = createContext<S3ContextType | undefined>(undefined)

// localStorage keys
const STORAGE_KEYS = {
  CREDENTIALS: 'walrus-s3-credentials',
  CONNECTED: 'walrus-s3-connected',
  TRANSFERRED_FILES: 'walrus-s3-transferred-files'
}

// Helper functions for localStorage
const loadFromStorage = <T,>(key: string, defaultValue: T): T => {
  try {
    const stored = localStorage.getItem(key)
    return stored ? JSON.parse(stored) : defaultValue
  } catch {
    return defaultValue
  }
}

const saveToStorage = (key: string, value: any) => {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Ignore storage errors
  }
}

export function S3Provider({ children }: { children: ReactNode }) {
  const [credentials, setCredentialsState] = useState<S3Credentials>(() =>
    loadFromStorage(STORAGE_KEYS.CREDENTIALS, {
      accessKeyId: '',
      secretAccessKey: '',
      region: 'us-east-1',
      sessionToken: ''
    })
  )

  const [isS3Connected, setIsS3ConnectedState] = useState(() =>
    loadFromStorage(STORAGE_KEYS.CONNECTED, false)
  )

  const [s3TransferredFiles, setS3TransferredFilesState] = useState<any[]>(() =>
    loadFromStorage(STORAGE_KEYS.TRANSFERRED_FILES, [])
  )

  // Wrapper functions that also save to localStorage
  const setCredentials = (creds: S3Credentials) => {
    setCredentialsState(creds)
    saveToStorage(STORAGE_KEYS.CREDENTIALS, creds)
  }

  const setIsS3Connected = (connected: boolean) => {
    setIsS3ConnectedState(connected)
    saveToStorage(STORAGE_KEYS.CONNECTED, connected)
  }

  const addS3TransferredFile = (file: any) => {
    setS3TransferredFilesState(prev => {
      const newFiles = [file, ...prev]
      saveToStorage(STORAGE_KEYS.TRANSFERRED_FILES, newFiles)
      return newFiles
    })
  }

  // Clear localStorage when disconnecting
  useEffect(() => {
    if (!isS3Connected) {
      saveToStorage(STORAGE_KEYS.CONNECTED, false)
    }
  }, [isS3Connected])

  return (
    <S3Context.Provider value={{
      credentials,
      setCredentials,
      isS3Connected,
      setIsS3Connected,
      s3TransferredFiles,
      addS3TransferredFile
    }}>
      {children}
    </S3Context.Provider>
  )
}

export function useS3Context() {
  const context = useContext(S3Context)
  if (context === undefined) {
    throw new Error('useS3Context must be used within a S3Provider')
  }
  return context
}