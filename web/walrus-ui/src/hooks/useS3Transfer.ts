import { useState, useCallback } from 'react'

interface S3Credentials {
  accessKeyId: string
  secretAccessKey: string
  region: string
  sessionToken?: string
}

interface S3Object {
  key: string
  size?: number
  lastModified: string
  etag?: string
}

interface TransferFilter {
  prefix: string
  include: string[]
  exclude: string[]
  minSize?: number
  maxSize?: number
}

interface TransferProgress {
  total: number
  completed: number
  failed: number
  currentFile?: string
}

interface TransferResult {
  key: string
  blobId?: string
  success: boolean
  error?: string
}

// Use localhost API server for S3 operations
const API_BASE_URL = 'http://localhost:3002'

export function useS3Transfer(
  walrusConfig: any,
  credentials: S3Credentials,
  isConnected: boolean,
  setIsConnected: (connected: boolean) => void,
  addTransferredFile: (file: any) => void
) {

  const [buckets, setBuckets] = useState<string[]>([])
  const [objects, setObjects] = useState<S3Object[]>([])
  const [selectedBucket, setSelectedBucket] = useState<string>('')
  const [selectedObjects, setSelectedObjects] = useState<Set<string>>(new Set())

  const [filters, setFilters] = useState<TransferFilter>({
    prefix: '',
    include: [],
    exclude: []
  })

  const [isLoadingBuckets, setIsLoadingBuckets] = useState(false)
  const [isLoadingObjects, setIsLoadingObjects] = useState(false)
  const [isTransferring, setIsTransferring] = useState(false)

  const [transferProgress, setTransferProgress] = useState<TransferProgress | null>(null)
  const [transferResults, setTransferResults] = useState<TransferResult[]>([])
  const [error, setError] = useState<string | null>(null)

  // Helper function for proxy requests
  const proxyRequest = useCallback(async (action: string, data: any = {}) => {
    try {
      const response = await fetch(`${API_BASE_URL}/api/s3/proxy`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          action,
          credentials: {
            accessKeyID: credentials.accessKeyId,
            secretAccessKey: credentials.secretAccessKey,
            region: credentials.region,
            sessionToken: credentials.sessionToken || ''
          },
          ...data
        })
      })

      const result = await response.json()

      if (!result.success) {
        throw new Error(result.error || 'Request failed')
      }

      return result.data
    } catch (err: any) {
      console.error(`S3 proxy error (${action}):`, err)
      throw err
    }
  }, [credentials])

  const connect = useCallback(async () => {
    try {
      setError(null)

      if (!credentials.accessKeyId || !credentials.secretAccessKey) {
        setError('Access Key ID and Secret Access Key are required')
        return false
      }

      // Test connection by listing buckets and store the results
      const data = await proxyRequest('listBuckets')
      const bucketNames = (data || []).map((b: any) => b.name || b)
      setBuckets(bucketNames)
      setIsConnected(true)
      return true
    } catch (err: any) {
      setError(err.message || 'Failed to connect to S3')
      return false
    }
  }, [credentials, proxyRequest, setIsConnected])

  const disconnect = useCallback(() => {
    setIsConnected(false)
    setBuckets([])
    setObjects([])
    setSelectedBucket('')
    setSelectedObjects(new Set())
    setTransferResults([])
  }, [setIsConnected])

  const listBuckets = useCallback(async () => {
    if (!isConnected) {
      setError('Not connected to S3')
      return
    }

    setIsLoadingBuckets(true)
    setError(null)

    try {
      const data = await proxyRequest('listBuckets')
      const bucketNames = (data || []).map((b: any) => b.name || b)
      setBuckets(bucketNames)
    } catch (err: any) {
      setError(err.message || 'Failed to list buckets')
    } finally {
      setIsLoadingBuckets(false)
    }
  }, [isConnected, proxyRequest])

  const listObjects = useCallback(async (bucket: string) => {
    if (!isConnected) {
      setError('Not connected to S3')
      return
    }

    setIsLoadingObjects(true)
    setError(null)

    try {
      const data = await proxyRequest('listObjects', {
        bucket,
        filter: {
          prefix: filters.prefix || '',
          include: filters.include,
          exclude: filters.exclude
        }
      })

      const objectList: S3Object[] = (data || []).map((obj: any) => ({
        key: obj.key,
        size: obj.size,
        lastModified: obj.lastModified,
        etag: obj.etag
      }))

      setObjects(objectList)
    } catch (err: any) {
      setError(err.message || 'Failed to list objects')
    } finally {
      setIsLoadingObjects(false)
    }
  }, [isConnected, filters, proxyRequest])

  const transferSingleFile = useCallback(async (
    bucket: string,
    key: string,
    epochs: number
  ): Promise<TransferResult> => {
    try {
      const response = await fetch(`${API_BASE_URL}/api/s3/transfer`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          credentials: {
            accessKeyID: credentials.accessKeyId,
            secretAccessKey: credentials.secretAccessKey,
            region: credentials.region,
            sessionToken: credentials.sessionToken || ''
          },
          bucket,
          keys: [key],
          epochs,
          encrypt: false
        })
      })

      const result = await response.json()

      if (!result.success) {
        throw new Error(result.error || 'Transfer failed')
      }

      const transferResult = result.data?.[0] || {}
      // If successful, add to the global files list
      if (transferResult.success) {
        const transferredFile = {
          name: key.split('/').pop() || key,
          size: transferResult.size || 0,
          blobId: transferResult.blobId,
          uploadedAt: new Date().toLocaleString(),
          source: 's3',
          originalS3Key: key,
          expiryEpoch: transferResult.expiryEpoch,
          registeredEpoch: transferResult.registeredEpoch,
          suiObjectId: transferResult.suiObjectId
        }
        addTransferredFile(transferredFile)
      }

      return {
        key,
        blobId: transferResult.blobId,
        success: transferResult.success || false,
        error: transferResult.error
      }
    } catch (err: any) {
      return {
        key,
        success: false,
        error: err.message || 'Transfer failed'
      }
    }
  }, [credentials, addTransferredFile])

  const startTransfer = useCallback(async (objectKeys: string[]) => {
    if (!selectedBucket || objectKeys.length === 0) {
      setError('No objects selected for transfer')
      return
    }

    setIsTransferring(true)
    setTransferProgress({
      total: objectKeys.length,
      completed: 0,
      failed: 0
    })
    setTransferResults([])

    const results: TransferResult[] = []
    const parallelLimit = 3

    for (let i = 0; i < objectKeys.length; i += parallelLimit) {
      const batch = objectKeys.slice(i, i + parallelLimit)

      const batchPromises = batch.map(key => {
        setTransferProgress(prev => ({
          ...prev!,
          currentFile: key
        }))

        return transferSingleFile(selectedBucket, key, walrusConfig.epochs)
      })

      const batchResults = await Promise.all(batchPromises)
      results.push(...batchResults)

      setTransferProgress(prev => ({
        ...prev!,
        completed: results.filter(r => r.success).length,
        failed: results.filter(r => !r.success).length
      }))

      setTransferResults([...results])
    }

    setIsTransferring(false)
    setTransferProgress(null)
  }, [selectedBucket, transferSingleFile, walrusConfig.epochs])

  const estimateCost = useCallback((totalSize: number) => {
    const encodedSizeBytes = totalSize * 5 + 64 * 1024 * 1024
    const encodedSizeMB = Math.ceil(encodedSizeBytes / (1024 * 1024))
    const costPerMBPerEpoch = 55000 / 5 // 11,000 FROST after subsidy
    const totalFROST = encodedSizeMB * costPerMBPerEpoch * walrusConfig.epochs
    return totalFROST / 1_000_000_000 // Convert to WAL
  }, [walrusConfig.epochs])

  return {
    buckets,
    objects,
    selectedBucket,
    setSelectedBucket,
    selectedObjects,
    setSelectedObjects,
    filters,
    setFilters,
    isConnected,
    isLoadingBuckets,
    isLoadingObjects,
    isTransferring,
    transferProgress,
    transferResults,
    connect,
    disconnect,
    listBuckets,
    listObjects,
    estimateCost,
    startTransfer,
    error
  }
}