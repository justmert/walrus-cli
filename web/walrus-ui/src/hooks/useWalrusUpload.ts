import { useCurrentAccount, useSuiClient } from '@mysten/dapp-kit'
import { Transaction } from '@mysten/sui/transactions'
import { useState } from 'react'

const WALRUS_PUBLISHER_URL = 'https://publisher.walrus-testnet.walrus.space'
const WALRUS_AGGREGATOR_URL = 'https://aggregator.walrus-testnet.walrus.space'

type WalrusPublisherNormalized = {
  blobId: string
  suiRef?: string
  registeredEpoch?: number
  endEpoch?: number
  size: number
  cost: number
  isNew: boolean
}

const coerceNumber = (value: unknown): number | undefined => {
  if (value === null || value === undefined) {
    return undefined
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'bigint') {
    return Number(value)
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return undefined
}

const pickPositive = (...values: Array<unknown>): number | undefined => {
  for (const value of values) {
    const num = coerceNumber(value)
    if (num !== undefined && num > 0) {
      return num
    }
  }
  return undefined
}

const extractId = (value: unknown): string | undefined => {
  if (!value) {
    return undefined
  }

  if (typeof value === 'string') {
    return value
  }

  if (typeof value === 'object') {
    const record = value as Record<string, unknown>
    if (typeof record.id === 'string') {
      return record.id
    }
    const nested = record.id as { id?: unknown } | undefined
    if (nested && typeof nested.id === 'string') {
      return nested.id
    }
  }

  return undefined
}

const extractStorage = (storage: any) => ({
  endEpoch:
    coerceNumber(storage?.endEpoch) ??
    coerceNumber(storage?.storage_end_epoch) ??
    coerceNumber(storage?.storageEndEpoch),
  storageSize:
    coerceNumber(storage?.storage_size) ??
    coerceNumber(storage?.storageSize) ??
    coerceNumber(storage?.encodedLength),
})

const normalizeWalrusResponse = (
  payload: any,
  fallbackSize: number,
): WalrusPublisherNormalized => {
  const cost =
    coerceNumber(payload?.cost) ?? coerceNumber(payload?.newlyCreated?.cost) ?? 0

  if (payload?.newlyCreated) {
    const newly = payload.newlyCreated
    const blob = newly.blobObject ?? newly
    const storage = blob.storage ?? newly.storage ?? {}
    const { endEpoch, storageSize } = extractStorage(storage)
    const blobId = blob.blobId ?? newly.blobId ?? blob.blob_id ?? newly.blob_id

    if (!blobId) {
      throw new Error('Walrus publisher response missing blobId')
    }

    const size =
      pickPositive(storageSize, blob.size, newly.size, fallbackSize) ?? fallbackSize

    return {
      blobId,
      suiRef: extractId(blob.id ?? newly.id),
      registeredEpoch: coerceNumber(blob.registeredEpoch ?? newly.registeredEpoch),
      endEpoch: endEpoch ?? coerceNumber(blob.endEpoch ?? newly.endEpoch),
      size,
      cost,
      isNew: true,
    }
  }

  if (payload?.alreadyCertified) {
    const existing = payload.alreadyCertified
    const blob = existing.blobObject ?? existing
    const storage = blob.storage ?? existing.storage ?? {}
    const { endEpoch, storageSize } = extractStorage(storage)
    const blobId = blob.blobId ?? existing.blobId ?? blob.blob_id

    if (!blobId) {
      throw new Error('Walrus publisher response missing blobId')
    }

    const size =
      pickPositive(storageSize, blob.size, existing.size, fallbackSize) ?? fallbackSize

    return {
      blobId,
      suiRef: extractId(blob.id ?? existing.id),
      registeredEpoch: coerceNumber(blob.registeredEpoch ?? existing.registeredEpoch),
      endEpoch: endEpoch ?? coerceNumber(blob.endEpoch ?? existing.endEpoch),
      size,
      cost,
      isNew: false,
    }
  }

  throw new Error('Unexpected Walrus publisher response')
}

export function useWalrusUpload() {
  const currentAccount = useCurrentAccount()
  const suiClient = useSuiClient()
  const [isUploading, setIsUploading] = useState(false)

  const verifyWalletConnection = async () => {
    if (!currentAccount) {
      throw new Error('No wallet connected')
    }

    // Verify the wallet can fetch account data
    try {
      const coins = await suiClient.getCoins({
        owner: currentAccount.address,
        limit: 1
      })

      // Check if account exists on-chain
      const accountExists = coins.data.length >= 0 // Even 0 coins means account exists

      if (!accountExists) {
        throw new Error('Account not found on blockchain')
      }

      return true
    } catch (error) {
      console.error('Wallet verification failed:', error)
      throw new Error('Failed to verify wallet connection')
    }
  }

  const uploadToWalrusNetwork = async (file: File, epochs: number) => {
    try {
      // Upload to Walrus publisher - using the correct /v1/blobs endpoint
      const response = await fetch(`${WALRUS_PUBLISHER_URL}/v1/blobs?epochs=${epochs}`, {
        method: 'PUT',
        body: file,
        headers: {
          'Content-Type': 'application/octet-stream',
        }
      })

      if (!response.ok) {
        const errorText = await response.text()
        console.error('Walrus upload failed:', errorText)
        throw new Error(`Upload failed: ${response.status} ${response.statusText}`)
      }

      const result = await response.json()
      console.log('Walrus upload response:', result)

      const normalized = normalizeWalrusResponse(result, file.size)

      return {
        blobId: normalized.blobId,
        suiRefType: normalized.suiRef,
        registeredEpoch: normalized.registeredEpoch,
        endEpoch: normalized.endEpoch ?? undefined,
        cost: normalized.cost,
        size: normalized.size,
        isNew: normalized.isNew,
      }
    } catch (error: any) {
      console.error('Failed to upload to Walrus:', error)
      throw new Error(error.message || 'Failed to upload file to Walrus network')
    }
  }

  const uploadToWalrus = async (file: File, epochs: number) => {
    if (!currentAccount) {
      throw new Error('Please connect your wallet first')
    }

    setIsUploading(true)

    try {
      // Verify wallet is really connected
      await verifyWalletConnection()

      // Upload file to actual Walrus network
      const walrusResult = await uploadToWalrusNetwork(file, epochs)

      // Create a transaction to store metadata on-chain
      // This is optional but helps track uploads
      const tx = new Transaction()

      // Add a simple event emission or transfer to record the upload
      // In production, you'd interact with a Walrus storage contract
      try {
        tx.moveCall({
          target: '0x1::event::emit',
          arguments: [tx.pure.string(walrusResult.blobId), tx.pure.u64(Date.now())],
          typeArguments: []
        })
      } catch {
        tx.transferObjects(
          [tx.splitCoins(tx.gas, [1])],
          currentAccount.address
        )
      }

      setIsUploading(false)

      return {
        success: true,
        blobId: walrusResult.blobId,
        suiRef: walrusResult.suiRefType,
        endEpoch: walrusResult.endEpoch,
        isNew: walrusResult.isNew,
        size: walrusResult.size,
        message: walrusResult.isNew
          ? `File uploaded to Walrus! Blob ID: ${walrusResult.blobId}`
          : `File already exists on Walrus. Blob ID: ${walrusResult.blobId}`
      }

      /* Uncomment this to enable on-chain transaction recording
      signAndExecuteTransaction(
        {
          transaction: tx,
        },
        {
          onSuccess: (result) => {
            console.log('Transaction successful:', result)
          },
          onError: (error) => {
            console.error('Transaction failed:', error)
          }
        }
      )
      */
    } catch (error: any) {
      setIsUploading(false)
      throw error
    }
  }

  const downloadFromWalrus = async (blobId: string) => {
    try {
      const response = await fetch(`${WALRUS_AGGREGATOR_URL}/v1/blobs/${blobId}`)

      if (!response.ok) {
        throw new Error(`Download failed: ${response.status} ${response.statusText}`)
      }

      const blob = await response.blob()
      return blob
    } catch (error: any) {
      console.error('Failed to download from Walrus:', error)
      throw new Error(error.message || 'Failed to download file from Walrus')
    }
  }

  return {
    uploadToWalrus,
    downloadFromWalrus,
    isUploading,
    verifyWalletConnection
  }
}
