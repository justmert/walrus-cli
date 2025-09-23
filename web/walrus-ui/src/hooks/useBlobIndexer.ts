import { useState, useCallback } from 'react'

interface IndexedBlob {
  blobId: string
  suiObjectId: string
  size: number
  endEpoch?: number
  storageRebate: number
  createdAt: string
  owner: string
  contentType?: string
  available: boolean
  identifier?: string
  source: string
}

interface BlobIndexerResponse {
  success: boolean
  data?: IndexedBlob[]
  error?: string
}

const API_BASE_URL = 'http://localhost:3002'

export function useBlobIndexer() {
  const [indexedBlobs, setIndexedBlobs] = useState<IndexedBlob[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const listUserBlobs = useCallback(async (userAddress: string) => {
    if (!userAddress) {
      setError('User address is required')
      return []
    }

    setIsLoading(true)
    setError(null)

    try {
      const response = await fetch(`${API_BASE_URL}/api/blobs/list`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ userAddress })
      })

      const result: BlobIndexerResponse = await response.json()

      if (!result.success) {
        throw new Error(result.error || 'Failed to fetch blobs')
      }

      const blobs = result.data || []
      setIndexedBlobs(blobs)
      return blobs
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to fetch indexed blobs'
      setError(errorMessage)
      console.error('Blob indexer error:', err)
      return []
    } finally {
      setIsLoading(false)
    }
  }, [])

  const searchUserBlobs = useCallback(async (userAddress: string, query: string) => {
    if (!userAddress) {
      setError('User address is required')
      return []
    }

    setIsLoading(true)
    setError(null)

    try {
      const response = await fetch(`${API_BASE_URL}/api/blobs/search`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ userAddress, query })
      })

      const result: BlobIndexerResponse = await response.json()

      if (!result.success) {
        throw new Error(result.error || 'Failed to search blobs')
      }

      const blobs = result.data || []
      setIndexedBlobs(blobs)
      return blobs
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to search indexed blobs'
      setError(errorMessage)
      console.error('Blob search error:', err)
      return []
    } finally {
      setIsLoading(false)
    }
  }, [])

  const getBlobDetails = useCallback(async (blobId: string) => {
    if (!blobId) {
      setError('Blob ID is required')
      return null
    }

    setIsLoading(true)
    setError(null)

    try {
      const response = await fetch(`${API_BASE_URL}/api/blobs/details`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ blobId })
      })

      const result: BlobIndexerResponse = await response.json()

      if (!result.success) {
        throw new Error(result.error || 'Failed to get blob details')
      }

      const blob = result.data?.[0]
      return blob || null
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to get blob details'
      setError(errorMessage)
      console.error('Blob details error:', err)
      return null
    } finally {
      setIsLoading(false)
    }
  }, [])

  const clearError = useCallback(() => {
    setError(null)
  }, [])

  const refreshBlobs = useCallback(async (userAddress: string) => {
    return await listUserBlobs(userAddress)
  }, [listUserBlobs])

  return {
    indexedBlobs,
    isLoading,
    error,
    listUserBlobs,
    searchUserBlobs,
    getBlobDetails,
    clearError,
    refreshBlobs
  }
}

export type { IndexedBlob }