import React, { useState } from 'react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import {
  Upload,
  Download,
  FileText,
  HardDrive,
  ExternalLink,
  Copy,
  Check,
  AlertCircle,
  Loader2,
  Search,
  RefreshCw,
  Lock,
  Shield
} from 'lucide-react'
import { formatBytes, formatWAL } from '@/lib/utils'
import { WalletButton, WalletStatus } from '@/components/wallet'
import { useNotification } from '@/hooks/useNotification'
import { NotificationContainer } from '@/components/NotificationContainer'
import { useWalrusSDK } from '@/hooks/useWalrusSDK'
import { useSealEncryption, type EncryptionConfig, type EncryptedFileMetadata } from '@/hooks/useSealEncryption'
import { useSealAllowlists } from '@/hooks/useSealAllowlists'
import { SealAllowlistPanel } from '@/components/SealAllowlistPanel'
import { useCurrentAccount } from '@mysten/dapp-kit'
import { ThemeToggle } from '@/components/ThemeToggle'
import { S3Transfer } from '@/components/S3Transfer'
import { S3Provider, useS3Context } from '@/contexts/S3Context'

// Types
const DEFAULT_SEAL_PACKAGE_ID = '0xc5ce2742cac46421b62028557f1d7aea8a4c50f651379a79afdf12cd88628807'
const DEFAULT_SEAL_MODULE_NAME = 'allowlist'
const DEFAULT_SEAL_MVR_NAME = '@pkg/seal-demo-1234'

const WALRUS_SERVICES = [
  {
    id: 'mysten-testnet',
    name: 'Walrus (Mysten testnet)',
    aggregatorUrl: 'https://aggregator.walrus-testnet.walrus.space',
    publisherUrl: 'https://publisher.walrus-testnet.walrus.space'
  },
  {
    id: 'custom',
    name: 'Custom endpoints',
    aggregatorUrl: '',
    publisherUrl: ''
  }
]

const coerceNumber = (value: unknown): number | undefined => {
  if (value === null || value === undefined) return undefined
  const num = Number(value)
  return Number.isFinite(num) ? num : undefined
}

const extractEndEpoch = (source: any): number | undefined => {
  if (!source) return undefined
  return (
    coerceNumber(source?.storage?.end_epoch) ??
    coerceNumber(source?.storage?.endEpoch) ??
    coerceNumber(source?.end_epoch) ??
    coerceNumber(source?.endEpoch)
  )
}

const extractRegisteredEpoch = (source: any): number | undefined =>
  coerceNumber(source?.registered_epoch ?? source?.registeredEpoch)

const extractIdentifier = (files: any[] | undefined): string | undefined => {
  if (!files || files.length === 0) return undefined
  const [first] = files
  return typeof first?.identifier === 'string' ? first.identifier : undefined
}

const truncateObjectId = (id: string) => (id.length > 10 ? `${id.slice(0, 10)}…` : id)

interface StoredFile {
  name: string
  size: number
  blobId: string
  uploadedAt: string
  expiryEpoch?: number
  registeredEpoch?: number
  suiObjectId?: string
  encryptionMetadata?: EncryptedFileMetadata
}

interface UploadFlowResult {
  files?: Array<{ blobId?: string; blobObject?: any; identifier?: string }>
  blobId?: string
}

interface UploadResult {
  blobId: string
  status: string
  aggregatorUrl: string
  publisherUrl: string
  suiObjectId?: string
  endEpoch?: number
  registeredEpoch?: number
  walrusExplorerUrl?: string
  suiExplorerUrl?: string
  encryptionEnabled: boolean
}

interface NetworkConfig {
  network: 'testnet' | 'mainnet' | 'custom'
  aggregatorUrl: string
  publisherUrl: string
  epochs: number
}

