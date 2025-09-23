import { useCurrentAccount, useSuiClient, useSignAndExecuteTransaction } from '@mysten/dapp-kit'
import { WalrusClient, WalrusFile, RetryableWalrusClientError } from '@mysten/walrus'
import { getFullnodeUrl, SuiClient } from '@mysten/sui/client'
import { useState, useEffect, useRef, useCallback } from 'react'

const READ_ONLY_SUI_CLIENT = new SuiClient({ url: getFullnodeUrl('testnet') })

export function useWalrusSDK() {
  const currentAccount = useCurrentAccount()
  const suiClient = useSuiClient()
  const { mutate: signAndExecuteTransaction } = useSignAndExecuteTransaction()
  const activeSuiClient = currentAccount ? suiClient : READ_ONLY_SUI_CLIENT
  const [isUploading, setIsUploading] = useState(false)
  const [isDownloading, setIsDownloading] = useState(false)
  const [isInitialized, setIsInitialized] = useState(false)
  const walrusClientRef = useRef<WalrusClient | null>(null)
  const initializationAttempts = useRef(0)
  const lastSuiClientRef = useRef<SuiClient | null>(activeSuiClient)

  useEffect(() => {
    if (lastSuiClientRef.current === activeSuiClient) {
      return
    }

    lastSuiClientRef.current = activeSuiClient

    if (walrusClientRef.current) {
      walrusClientRef.current = null
      setIsInitialized(false)
      initializationAttempts.current = 0
    }
  }, [activeSuiClient])

  // Initialize WalrusClient with proper configuration
  useEffect(() => {
    const initializeWalrus = async () => {
      if (walrusClientRef.current || initializationAttempts.current > 3) {
        return
      }

      try {
        initializationAttempts.current++

        // Import WASM URL dynamically for better compatibility
        const wasmModule = await import('@mysten/walrus-wasm/web/walrus_wasm_bg.wasm?url')
        const wasmUrl = wasmModule.default

        // Create WalrusClient with optimized configuration
        walrusClientRef.current = new WalrusClient({
          network: 'testnet',
          suiClient: activeSuiClient,
          wasmUrl,
          // Configure upload relay to reduce client-side requests
          uploadRelay: {
            host: 'https://upload-relay.testnet.walrus.space',
            sendTip: {
              max: 1_000 // Max tip of 1000 MIST
            }
          },
          // Optimize network requests with timeout and custom fetch
          storageNodeClientOptions: {
            timeout: 60_000, // 60 second timeout
            onError: (error: any) => {
              console.warn('Storage node error:', error.message || error)
            },
            fetch: (url, options) => {
              // Add retry logic for transient failures
              return fetch(url, {
                ...options,
                signal: AbortSignal.timeout(60_000)
              })
            }
          }
        })

        setIsInitialized(true)
        console.log('Walrus client initialized successfully')
      } catch (error) {
        console.error('Failed to initialize Walrus client:', error)
        // Retry after a delay
        if (initializationAttempts.current < 3) {
          setTimeout(() => initializeWalrus(), 2000 * initializationAttempts.current)
        }
      }
    }

    initializeWalrus()
  }, [activeSuiClient])

  // Upload file using WalrusFile API for better abstraction
  const uploadToWalrus = async (file: File, epochs: number = 5) => {
    if (!currentAccount) {
      throw new Error('Please connect your wallet first')
    }

    if (!walrusClientRef.current) {
      throw new Error('Walrus client not initialized. Please wait...')
    }

    setIsUploading(true)

    try {
      // Convert file to WalrusFile with metadata
      const buffer = await file.arrayBuffer()
      const walrusFile = WalrusFile.from({
        contents: new Uint8Array(buffer),
        identifier: file.name,
        tags: {
          'content-type': file.type || 'application/octet-stream',
          'uploaded-at': new Date().toISOString(),
          'file-size': file.size.toString()
        }
      })

      // Use writeFilesFlow for better control and progress tracking
      const flow = walrusClientRef.current.writeFilesFlow({
        files: [walrusFile],
      })

      // Encode the files first
      await flow.encode()
      console.log('Files encoded, ready for upload')

      return {
        flow,
        fileName: file.name,
        fileSize: file.size,
        epochs,
        walrusFile
      }
    } catch (error: any) {
      setIsUploading(false)
      // Check for retryable errors
      if (error instanceof RetryableWalrusClientError) {
        walrusClientRef.current?.reset()
        throw new Error('Network error. Please try again.')
      }
      throw new Error(error.message || 'Failed to prepare file for upload')
    }
  }

  // Execute the full upload flow with improved error handling
  const executeUploadFlow = async (
    flow: any,
    epochs: number,
    onRegisterSuccess?: (result: any) => void,
    onUploadSuccess?: () => void,
    onProgress?: (progress: number) => void
  ) => {
    if (!currentAccount) {
      throw new Error('Wallet not connected')
    }

    try {
      // Step 1: Register files on-chain
      const registerTx = flow.register({
        epochs,
        deletable: true,
        owner: currentAccount.address
      })

      return new Promise((resolve, reject) => {
        signAndExecuteTransaction(
          { transaction: registerTx },
          {
            onSuccess: async (registerResult) => {
              console.log('Files registered:', registerResult)

              if (onRegisterSuccess) {
                onRegisterSuccess(registerResult)
              }

              try {
                // Step 2: Upload to storage nodes (or upload relay if configured)
                await flow.upload({
                  digest: registerResult.digest,
                  progressCallback: (progress: any) => {
                    console.log('Upload progress:', progress)
                    if (onProgress) {
                      // Calculate progress percentage
                      const percentage = progress.completed
                        ? Math.round((progress.completed / progress.total) * 100)
                        : 50
                      onProgress(percentage)
                    }
                  }
                })
                console.log('Upload complete')

                if (onUploadSuccess) {
                  onUploadSuccess()
                }

                // Step 3: Certify the files
                const certifyTx = flow.certify()

                signAndExecuteTransaction(
                  { transaction: certifyTx },
                  {
                    onSuccess: async (certifyResult) => {
                      console.log('Files certified:', certifyResult)
                      setIsUploading(false)

                      // Get the list of uploaded files
                      const uploadedFiles = await flow.listFiles()

                      resolve({
                        files: uploadedFiles,
                        blobId: uploadedFiles[0]?.blobId,
                        registerDigest: registerResult.digest,
                        certifyDigest: certifyResult.digest
                      })
                    },
                    onError: (error) => {
                      console.error('Certification failed:', error)
                      setIsUploading(false)

                      // Check if it's a retryable error
                      if (error instanceof RetryableWalrusClientError) {
                        walrusClientRef.current?.reset()
                        reject(new Error('Network error during certification. Please try again.'))
                      } else {
                        reject(new Error('Failed to certify files: ' + error.message))
                      }
                    }
                  }
                )
              } catch (uploadError: any) {
                console.error('Upload to storage failed:', uploadError)
                setIsUploading(false)

                if (uploadError instanceof RetryableWalrusClientError) {
                  walrusClientRef.current?.reset()
                  reject(new Error('Network error during upload. Please try again.'))
                } else {
                  reject(new Error('Failed to upload to storage: ' + uploadError.message))
                }
              }
            },
            onError: (error) => {
              console.error('Registration failed:', error)
              setIsUploading(false)
              reject(new Error('Failed to register files: ' + error.message))
            }
          }
        )
      })
    } catch (error: any) {
      setIsUploading(false)
      throw error
    }
  }

  // Download files from Walrus using WalrusFile API
  const downloadFromWalrus = async (blobId: string): Promise<Blob> => {
    if (!walrusClientRef.current) {
      throw new Error('Walrus client not initialized')
    }

    setIsDownloading(true)

    try {
      // Use getFiles for better file handling
      const files = await walrusClientRef.current.getFiles({ ids: [blobId] })

      if (files.length === 0) {
        throw new Error('File not found')
      }

      const walrusFile = files[0]

      // Get file metadata if available
      const identifier = await walrusFile.getIdentifier()
      const tags = await walrusFile.getTags()
      const contentType = tags['content-type'] || 'application/octet-stream'

      // Get file contents
      const data = await walrusFile.bytes()

      // Create blob with proper content type
      const blob = new Blob([data], { type: contentType })

      // Add filename if available
      if (identifier) {
        Object.defineProperty(blob, 'name', {
          value: identifier,
          writable: false
        })
      }

      return blob
    } catch (error: any) {
      console.error('Download error:', error)

      // Check for retryable errors
      if (error instanceof RetryableWalrusClientError) {
        walrusClientRef.current?.reset()
        throw new Error('Network error. Please try again.')
      }

      throw new Error(error.message || 'Failed to download from Walrus')
    } finally {
      setIsDownloading(false)
    }
  }

  // Get blob info with enhanced metadata
  const getBlobInfo = async (blobId: string) => {
    if (!walrusClientRef.current) {
      throw new Error('Walrus client not initialized')
    }

    try {
      // Try to get as WalrusBlob first for more info
      const blob = await walrusClientRef.current.getBlob({ blobId })
      const metadata = await walrusClientRef.current.getBlobMetadata({ blobId })

      // Check if it's a quilt with multiple files
      let fileCount = 1
      let files: any[] = []

      try {
        files = await blob.files()
        fileCount = files.length
      } catch {
        // Not a quilt, single blob
      }

      return {
        ...metadata,
        fileCount,
        isQuilt: fileCount > 1,
        files: files.map(f => ({
          identifier: f.identifier,
          tags: f.tags
        }))
      }
    } catch (error: any) {
      console.error('Failed to get blob info:', error)

      if (error instanceof RetryableWalrusClientError) {
        walrusClientRef.current?.reset()
        throw new Error('Network error. Please try again.')
      }

      throw error
    }
  }

  // Get storage cost estimation from the Walrus client
  const estimateStorageCost = useCallback(async (size: number, epochs: number) => {
    const client = walrusClientRef.current
    if (!client) {
      throw new Error('Walrus client not initialized')
    }

    if (epochs <= 0) {
      throw new Error('Epochs must be greater than zero')
    }

    const frostToWal = (value: bigint) => Number(value) / 1_000_000_000

    const { storageCost, writeCost, totalCost } = await client.storageCost(size, epochs)

    return {
      storageCost: frostToWal(storageCost),
      writeCost: frostToWal(writeCost),
      totalCost: frostToWal(totalCost)
    }
  }, [])

  // Upload multiple files as a quilt
  const uploadMultipleFiles = useCallback(async (
    files: File[],
    epochs: number = 5
  ) => {
    if (!currentAccount) {
      throw new Error('Please connect your wallet first')
    }

    if (!walrusClientRef.current) {
      throw new Error('Walrus client not initialized')
    }

    setIsUploading(true)

    try {
      // Convert files to WalrusFiles
      const walrusFiles = await Promise.all(
        files.map(async (file) => {
          const buffer = await file.arrayBuffer()
          return WalrusFile.from({
            contents: new Uint8Array(buffer),
            identifier: file.name,
            tags: {
              'content-type': file.type || 'application/octet-stream',
              'file-size': file.size.toString()
            }
          })
        })
      )

      // Create upload flow for multiple files
      const flow = walrusClientRef.current.writeFilesFlow({
        files: walrusFiles,
      })

      await flow.encode()

      return {
        flow,
        fileCount: files.length,
        totalSize: files.reduce((sum, f) => sum + f.size, 0),
        epochs
      }
    } catch (error: any) {
      setIsUploading(false)

      if (error instanceof RetryableWalrusClientError) {
        walrusClientRef.current?.reset()
        throw new Error('Network error. Please try again.')
      }

      throw new Error(error.message || 'Failed to prepare files for upload')
    }
  }, [currentAccount])

  // Get Walrus system state for debugging
  const getSystemState = useCallback(async () => {
    if (!walrusClientRef.current) {
      throw new Error('Walrus client not initialized')
    }

    try {
      const [systemState, stakingState] = await Promise.all([
        walrusClientRef.current.systemState(),
        walrusClientRef.current.stakingState()
      ])

      return {
        system: systemState,
        staking: stakingState,
        currentEpoch: stakingState.epoch,
        storagePrice: systemState.storage_price_per_unit_size,
        writePrice: systemState.write_price_per_unit_size
      }
    } catch (error) {
      console.error('Failed to get system state:', error)
      throw error
    }
  }, [])

  return {
    uploadToWalrus,
    executeUploadFlow,
    downloadFromWalrus,
    getBlobInfo,
    estimateStorageCost,
    uploadMultipleFiles,
    getSystemState,
    isUploading,
    isDownloading,
    isInitialized,
    walrusClient: walrusClientRef.current
  }
}
