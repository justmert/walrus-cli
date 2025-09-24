import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Cloud,
  Upload,
  FolderOpen,
  FileText,
  AlertCircle,
  Loader2,
  CheckCircle,
  XCircle,
  RefreshCw,
  Filter,
  Shield,
  Eye,
  EyeOff,
  ExternalLink,
  Copy
} from 'lucide-react'
import { useS3Transfer } from '@/hooks/useS3Transfer'
import { formatBytes, formatWAL } from '@/lib/utils'
import { Checkbox } from '@/components/ui/checkbox'
import { useS3Context } from '@/contexts/S3Context'
import { useNotification } from '@/hooks/useNotification'

interface S3TransferProps {
  walrusConfig: {
    aggregatorUrl: string
    publisherUrl: string
    epochs: number
  }
  onTransferComplete?: (results: any[]) => void
}

export function S3Transfer({ walrusConfig }: S3TransferProps) {
  const { success, info, warning } = useNotification()
  const {
    credentials,
    setCredentials,
    isS3Connected,
    setIsS3Connected,
    addS3TransferredFile
  } = useS3Context()

  const {
    buckets,
    objects,
    selectedBucket,
    setSelectedBucket,
    selectedObjects,
    setSelectedObjects,
    filters,
    setFilters,
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
    error: s3Error
  } = useS3Transfer(walrusConfig, credentials, isS3Connected, setIsS3Connected, addS3TransferredFile)

  const [showSecretKey, setShowSecretKey] = useState(false)
  const [activeTab, setActiveTab] = useState('connect')

  // Auto-reconnect on page load if credentials are persisted and marked as connected
  useEffect(() => {
    const autoReconnect = async () => {
      if (isS3Connected && credentials.accessKeyId && credentials.secretAccessKey && buckets.length === 0) {
        try {
          await connect()
          // Only switch to browse tab if connection succeeds
          setActiveTab('browse')
        } catch (error) {
          // If auto-reconnect fails, reset connection state
          console.warn('Auto-reconnect failed:', error)
          setIsS3Connected(false)
          setActiveTab('connect')
        }
      }
    }

    autoReconnect()
  }, [isS3Connected, credentials.accessKeyId, credentials.secretAccessKey, buckets.length, connect, setIsS3Connected])


  const handleConnect = async () => {
    info('Connecting to S3...', 'Validating AWS credentials')
    const connectionSuccess = await connect()
    if (connectionSuccess) {
      success('Connected to S3', `Connected to AWS region: ${credentials.region}`)
    }
  }

  const handleBucketSelect = async (bucket: string) => {
    setSelectedBucket(bucket)
    info(`Loading objects from ${bucket}...`, 'Fetching bucket contents')
    await listObjects(bucket)
  }

  const handleObjectToggle = (key: string) => {
    setSelectedObjects(prev => {
      const newSet = new Set(prev)
      if (newSet.has(key)) {
        newSet.delete(key)
      } else {
        newSet.add(key)
      }
      return newSet
    })
  }

  const handleSelectAll = () => {
    if (selectedObjects.size === objects.length) {
      setSelectedObjects(new Set())
    } else {
      setSelectedObjects(new Set(objects.map(obj => obj.key)))
    }
  }

  const selectedObjectsArray = objects.filter(obj => selectedObjects.has(obj.key))
  const totalSize = selectedObjectsArray.reduce((acc, obj) => acc + (obj.size || 0), 0)
  const estimatedCost = totalSize > 0 ? estimateCost(totalSize) : 0

  return (
    <div className="space-y-6">
      {/* Progress Indicator */}
      <div className="flex items-center justify-center space-x-2 text-sm text-muted-foreground">
        <div className={`flex items-center space-x-2 ${isS3Connected ? 'text-green-600' : activeTab === 'connect' ? 'text-primary' : ''}`}>
          <div className={`w-2 h-2 rounded-full ${isS3Connected ? 'bg-green-600' : activeTab === 'connect' ? 'bg-primary' : 'bg-muted'}`} />
          <span>Connect</span>
        </div>
        <div className="w-8 h-px bg-border" />
        <div className={`flex items-center space-x-2 ${selectedBucket ? 'text-green-600' : activeTab === 'browse' ? 'text-primary' : !isS3Connected ? 'text-muted-foreground' : ''}`}>
          <div className={`w-2 h-2 rounded-full ${selectedBucket ? 'bg-green-600' : activeTab === 'browse' ? 'bg-primary' : 'bg-muted'}`} />
          <span>Browse</span>
        </div>
        <div className="w-8 h-px bg-border" />
        <div className={`flex items-center space-x-2 ${selectedObjects.size > 0 ? 'text-green-600' : activeTab === 'transfer' ? 'text-primary' : 'text-muted-foreground'}`}>
          <div className={`w-2 h-2 rounded-full ${selectedObjects.size > 0 ? 'bg-green-600' : activeTab === 'transfer' ? 'bg-primary' : 'bg-muted'}`} />
          <span>Transfer</span>
        </div>
        <div className="w-8 h-px bg-border" />
        <div className={`flex items-center space-x-2 ${transferResults.length > 0 ? 'text-green-600' : activeTab === 'results' ? 'text-primary' : 'text-muted-foreground'}`}>
          <div className={`w-2 h-2 rounded-full ${transferResults.length > 0 ? 'bg-green-600' : activeTab === 'results' ? 'bg-primary' : 'bg-muted'}`} />
          <span>Results</span>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid w-full grid-cols-4">
          <TabsTrigger value="connect">Connect</TabsTrigger>
          <TabsTrigger value="browse" disabled={!isS3Connected}>Browse</TabsTrigger>
          <TabsTrigger value="transfer" disabled={!isS3Connected || selectedObjects.size === 0}>Transfer</TabsTrigger>
          <TabsTrigger value="results" disabled={transferResults.length === 0}>Results</TabsTrigger>
        </TabsList>

        <TabsContent value="connect">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Cloud className="w-5 h-5" />
                AWS S3 Connection
              </CardTitle>
              <CardDescription>
                Enter your AWS credentials to connect to S3
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {isS3Connected && (
                <Alert>
                  <CheckCircle className="w-4 h-4 text-green-600" />
                  <AlertDescription>
                    Connected to AWS S3 in {credentials.region}
                  </AlertDescription>
                </Alert>
              )}

              <div className="space-y-2">
                <label className="text-sm font-medium">Access Key ID</label>
                <Input
                  placeholder="AKIA..."
                  value={credentials.accessKeyId}
                  onChange={(e) => setCredentials({ ...credentials, accessKeyId: e.target.value })}
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium">Secret Access Key</label>
                <div className="relative">
                  <Input
                    type={showSecretKey ? "text" : "password"}
                    placeholder="Enter your secret key"
                    value={credentials.secretAccessKey}
                    onChange={(e) => setCredentials({ ...credentials, secretAccessKey: e.target.value })}
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    className="absolute right-1 top-1 h-7 w-7"
                    onClick={() => setShowSecretKey(!showSecretKey)}
                  >
                    {showSecretKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </Button>
                </div>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium">Region</label>
                <Input
                  placeholder="us-east-1"
                  value={credentials.region}
                  onChange={(e) => setCredentials({ ...credentials, region: e.target.value })}
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium">Session Token (Optional)</label>
                <Input
                  placeholder="For temporary credentials"
                  value={credentials.sessionToken}
                  onChange={(e) => setCredentials({ ...credentials, sessionToken: e.target.value })}
                />
              </div>

              {s3Error && (
                <Alert variant="destructive">
                  <AlertCircle className="w-4 h-4" />
                  <AlertDescription>{s3Error}</AlertDescription>
                </Alert>
              )}

              <div className="flex gap-2">
                {isS3Connected ? (
                  <>
                    <Button
                      variant="outline"
                      className="flex-1"
                      onClick={disconnect}
                    >
                      <Cloud className="w-4 h-4 mr-2" />
                      Disconnect
                    </Button>
                    <Button
                      className="flex-1"
                      onClick={() => setActiveTab('browse')}
                    >
                      Browse Files
                    </Button>
                  </>
                ) : (
                  <Button
                    className="w-full"
                    onClick={handleConnect}
                    disabled={!credentials.accessKeyId || !credentials.secretAccessKey}
                  >
                    <Cloud className="w-4 h-4 mr-2" />
                    Connect to S3
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="browse">
          <div className="grid gap-6 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span className="flex items-center gap-2">
                    <FolderOpen className="w-4 h-4" />
                    Buckets
                  </span>
                  <Button
                    size="icon"
                    variant="ghost"
                    onClick={listBuckets}
                    disabled={isLoadingBuckets}
                  >
                    {isLoadingBuckets ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <RefreshCw className="w-4 h-4" />
                    )}
                  </Button>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {buckets.map((bucket) => (
                    <div
                      key={bucket}
                      className={`p-3 rounded-lg border cursor-pointer transition-colors ${
                        selectedBucket === bucket
                          ? 'bg-primary/10 border-primary'
                          : 'hover:bg-muted'
                      }`}
                      onClick={() => handleBucketSelect(bucket)}
                    >
                      <div className="flex items-center gap-2">
                        <FolderOpen className="w-4 h-4" />
                        <span className="font-medium">{bucket}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <span className="flex items-center gap-2">
                    <FileText className="w-4 h-4" />
                    Objects {selectedBucket && `in ${selectedBucket}`}
                  </span>
                  {selectedBucket && (
                    <div className="flex items-center gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={handleSelectAll}
                      >
                        {selectedObjects.size === objects.length ? 'Deselect All' : 'Select All'}
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        onClick={() => listObjects(selectedBucket)}
                        disabled={isLoadingObjects}
                      >
                        {isLoadingObjects ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <RefreshCw className="w-4 h-4" />
                        )}
                      </Button>
                    </div>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {selectedBucket ? (
                  <div className="space-y-2">
                    <div className="flex gap-2 mb-4">
                      <Input
                        placeholder="Filter by prefix..."
                        value={filters.prefix}
                        onChange={(e) => setFilters({ ...filters, prefix: e.target.value })}
                        className="flex-1"
                      />
                      <Button
                        size="icon"
                        variant="outline"
                        onClick={() => listObjects(selectedBucket)}
                      >
                        <Filter className="w-4 h-4" />
                      </Button>
                    </div>

                    <div className="space-y-2 max-h-80 overflow-y-auto">
                      {objects
                        .filter(obj => !filters.prefix || obj.key.startsWith(filters.prefix))
                        .map((object) => (
                          <div
                            key={object.key}
                            className="p-3 rounded-lg border hover:bg-muted"
                          >
                            <div className="flex items-start gap-3">
                              <Checkbox
                                checked={selectedObjects.has(object.key)}
                                onCheckedChange={() => handleObjectToggle(object.key)}
                              />
                              <div className="flex-1 min-w-0">
                                <p className="font-medium text-sm truncate">{object.key}</p>
                                <div className="flex items-center gap-4 text-xs text-muted-foreground">
                                  <span>{formatBytes(object.size || 0)}</span>
                                  <span>{new Date(object.lastModified).toLocaleDateString()}</span>
                                </div>
                              </div>
                            </div>
                          </div>
                        ))}
                    </div>

                    {selectedObjects.size > 0 && (
                      <div className="mt-4 p-4 bg-primary/10 rounded-lg border-2 border-primary/20">
                        <div className="flex items-center justify-between">
                          <div className="text-sm">
                            <p className="font-medium">{selectedObjects.size} objects selected</p>
                            <p className="text-muted-foreground">{formatBytes(totalSize)} total</p>
                          </div>
                          <Button
                            onClick={() => setActiveTab('transfer')}
                            className="bg-primary hover:bg-primary/90"
                          >
                            Configure Transfer →
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-center text-muted-foreground py-8">
                    Select a bucket to view objects
                  </p>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="transfer">
          <Card>
            <CardHeader>
              <CardTitle>Transfer Configuration</CardTitle>
              <CardDescription>
                Configure and start the transfer from S3 to Walrus
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Selected Files</label>
                  <div className="p-4 bg-muted rounded-lg">
                    <p className="text-2xl font-bold">{selectedObjects.size}</p>
                    <p className="text-sm text-muted-foreground">Total: {formatBytes(totalSize)}</p>
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">Estimated Cost</label>
                  <div className="p-4 bg-muted rounded-lg">
                    <p className="text-2xl font-bold">{formatWAL(estimatedCost)} WAL</p>
                    <p className="text-sm text-muted-foreground">
                      {walrusConfig.epochs} epochs
                    </p>
                  </div>
                </div>
              </div>

              <div className="space-y-4">
                <div className="flex items-center space-x-2">
                  <Checkbox id="encrypt" />
                  <label
                    htmlFor="encrypt"
                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                  >
                    <Shield className="w-4 h-4 inline mr-2" />
                    Enable Seal encryption for transferred files
                  </label>
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox id="parallel" defaultChecked />
                  <label
                    htmlFor="parallel"
                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                  >
                    Use parallel transfers (up to 5 concurrent)
                  </label>
                </div>
              </div>

              {transferProgress && (
                <div className="space-y-2">
                  <div className="flex justify-between text-sm">
                    <span>Transfer Progress</span>
                    <span>{transferProgress.completed}/{transferProgress.total}</span>
                  </div>
                  <Progress value={(transferProgress.completed / transferProgress.total) * 100} />
                  {transferProgress.currentFile && (
                    <p className="text-xs text-muted-foreground">
                      Transferring: {transferProgress.currentFile}
                    </p>
                  )}
                </div>
              )}

              <div className="flex justify-center gap-4">
                <Button
                  className="w-full max-w-md"
                  size="lg"
                  onClick={async () => {
                    info('Starting S3 to Walrus transfer...', `Transferring ${selectedObjects.size} files`)
                    await startTransfer(Array.from(selectedObjects))
                    if (transferResults.some(r => r.success)) {
                      success('Transfer completed!', 'Files successfully transferred to Walrus')
                    }
                    if (transferResults.some(r => !r.success)) {
                      warning('Some transfers failed', 'Check results for details')
                    }
                  }}
                  disabled={isTransferring || selectedObjects.size === 0}
                >
                  {isTransferring ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      Transferring {selectedObjects.size} files...
                    </>
                  ) : (
                    <>
                      <Upload className="w-4 h-4 mr-2" />
                      Transfer {selectedObjects.size} Files to Walrus
                    </>
                  )}
                </Button>
              </div>

              {transferResults.length > 0 && !isTransferring && (
                <div className="mt-4 text-center">
                  <Button
                    variant="outline"
                    onClick={() => setActiveTab('results')}
                    className="bg-green-50 border-green-200 hover:bg-green-100"
                  >
                    <CheckCircle className="w-4 h-4 mr-2 text-green-600" />
                    See Transfer Results ({transferResults.filter(r => r.success).length}/{transferResults.length} successful)
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="results">
          <Card>
            <CardHeader>
              <CardTitle>Transfer Results</CardTitle>
              <CardDescription>
                Results of S3 to Walrus transfer
              </CardDescription>
            </CardHeader>
            <CardContent>
              {transferResults.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  <Upload className="w-12 h-12 mx-auto mb-4 opacity-50" />
                  <p>No transfer results yet</p>
                  <p className="text-sm">Complete a transfer to see results here</p>
                </div>
              ) : (
                <>
                  <div className="space-y-4">
                    {transferResults.map((result, idx) => (
                      <div
                        key={idx}
                        className={`p-4 rounded-lg border ${
                          result.success
                            ? 'border-green-200 bg-green-50 dark:bg-green-950/20'
                            : 'border-red-200 bg-red-50 dark:bg-red-950/20'
                        }`}
                      >
                        <div className="space-y-3">
                          <div className="flex items-start justify-between">
                            <div className="flex items-start gap-3">
                              {result.success ? (
                                <CheckCircle className="w-5 h-5 text-green-600 mt-0.5" />
                              ) : (
                                <XCircle className="w-5 h-5 text-red-600 mt-0.5" />
                              )}
                              <div>
                                <p className="font-medium text-sm">{result.key}</p>
                                <p className="text-xs text-muted-foreground">
                                  {result.success ? 'Successfully transferred' : 'Transfer failed'}
                                </p>
                              </div>
                            </div>
                          </div>

                          {result.success && result.blobId && (
                            <div className="pl-8 space-y-2">
                              <div className="flex items-center justify-between p-2 bg-background rounded border">
                                <div>
                                  <p className="text-xs text-muted-foreground uppercase tracking-wide">Full Blob ID</p>
                                  <code className="text-xs font-mono break-all">{result.blobId}</code>
                                </div>
                                <div className="flex gap-2 ml-4">
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => {
                                      navigator.clipboard.writeText(result.blobId!)
                                      success('Copied!', 'Blob ID copied to clipboard')
                                    }}
                                  >
                                    <Copy className="w-3 h-3" />
                                  </Button>
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => window.open(`https://walruscan.com/testnet/blob/${result.blobId}`, '_blank')}
                                  >
                                    <ExternalLink className="w-3 h-3" />
                                  </Button>
                                </div>
                              </div>
                            </div>
                          )}

                          {result.error && (
                            <div className="pl-8">
                              <div className="p-2 bg-red-100 dark:bg-red-950/30 rounded border border-red-200">
                                <p className="text-xs text-red-700 dark:text-red-400 font-medium">Error Details:</p>
                                <p className="text-xs text-red-600 dark:text-red-300 mt-1">{result.error}</p>
                              </div>
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}