function AppContent() {
  const { notifications, success, error, warning, info, removeNotification } = useNotification()
  const currentAccount = useCurrentAccount()
  const {
    uploadToWalrus,
    executeUploadFlow,
    downloadFromWalrus,
    estimateStorageCost,
    getBlobInfo,
    isDownloading: sdkDownloading,
    isInitialized: walrusReady
  } = useWalrusSDK()
  const { encryptFile, decryptFile, keyServers, isEncrypting } = useSealEncryption()
  const keyServerCount = keyServers.length || 1
  const [activeTab, setActiveTab] = useState('upload')
  const [uploadProgress, setUploadProgress] = useState(0)
  const [isUploading, setIsUploading] = useState(false)
  const [copiedBlobId, setCopiedBlobId] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [enableEncryption, setEnableEncryption] = useState(() =>
    localStorage.getItem('walrus-enable-encryption') === 'true'
  )
  const [encryptionThreshold, setEncryptionThreshold] = useState(() =>
    parseInt(localStorage.getItem('walrus-encryption-threshold') || '2')
  )

  // Store files uploaded in this session
  const [files, setFiles] = useState<StoredFile[]>([])
  const { s3TransferredFiles } = useS3Context()

  // Combine regular files with S3 transferred files and sort by upload time (most recent first)
  const allFiles = [...files, ...s3TransferredFiles].sort((a, b) => {
    // Parse dates and sort in descending order
    const dateA = new Date(a.uploadedAt).getTime()
    const dateB = new Date(b.uploadedAt).getTime()
    return dateB - dateA // Most recent first
  })
  const [lastUploadResult, setLastUploadResult] = useState<UploadResult | null>(null)

  // Persisted settings
  const [sealPackageId, setSealPackageId] = useState(() =>
    localStorage.getItem('walrus-seal-package-id') || DEFAULT_SEAL_PACKAGE_ID
  )
  const [sealModuleName, setSealModuleName] = useState(() =>
    localStorage.getItem('walrus-seal-module-name') || DEFAULT_SEAL_MODULE_NAME
  )
  const [sealPolicyId, setSealPolicyId] = useState(() =>
    localStorage.getItem('walrus-seal-policy-id') || ''
  )
  const [sealMvrName, setSealMvrName] = useState(() =>
    localStorage.getItem('walrus-seal-mvr-name') || DEFAULT_SEAL_MVR_NAME
  )
  const [selectedService, setSelectedService] = useState(() =>
    localStorage.getItem('walrus-selected-service') || WALRUS_SERVICES[0].id
  )

  const {
    allowlists,
    loading: allowlistsLoading,
    refresh: refreshAllowlists,
  } = useSealAllowlists({ packageId: sealPackageId })

  React.useEffect(() => {
    if (!enableEncryption) {
      return
    }
    setEncryptionThreshold((prev) => {
      const next = Math.min(prev, Math.max(1, keyServerCount))
      return next === prev ? prev : next
    })
  }, [enableEncryption, keyServerCount])

  const [config, setConfig] = useState<NetworkConfig>(() => {
    const stored = localStorage.getItem('walrus-network-config')
    return stored ? JSON.parse(stored) : {
      network: 'testnet',
      aggregatorUrl: 'https://aggregator.walrus-testnet.walrus.space',
      publisherUrl: 'https://publisher.walrus-testnet.walrus.space',
      epochs: 5
    }
  })

  // Persist settings to localStorage
  React.useEffect(() => {
    localStorage.setItem('walrus-network-config', JSON.stringify(config))
  }, [config])

  React.useEffect(() => {
    localStorage.setItem('walrus-seal-package-id', sealPackageId)
  }, [sealPackageId])

  React.useEffect(() => {
    localStorage.setItem('walrus-seal-module-name', sealModuleName)
  }, [sealModuleName])

  React.useEffect(() => {
    localStorage.setItem('walrus-seal-policy-id', sealPolicyId)
  }, [sealPolicyId])

  React.useEffect(() => {
    localStorage.setItem('walrus-seal-mvr-name', sealMvrName)
  }, [sealMvrName])

  React.useEffect(() => {
    localStorage.setItem('walrus-selected-service', selectedService)
  }, [selectedService])

  React.useEffect(() => {
    localStorage.setItem('walrus-enable-encryption', enableEncryption.toString())
  }, [enableEncryption])

  React.useEffect(() => {
    localStorage.setItem('walrus-encryption-threshold', encryptionThreshold.toString())
  }, [encryptionThreshold])


  // Cost calculator state
  const [calculatorSize, setCalculatorSize] = useState<number>(100) // 100MB default
  const [calculatorEpochs, setCalculatorEpochs] = useState<number>(config.epochs)
  const [calculatorCost, setCalculatorCost] = useState<{ total: number; storage: number; write: number } | null>(null)
  const [uploadCost, setUploadCost] = useState<{ total: number; storage: number; write: number } | null>(null)
  const [walPriceUSD] = useState<number>(0.425) // This should come from an API in production

  // Auto-calculate cost whenever inputs change
  React.useEffect(() => {
    let cancelled = false

    const calculateCost = async () => {
      if (calculatorSize <= 0 || calculatorEpochs <= 0) {
        setCalculatorCost(null)
        return
      }

      if (!walrusReady) {
        setCalculatorCost(null)
        return
      }

      try {
        const sizeInBytes = calculatorSize * 1024 * 1024
        const costs = await estimateStorageCost(sizeInBytes, calculatorEpochs)

        if (!cancelled) {
          setCalculatorCost({
            total: costs.totalCost,
            storage: costs.storageCost,
            write: costs.writeCost,
          })
        }
      } catch (err) {
        console.error('Cost calculation failed:', err)
        if (!cancelled) {
          setCalculatorCost(null)
        }
      }
    }

    calculateCost()

    return () => {
      cancelled = true
    }
  }, [calculatorSize, calculatorEpochs, estimateStorageCost, walrusReady])

  // Calculate upload cost when file is selected
  React.useEffect(() => {
    let cancelled = false

    const calculateUploadCost = async () => {
      if (!selectedFile || config.epochs <= 0) {
        setUploadCost(null)
        return
      }

      if (!walrusReady) {
        setUploadCost(null)
        return
      }

      try {
        const costs = await estimateStorageCost(selectedFile.size, config.epochs)

        if (!cancelled) {
          setUploadCost({
            total: costs.totalCost,
            storage: costs.storageCost,
            write: costs.writeCost,
          })
        }
      } catch (err) {
        console.error('Upload cost calculation failed:', err)
        if (!cancelled) {
          setUploadCost(null)
        }
      }
    }

    calculateUploadCost()

    return () => {
      cancelled = true
    }
  }, [selectedFile, config.epochs, estimateStorageCost, walrusReady])

  React.useEffect(() => {
    if (selectedService === 'custom') {
      return
    }

    const service = WALRUS_SERVICES.find((svc) => svc.id === selectedService)
    if (service) {
      setConfig((prev) => ({
        ...prev,
        aggregatorUrl: service.aggregatorUrl,
        publisherUrl: service.publisherUrl
      }))
    }
  }, [selectedService])

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      setSelectedFile(e.target.files[0])
      setLastUploadResult(null)
    }
  }

  const handleUpload = async () => {
    if (!selectedFile) return

    // Check if wallet is connected
    if (!currentAccount) {
      error('Wallet not connected', 'Please connect your wallet to upload files')
      return
    }

    setIsUploading(true)
    setUploadProgress(10)

    try {
      let fileToUpload = selectedFile
      let encryptionMetadata: EncryptedFileMetadata | undefined

      if (enableEncryption) {
        if (!sealPolicyId) {
          throw new Error('Seal policy ID is required before encrypting files.')
        }

        info('Encrypting file...', 'Using Seal for secure encryption')
        setUploadProgress(15)
        await new Promise((resolve) => setTimeout(resolve, 0))

        const fileBuffer = await selectedFile.arrayBuffer()
        const encryptionConfig: EncryptionConfig = {
          enableEncryption: true,
          threshold: encryptionThreshold,
          packageId: sealPackageId,
          moduleName: sealModuleName,
          policyId: sealPolicyId,
          mvrName: sealMvrName
        }

        const { encryptedData, metadata } = await encryptFile(new Uint8Array(fileBuffer), encryptionConfig)

        encryptionMetadata = {
          ...metadata,
          originalFileName: selectedFile.name,
          originalMimeType: selectedFile.type
        }

        fileToUpload = new File([encryptedData], `${selectedFile.name}.sealed`, {
          type: 'application/octet-stream'
        })

        success(
          'File encrypted',
          `Using ${encryptionConfig.threshold}-of-${keyServers.length} Seal threshold`
        )
      }

      setUploadProgress(20)
      info(`Preparing ${selectedFile.name}...`, 'Encoding file for Walrus')

      // Prepare upload flow
      const uploadData = await uploadToWalrus(fileToUpload, config.epochs)

      // Execute the full upload flow with wallet signing
      const flowResult = (await executeUploadFlow(
        uploadData.flow,
        config.epochs,
        () => {
          setUploadProgress(50)
          info('Uploading to storage nodes...', 'This may take a moment')
        },
        () => {
          setUploadProgress(80)
          info('Certifying blob...', 'Please approve the final transaction')
        }
      )) as UploadFlowResult

      if (!flowResult.blobId) {
        throw new Error('Upload flow did not return a blob ID')
      }

      setUploadProgress(100)

      const blobId = flowResult.blobId
      if (!blobId) {
        throw new Error('Walrus flow did not return a blob ID')
      }

      const walrusFileInfo = flowResult.files?.[0]
      const blobObject = walrusFileInfo?.blobObject
      let expiryEpoch = extractEndEpoch(blobObject)
      let registeredEpoch = extractRegisteredEpoch(blobObject)
      let derivedName = walrusFileInfo?.identifier ?? selectedFile.name

      try {
        const blobInfo: any = await getBlobInfo(blobId)
        expiryEpoch = extractEndEpoch(blobInfo) ?? expiryEpoch
        registeredEpoch = extractRegisteredEpoch(blobInfo) ?? registeredEpoch
        derivedName = extractIdentifier(blobInfo?.files) ?? derivedName
      } catch (infoErr) {
        console.warn('Unable to refresh Walrus metadata', infoErr)
      }

      const newFile: StoredFile = {
        name: derivedName ?? selectedFile.name,
        size: selectedFile.size,
        blobId,
        uploadedAt: new Date().toLocaleString(),
        expiryEpoch,
        registeredEpoch,
        suiObjectId: blobObject?.id?.id,
        encryptionMetadata
      }
      setFiles((prev) => [newFile, ...prev])

      const aggregatorBase = config.aggregatorUrl.replace(/\/+$/, '')
      const publisherBase = config.publisherUrl.replace(/\/+$/, '')
      const walrusExplorerUrl = config.network === 'mainnet'
        ? `https://walruscan.com/blob/${blobId}`
        : `https://walruscan.com/testnet/blob/${blobId}`
      const suiExplorerBase = config.network === 'mainnet'
        ? 'https://suiscan.xyz/mainnet/object'
        : 'https://suiscan.xyz/testnet/object'

      setLastUploadResult({
        blobId,
        status: walrusFileInfo ? 'Stored in Walrus' : 'Stored',
        aggregatorUrl: aggregatorBase,
        publisherUrl: publisherBase,
        suiObjectId: blobObject?.id?.id,
        endEpoch: expiryEpoch,
        registeredEpoch,
        walrusExplorerUrl,
        suiExplorerUrl: blobObject?.id?.id ? `${suiExplorerBase}/${blobObject.id.id}` : undefined,
        encryptionEnabled: Boolean(encryptionMetadata?.isEncrypted)
      })

      // Update CLI index file
      try {
        await fetch('http://localhost:3002/api/index/update', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            fileName: derivedName ?? selectedFile.name,
            blobId,
            size: selectedFile.size,
            expiryEpoch: expiryEpoch || 0
          })
        })
      } catch (err) {
        console.warn('Failed to update CLI index:', err)
      }

      success(
        'File uploaded successfully!',
        `Blob ID: ${flowResult.blobId.substring(0, 20)}...`
      )

      setSelectedFile(null)
    } catch (err: any) {
      error('Upload failed', err.message || 'Failed to upload file to Walrus')
    } finally {
      setIsUploading(false)
      setUploadProgress(0)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
      .then(() => {
        setCopiedBlobId(text)
        success('Copied to clipboard', `Blob ID: ${text.substring(0, 12)}...`)
        setTimeout(() => setCopiedBlobId(null), 2000)
      })
      .catch(() => {
        error('Failed to copy', 'Could not copy to clipboard')
      })
  }

  // Remove unused variable

  // File selection handler - also trigger cost calculation

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 dark:from-neutral-950 dark:to-neutral-900">
      {/* Notification Container */}
      <NotificationContainer notifications={notifications} onRemove={removeNotification} />
      {/* Header */}
      <header className="border-b bg-white/50 dark:bg-slate-900/50 backdrop-blur-md sticky top-0 z-10">
        <div className="container mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-xl font-bold">YK Labs - Walrus Demo</h1>
              <p className="text-xs text-muted-foreground">Decentralized storage MVP with Seal encryption</p>
            </div>

            <div className="flex items-center gap-4">
              <WalletStatus networkName={config.network} />
              <ThemeToggle />
              <WalletButton />
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
          <TabsList className="grid w-full max-w-3xl mx-auto grid-cols-6">
            <TabsTrigger value="upload">Upload</TabsTrigger>
            <TabsTrigger value="files">Files</TabsTrigger>
            <TabsTrigger value="download">Download</TabsTrigger>
            <TabsTrigger value="s3">S3 Transfer</TabsTrigger>
            <TabsTrigger value="cost">Cost</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>

          {/* Upload Tab */}
          <TabsContent value="upload" className="space-y-4">
            <Card className="max-w-3xl mx-auto">
              <CardHeader>
                <CardTitle>Upload Files</CardTitle>
                <CardDescription>
                  Upload your files to Walrus decentralized storage
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-1">
                  <label className="text-sm font-medium">Walrus service</label>
                  <select
                    className="w-full h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm"
                    value={selectedService}
                    onChange={(e) => setSelectedService(e.target.value)}
                  >
                    {WALRUS_SERVICES.map((service) => (
                      <option key={service.id} value={service.id}>
                        {service.name}
                      </option>
                    ))}
                  </select>
                  <p className="text-xs text-muted-foreground">
                    {selectedService === 'custom'
                      ? 'Using custom endpoints configured in Settings → Network.'
                      : 'Using Walrus testnet endpoints. Change them in Settings if needed.'}
                  </p>
                </div>

                <div className="border-2 border-dashed rounded-lg p-8 text-center hover:border-primary/50 transition-colors">
                  <input
                    type="file"
                    id="file-upload"
                    className="hidden"
                    onChange={handleFileSelect}
                    disabled={isUploading}
                  />
                  <label htmlFor="file-upload" className="cursor-pointer">
                    <Upload className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
                    <p className="text-sm text-muted-foreground">
                      Click to browse or drag and drop your files
                    </p>
                  </label>
                </div>

                {selectedFile && (
                  <>
                    <div className="bg-muted rounded-lg p-4 flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <FileText className="w-5 h-5 text-muted-foreground" />
                        <div>
                          <p className="font-medium text-sm">{selectedFile.name}</p>
                          <p className="text-xs text-muted-foreground">
                            {formatBytes(selectedFile.size)}
                          </p>
                          {enableEncryption && (
                            <span className="mt-1 inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-medium text-blue-700">
                              <Shield className="w-3 h-3" />
                              {`${encryptionThreshold}-of-${keyServerCount} Seal threshold`}
                            </span>
                          )}
                        </div>
                      </div>
                      <div className="text-right">
                        <p className="text-xs text-muted-foreground">Storage cost</p>
                        {uploadCost ? (
                          <div>
                            <p className="font-mono text-sm font-semibold">
                              {formatWAL(uploadCost.total)} WAL
                            </p>
                            <p className="text-xs text-gray-500">
                              ≈ ${(uploadCost.total * walPriceUSD).toFixed(4)} USD
                            </p>
                          </div>
                        ) : (
                          <p className="font-mono text-sm">Calculating...</p>
                        )}
                      </div>
                    </div>

                    {/* Encryption Options */}
                    <div className="border rounded-lg p-4 space-y-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Shield className="w-4 h-4 text-blue-600" />
                          <span className="font-medium text-sm">Seal Encryption</span>
                        </div>
                        <label className="relative inline-flex items-center cursor-pointer">
                          <input
                            type="checkbox"
                            className="sr-only"
                            checked={enableEncryption}
                            onChange={(e) => setEnableEncryption(e.target.checked)}
                            disabled={isUploading || isEncrypting}
                          />
                          <div className={`w-11 h-6 rounded-full transition-colors ${
                            enableEncryption ? 'bg-blue-600' : 'bg-gray-200'
                          }`}>
                            <div className={`w-5 h-5 bg-white rounded-full shadow transform transition-transform ${
                              enableEncryption ? 'translate-x-5' : 'translate-x-0.5'
                            } mt-0.5`} />
                          </div>
                        </label>
                      </div>

                      {enableEncryption && (
                        <div className="space-y-2 pl-6">
                          <div className="flex items-center gap-2 text-xs text-gray-600">
                            <Lock className="w-3 h-3" />
                            <span>End-to-end encrypted before storage</span>
                          </div>
                          {isEncrypting && (
                            <div className="flex items-center gap-2 text-xs text-blue-600">
                              <Loader2 className="w-3 h-3 animate-spin" />
                              <span>Encrypting via Seal…</span>
                            </div>
                          )}
                          <div className="flex items-center gap-2">
                            <label className="text-xs text-gray-600">Threshold:</label>
                            <select
                              className="h-7 rounded border border-input bg-transparent px-2 text-xs"
                              value={encryptionThreshold}
                              onChange={(e) => setEncryptionThreshold(Number(e.target.value))}
                              disabled={!enableEncryption || isUploading || isEncrypting}
                            >
                              {Array.from({ length: Math.max(1, keyServerCount) }, (_, idx) => idx + 1).map((option) => (
                                <option key={option} value={option}>
                                  {option}-of-{keyServerCount}
                                </option>
                              ))}
                            </select>
                          </div>
                          {allowlists.length > 0 && (
                            <div className="space-y-1">
                              <label className="text-xs text-gray-600">Policy from allowlist</label>
                              <select
                                className="h-7 rounded border border-input bg-transparent px-2 text-xs"
                                value={allowlists.some((a) => a.id === sealPolicyId) ? sealPolicyId : ''}
                                onChange={(e) => {
                                  const value = e.target.value
                                  if (value) {
                                    setSealPolicyId(value)
                                  }
                                }}
                              >
                                <option value="">Manual entry</option>
                                {allowlists.map((allowlist) => (
                                  <option key={allowlist.id} value={allowlist.id}>
                                    {allowlist.name} ({truncateObjectId(allowlist.id)})
                                  </option>
                                ))}
                              </select>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 text-xs"
                                onClick={async () => { await refreshAllowlists() }}
                                disabled={allowlistsLoading || isEncrypting}
                              >
                                {allowlistsLoading ? (
                                  <>
                                    <Loader2 className="w-3 h-3 mr-2 animate-spin" />Refreshing…
                                  </>
                                ) : (
                                  <>
                                    <RefreshCw className="w-3 h-3 mr-2" />Refresh allowlists
                                  </>
                                )}
                              </Button>
                            </div>
                          )}
                          <div className="text-xs text-gray-500">
                            Key servers: {keyServers.map((server) => truncateObjectId(server.objectId)).join(', ')}
                          </div>
                          {sealPolicyId === '' && (
                            <div className="text-xs text-red-500">
                              Configure a Seal policy ID in Settings before uploading encrypted files.
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </>
                )}

                {lastUploadResult && (
                  <Card className="border-primary/40">
                    <CardHeader>
                      <CardTitle className="text-base">Latest upload summary</CardTitle>
                      <CardDescription>Details from the Walrus publisher response</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3 text-sm">
                      <div className="flex items-center justify-between">
                        <span className="text-muted-foreground">Blob ID</span>
                        <div className="flex items-center gap-2">
                          <code className="font-mono text-xs">{lastUploadResult.blobId}</code>
                          <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => copyToClipboard(lastUploadResult.blobId)}>
                            {copiedBlobId === lastUploadResult.blobId ? (
                              <Check className="w-3 h-3 text-green-500" />
                            ) : (
                              <Copy className="w-3 h-3" />
                            )}
                          </Button>
                        </div>
                      </div>
                      <div className="grid md:grid-cols-3 gap-2">
                        <div>
                          <p className="text-muted-foreground text-xs uppercase tracking-wide">Status</p>
                          <p className="text-xs">{lastUploadResult.status}</p>
                        </div>
                        <div>
                          <p className="text-muted-foreground text-xs uppercase tracking-wide">Aggregator</p>
                          <p className="font-mono text-xs break-all">{lastUploadResult.aggregatorUrl || '—'}</p>
                        </div>
                        <div>
                          <p className="text-muted-foreground text-xs uppercase tracking-wide">Publisher</p>
                          <p className="font-mono text-xs break-all">{lastUploadResult.publisherUrl || '—'}</p>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
                        {lastUploadResult.registeredEpoch !== undefined && (
                          <span>Registered epoch: {lastUploadResult.registeredEpoch}</span>
                        )}
                        {lastUploadResult.endEpoch !== undefined && (
                          <span>Expires epoch: {lastUploadResult.endEpoch}</span>
                        )}
                        <span>
                          Encryption: {lastUploadResult.encryptionEnabled ? 'Seal (enabled)' : 'Disabled'}
                        </span>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {lastUploadResult.walrusExplorerUrl && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => window.open(lastUploadResult.walrusExplorerUrl, '_blank')}
                          >
                            <ExternalLink className="w-4 h-4 mr-1" />
                            Walruscan
                          </Button>
                        )}
                        {lastUploadResult.suiExplorerUrl && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => window.open(lastUploadResult.suiExplorerUrl, '_blank')}
                          >
                            <ExternalLink className="w-4 h-4 mr-1" />
                            Sui explorer
                          </Button>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                )}

                {isUploading && (
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span>Uploading...</span>
                      <span>{uploadProgress}%</span>
                    </div>
                    <Progress value={uploadProgress} className="h-2" />
                  </div>
                )}

                <div className="flex justify-between items-center">
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <AlertCircle className="w-4 h-4" />
                    <span>Storage duration: {config.epochs} epochs</span>
                  </div>
                  <Button
                    onClick={handleUpload}
                    disabled={!selectedFile || isUploading}
                  >
                    {isUploading ? (
                      <>
                        <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                        Uploading...
                      </>
                    ) : (
                      <>
                        <Upload className="w-4 h-4 mr-2" />
                        Upload File
                      </>
                    )}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Files Tab */}
          <TabsContent value="files" className="space-y-4">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div>
                    <CardTitle>Stored Files</CardTitle>
                    <CardDescription>
                      Manage your files stored on Walrus
                    </CardDescription>
                  </div>
                  <div className="flex gap-2">
                    <div className="relative">
                      <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                      <Input
                        placeholder="Search files..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9 w-64"
                      />
                    </div>
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={async () => {
                        info('Refreshing files...', 'Fetching latest metadata from Walrus')
                        try {
                          const updated = await Promise.all(
                            files.map(async (file) => {
                              try {
                                const blobInfo = await getBlobInfo(file.blobId)
                                return {
                                  ...file,
                                  expiryEpoch: extractEndEpoch(blobInfo) ?? file.expiryEpoch,
                                  registeredEpoch:
                                    extractRegisteredEpoch(blobInfo) ?? file.registeredEpoch
                                }
                              } catch (refreshErr) {
                                console.warn('Unable to refresh blob', refreshErr)
                                return file
                              }
                            })
                          )
                          setFiles(updated)
                          success('Files refreshed', 'Metadata synced from Walrus')
                        } catch (refreshError: any) {
                          error('Refresh failed', refreshError.message || 'Unable to refresh files')
                        }
                      }}
                    >
                      <RefreshCw className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {allFiles.filter(file =>
                    file.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                    file.blobId.toLowerCase().includes(searchQuery.toLowerCase())
                  ).length === 0 ? (
                    <div className="text-center py-12 text-muted-foreground">
                      <HardDrive className="w-12 h-12 mx-auto mb-4 opacity-50" />
                      <p>No files found</p>
                    </div>
                  ) : (
                    allFiles.filter(file =>
                      file.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                      file.blobId.toLowerCase().includes(searchQuery.toLowerCase())
                    ).map((file) => (
                      <div
                        key={file.blobId}
                        className="border rounded-lg p-4 hover:bg-muted/50 transition-colors"
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-3">
                            <FileText className="w-5 h-5 text-muted-foreground" />
                            <div>
                              <p className="font-medium">{file.name}</p>
                              <div className="flex items-center gap-4 text-xs text-muted-foreground">
                                <span>{formatBytes(file.size)}</span>
                                <span>Uploaded {file.uploadedAt}</span>
                                {file.source === 's3' && (
                                  <span className="inline-flex items-center rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-medium text-blue-800">
                                    S3 Transfer
                                  </span>
                                )}
                                <span>
                                  Expires:{' '}
                                  {file.expiryEpoch !== undefined ? `Epoch ${file.expiryEpoch}` : 'Unknown'}
                                </span>
                                {file.registeredEpoch !== undefined && (
                                  <span>Registered: Epoch {file.registeredEpoch}</span>
                                )}
                              </div>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            <div className="text-right mr-4">
                              <p className="text-xs text-muted-foreground">Blob ID</p>
                              <div className="flex items-center gap-1">
                                <code className="text-xs font-mono">
                                  {file.blobId.substring(0, 12)}...
                                </code>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-6 w-6"
                                  onClick={() => copyToClipboard(file.blobId)}
                                >
                                  {copiedBlobId === file.blobId ? (
                                    <Check className="w-3 h-3 text-green-500" />
                                  ) : (
                                    <Copy className="w-3 h-3" />
                                  )}
                                </Button>
                              </div>
                              {file.suiObjectId && (
                                <div className="mt-1 text-[10px] text-muted-foreground">
                                  Sui Object: <code>{truncateObjectId(file.suiObjectId)}</code>
                                </div>
                              )}
                            </div>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => window.open(`https://walruscan.com/testnet/blob/${file.blobId}`, '_blank')}
                            >
                              <ExternalLink className="w-4 h-4 mr-1" />
                              Walruscan
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={sdkDownloading}
                              onClick={async () => {
                                info(`Downloading ${file.name}...`, 'Retrieving file from Walrus network')
                                try {
                                  const originalName = file.encryptionMetadata?.originalFileName ?? file.name
                                  let blob = await downloadFromWalrus(file.blobId)
                                  let downloadName = (blob as any).name || file.name

                                  if (file.encryptionMetadata?.isEncrypted && currentAccount) {
                                    info('Decrypting file...', 'Using Seal to decrypt')

                                    const encryptedData = new Uint8Array(await blob.arrayBuffer())
                                    const decryptedData = await decryptFile(
                                      encryptedData,
                                      file.encryptionMetadata,
                                      currentAccount.address
                                    )

                                    const mimeType =
                                      file.encryptionMetadata?.originalMimeType ?? 'application/octet-stream'

                                    blob = new Blob([decryptedData], { type: mimeType })
                                    downloadName = originalName
                                    success('File decrypted', 'Seal decryption successful')
                                  } else if (file.encryptionMetadata?.isEncrypted && !currentAccount) {
                                    warning('Wallet required', 'Connect an authorized wallet to decrypt this file')
                                  }

                                  if (downloadName.endsWith('.sealed')) {
                                    downloadName = downloadName.replace(/\.sealed$/, '')
                                  }

                                  // Create download link
                                  const url = URL.createObjectURL(blob)
                                  const a = document.createElement('a')
                                  a.href = url
                                  a.download = downloadName
                                  document.body.appendChild(a)
                                  a.click()
                                  document.body.removeChild(a)
                                  URL.revokeObjectURL(url)

                                  success('Download complete', `${downloadName} has been downloaded`)
                                } catch (err: any) {
                                  error('Download failed', err.message || 'Failed to download file from Walrus')
                                }
                              }}
                            >
                              <Download className="w-4 h-4 mr-1" />
                              Download
                            </Button>
                          </div>
                          {file.encryptionMetadata?.isEncrypted && (
                            <div className="flex items-center gap-2 mt-2">
                              <Shield className="w-4 h-4 text-blue-600" />
                              <span className="text-xs text-blue-600">
                                Encrypted ({file.encryptionMetadata.threshold ?? '?'}-of-
                                {file.encryptionMetadata.keyServers?.length ?? keyServers.length} threshold)
                              </span>
                            </div>
                          )}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Download Tab */}
          <TabsContent value="download" className="space-y-4">
            <Card className="max-w-3xl mx-auto">
              <CardHeader>
                <CardTitle>Download File</CardTitle>
                <CardDescription>
                  Download files from Walrus using blob ID
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Blob ID</label>
                  <Input
                    id="download-blob-id"
                    placeholder="Enter the blob ID of the file to download"
                    className="font-mono"
                  />
                </div>
                <Button
                  className="w-full"
                  disabled={sdkDownloading}
                  onClick={async () => {
                    const input = document.getElementById('download-blob-id') as HTMLInputElement
                    if (!input?.value) {
                      warning('Missing blob ID', 'Please enter a valid blob ID to download')
                      return
                    }
                    info('Starting download...', `Fetching blob ${input.value.substring(0, 12)}...`)
                    try {
                      // Try to get blob info for filename
                      let fileName = `walrus-${input.value.substring(0, 12)}.bin`
                      try {
                        const blobInfo = await getBlobInfo(input.value)
                        if (blobInfo.files?.length > 0 && blobInfo.files[0].identifier) {
                          fileName = blobInfo.files[0].identifier
                        }
                      } catch {
                        // Use default filename if info fetch fails
                      }

                      let blob = await downloadFromWalrus(input.value)

                      // Check if blob is encrypted (has .sealed extension or metadata)
                      const isEncrypted = fileName.endsWith('.sealed')
                      if (isEncrypted && currentAccount) {
                        warning('Encrypted file detected', 'Note: Cannot auto-decrypt without metadata. File will be downloaded encrypted.')
                      }

                      // Use filename from blob if available
                      const downloadName = (blob as any).name || fileName

                      // Create download link
                      const url = URL.createObjectURL(blob)
                      const a = document.createElement('a')
                      a.href = url
                      a.download = downloadName
                      document.body.appendChild(a)
                      a.click()
                      document.body.removeChild(a)
                      URL.revokeObjectURL(url)

                      success('Download complete', `${downloadName} has been downloaded`)
                    } catch (err: any) {
                      error('Download failed', err.message || 'Failed to download file from Walrus')
                    }
                  }}
                >
                  <Download className="w-4 h-4 mr-2" />
                  Download File
                </Button>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Cost Calculator Tab */}
          <TabsContent value="cost" className="space-y-4">
            <Card className="max-w-2xl mx-auto">
              <CardHeader>
                <CardTitle>Storage Cost Calculator</CardTitle>
                <CardDescription>
                  Estimate storage costs for your files
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm font-medium">File Size (MB)</label>
                    <Input
                      type="number"
                      placeholder="100"
                      value={calculatorSize}
                      onChange={(e) => setCalculatorSize(Number(e.target.value) || 0)}
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Storage Duration (Epochs)</label>
                    <Input
                      type="number"
                      placeholder="5"
                      value={calculatorEpochs}
                      onChange={(e) => setCalculatorEpochs(Number(e.target.value) || 1)}
                    />
                  </div>
                </div>

                <div className="bg-muted rounded-lg p-6 text-center">
                  <p className="text-sm text-muted-foreground mb-2">Estimated Cost</p>
                  {calculatorCost ? (
                    <>
                      <p className="text-3xl font-bold font-mono text-blue-600">
                        {formatWAL(calculatorCost.total)} WAL
                      </p>
                      <p className="text-sm text-gray-600 mt-1">
                        ≈ ${(calculatorCost.total * walPriceUSD).toFixed(calculatorCost.total * walPriceUSD >= 0.01 ? 2 : 4)} USD
                      </p>
                      <div className="mt-4 pt-3 border-t text-xs text-left space-y-1">
                        <p>Storage: {formatWAL(calculatorCost.storage)} WAL</p>
                        <p>Write fee: {formatWAL(calculatorCost.write)} WAL</p>
                        <p className="text-muted-foreground pt-1">{calculatorSize} MB × {calculatorEpochs} epochs</p>
                      </div>
                    </>
                  ) : (
                    <p className="text-xl text-gray-400">
                      {calculatorSize > 0 && calculatorEpochs > 0 ? 'Calculating...' : 'Enter size and epochs'}
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* S3 Transfer Tab */}
          <TabsContent value="s3" className="space-y-4">
            <S3Transfer
              walrusConfig={{
                aggregatorUrl: config.aggregatorUrl,
                publisherUrl: config.publisherUrl,
                epochs: config.epochs
              }}
              onTransferComplete={(results) => {
                const successCount = results.filter(r => r.success).length
                if (successCount > 0) {
                  success('S3 Transfer Complete', `${successCount} files transferred successfully`)
                }
              }}
            />
          </TabsContent>

          {/* Settings Tab */}
          <TabsContent value="settings" className="space-y-4">
            <Card className="max-w-2xl mx-auto">
              <CardHeader>
                <CardTitle>Network Configuration</CardTitle>
                <CardDescription>
                  Configure your Walrus network settings
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Network</label>
                  <select className="w-full h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm">
                    <option value="testnet">Testnet (Free)</option>
                    <option value="mainnet">Mainnet</option>
                    <option value="custom">Custom</option>
                  </select>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Aggregator URL</label>
                  <Input
                    value={config.aggregatorUrl}
                    onChange={(e) => {
                      setConfig(prev => ({ ...prev, aggregatorUrl: e.target.value }))
                      setSelectedService('custom')
                    }}
                    placeholder="Enter aggregator URL"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Publisher URL</label>
                  <Input
                    value={config.publisherUrl}
                    onChange={(e) => {
                      setConfig(prev => ({ ...prev, publisherUrl: e.target.value }))
                      setSelectedService('custom')
                    }}
                    placeholder="Enter publisher URL"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Default Storage Duration</label>
                  <select
                    className="w-full h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm"
                    value={config.epochs}
                    onChange={(e) => {
                      const newEpochs = Number(e.target.value)
                      setConfig(prev => ({ ...prev, epochs: newEpochs }))
                      setCalculatorEpochs(newEpochs)
                      success('Settings updated', `Default storage duration set to ${newEpochs} epochs`)
                    }}
                  >
                    <option value="1">1 epoch (~24 hours)</option>
                    <option value="5">5 epochs (~5 days)</option>
                    <option value="10">10 epochs (~10 days)</option>
                    <option value="30">30 epochs (~30 days)</option>
                  </select>
                </div>
              </CardContent>
            </Card>

            <Card className="max-w-2xl mx-auto">
              <CardHeader>
                <CardTitle>Seal Configuration</CardTitle>
                <CardDescription>
                  Configure Seal policy details used for encryption and decryption
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Seal Package ID</label>
                  <Input
                    value={sealPackageId}
                    onChange={(e) => setSealPackageId(e.target.value.trim())}
                    placeholder="0xc5ce..."
                  />
                  <p className="text-xs text-muted-foreground">
                    Default: {DEFAULT_SEAL_PACKAGE_ID.slice(0, 16)}…
                  </p>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Module Name</label>
                  <Input
                    value={sealModuleName}
                    onChange={(e) => setSealModuleName(e.target.value.trim())}
                    placeholder="allowlist"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Policy Object ID</label>
                  <Input
                    value={sealPolicyId}
                    onChange={(e) => setSealPolicyId(e.target.value.trim())}
                    placeholder="Paste allowlist or policy object ID"
                  />
                  <p className="text-xs text-muted-foreground">
                    Required to upload encrypted files. Create an allowlist using the Seal example or your own Move
                    package, then paste the resulting object ID here.
                  </p>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">MVR Name</label>
                  <Input
                    value={sealMvrName}
                    onChange={(e) => setSealMvrName(e.target.value.trim())}
                    placeholder="@pkg/seal-demo-1234"
                  />
                  <p className="text-xs text-muted-foreground">Seal Move Verification Registry name (defaults to the official testnet deployment).</p>
                </div>

                <div className="flex justify-end">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setSealPackageId(DEFAULT_SEAL_PACKAGE_ID)
                      setSealModuleName(DEFAULT_SEAL_MODULE_NAME)
                      setSealMvrName(DEFAULT_SEAL_MVR_NAME)
                      success('Seal defaults restored', 'Reverted to official testnet configuration')
                      refreshAllowlists()
                    }}
                  >
                    Reset to defaults
                  </Button>
                </div>

                <SealAllowlistPanel
                  packageId={sealPackageId}
                  selectedAllowlistId={sealPolicyId}
                  onSelectAllowlist={(allowlist) => {
                    setSealPolicyId(allowlist.id)
                  }}
                  onAllowlistsChanged={refreshAllowlists}
                />
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </main>
    </div>
  )
}

function App() {
  return (
    <S3Provider>
      <AppContent />
    </S3Provider>
  )
}

export default App
