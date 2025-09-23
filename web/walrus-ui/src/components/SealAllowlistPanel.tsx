import { useState } from 'react'
import { useCurrentAccount, useSignAndExecuteTransaction } from '@mysten/dapp-kit'
import { useSealAllowlists, type SealAllowlist } from '@/hooks/useSealAllowlists'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { Transaction } from '@mysten/sui/transactions'
import { Check, Copy, Loader2, RotateCw } from 'lucide-react'

interface SealAllowlistPanelProps {
  packageId: string
  onSelectAllowlist: (allowlist: SealAllowlist) => void
  selectedAllowlistId?: string
  onAllowlistsChanged?: () => void
}

export function SealAllowlistPanel({ packageId, onSelectAllowlist, selectedAllowlistId, onAllowlistsChanged }: SealAllowlistPanelProps) {
  const { allowlists, loading, refresh } = useSealAllowlists({ packageId })
  const currentAccount = useCurrentAccount()
  const { mutate: signAndExecute } = useSignAndExecuteTransaction()

  const [newAllowlistName, setNewAllowlistName] = useState('')
  const [creating, setCreating] = useState(false)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const canManage = Boolean(currentAccount && packageId)

  const handleCreateAllowlist = async () => {
    if (!canManage || !newAllowlistName.trim()) {
      return
    }

    setCreating(true)
    try {
      const tx = new Transaction()
      tx.moveCall({
        target: `${packageId}::allowlist::create_allowlist_entry`,
        arguments: [tx.pure.string(newAllowlistName.trim())],
      })
      tx.setGasBudget(10_000_000)

      const result: any = await new Promise((resolve, reject) => {
        signAndExecute(
          { transaction: tx },
          {
            onSuccess: (res) => resolve(res),
            onError: reject,
          }
        )
      })

      const createdAllowlistId: string | undefined = Array.isArray(result?.effects?.created)
        ? result.effects.created
            .map((created: any) => created?.reference?.objectId)
            .find(Boolean)
        : undefined

      setNewAllowlistName('')
      const updated = await refresh()
      await onAllowlistsChanged?.()

      if (createdAllowlistId) {
        const created = updated.find((entry) => entry.id === createdAllowlistId)
        if (created) {
          onSelectAllowlist(created)
        }
      }

    } catch (createError) {
      console.error('Failed to create allowlist', createError)
    } finally {
      setCreating(false)
    }
  }

  const handleCopy = (value: string) => {
    navigator.clipboard
      .writeText(value)
      .then(() => {
        setCopiedId(value)
        setTimeout(() => setCopiedId(null), 2000)
      })
      .catch((err) => {
        console.error('Failed to copy allowlist id', err)
      })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Seal Allowlists</CardTitle>
        <CardDescription>
          Create or reuse allowlists to obtain policy object IDs for Seal encryption
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!currentAccount && (
          <p className="text-sm text-muted-foreground">
            Connect your wallet to create or manage allowlists.
          </p>
        )}

        <div className="flex gap-3 items-end">
          <div className="flex-1 space-y-2">
            <label className="text-xs font-medium text-muted-foreground">Allowlist name</label>
            <Input
              value={newAllowlistName}
              onChange={(e) => setNewAllowlistName(e.target.value)}
              placeholder="Team archive"
              disabled={!canManage}
            />
          </div>
          <Button onClick={handleCreateAllowlist} disabled={!canManage || !newAllowlistName.trim() || creating}>
            {creating ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : null}
            Create
          </Button>
          <Button
            variant="outline"
            onClick={async () => {
              await refresh()
              await onAllowlistsChanged?.()
            }}
            disabled={!canManage || loading}
          >
            {loading ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <RotateCw className="w-4 h-4 mr-2" />} 
            Refresh
          </Button>
        </div>

        <div className="rounded-md border max-h-64 overflow-y-auto">
          <div className="divide-y">
              {allowlists.length === 0 ? (
                <div className="p-4 text-sm text-muted-foreground">
                  {loading ? 'Loading allowlists…' : 'No allowlists found. Create one to get a policy ID.'}
                </div>
              ) : (
                allowlists.map((allowlist) => {
                  const isSelected = selectedAllowlistId === allowlist.id
                  return (
                    <div
                      key={allowlist.id}
                      className={cn(
                        'p-4 space-y-2 transition-colors',
                        isSelected ? 'bg-primary/5 border-l-2 border-primary' : 'hover:bg-muted/50'
                      )}
                    >
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="font-medium text-sm">{allowlist.name}</p>
                          <p className="text-xs text-muted-foreground">Policy ID: {allowlist.id}</p>
                          {allowlist.capId && (
                            <p className="text-xs text-muted-foreground">Cap ID: {allowlist.capId}</p>
                          )}
                        </div>
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            variant={isSelected ? 'default' : 'secondary'}
                            onClick={() => onSelectAllowlist(allowlist)}
                          >
                            {isSelected ? 'Selected' : 'Use policy'}
                          </Button>
                          <Button
                            size="icon"
                            variant="ghost"
                            onClick={() => handleCopy(allowlist.id)}
                            aria-label="Copy allowlist ID"
                          >
                            {copiedId === allowlist.id ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                          </Button>
                        </div>
                      </div>
                    </div>
                  )
                })
              )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
