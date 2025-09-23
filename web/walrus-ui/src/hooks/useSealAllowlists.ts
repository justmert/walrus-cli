import { useCallback, useEffect, useState } from 'react'
import { useCurrentAccount, useSuiClient } from '@mysten/dapp-kit'

export interface SealAllowlist {
  id: string
  name: string
  capId?: string
}

interface UseSealAllowlistsOptions {
  packageId?: string
  autoRefresh?: boolean
}

const normalizePackageId = (packageId?: string) => packageId?.trim() ?? ''

export function useSealAllowlists({ packageId, autoRefresh = true }: UseSealAllowlistsOptions) {
  const suiClient = useSuiClient()
  const currentAccount = useCurrentAccount()

  const [allowlists, setAllowlists] = useState<SealAllowlist[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const refresh = useCallback(async (): Promise<SealAllowlist[]> => {
    const pkg = normalizePackageId(packageId)
    if (!currentAccount || !pkg) {
      setAllowlists([])
      return []
    }

    setLoading(true)
    setError(null)

    try {
      const caps = await suiClient.getOwnedObjects({
        owner: currentAccount.address,
        options: {
          showContent: true,
        },
        filter: {
          StructType: `${pkg}::allowlist::Cap`,
        },
      })

      const fetched = await Promise.all(
        caps.data.map(async (object) => {
          const fields = (object.data?.content as { fields?: any })?.fields
          const allowlistId: string | undefined = fields?.allowlist_id
          const capId: string | undefined = fields?.id?.id

          if (!allowlistId) {
            return null
          }

          try {
            const allowlistObject = await suiClient.getObject({
              id: allowlistId,
              options: { showContent: true },
            })
            const allowlistFields = (allowlistObject.data?.content as { fields?: any })?.fields
            const name: string = allowlistFields?.name ?? 'Untitled allowlist'

            return {
              id: allowlistId,
              name,
              capId,
            } as SealAllowlist
          } catch (readError) {
            console.warn('Failed to load allowlist metadata', readError)
            return {
              id: allowlistId,
              name: 'Allowlist',
              capId,
            }
          }
        })
      )

      const filtered = fetched.filter((entry): entry is SealAllowlist => entry !== null)
      setAllowlists(filtered)
      return filtered
    } catch (refreshError) {
      console.error('Failed to load Seal allowlists', refreshError)
      setError(refreshError as Error)
      setAllowlists([])
      return []
    } finally {
      setLoading(false)
    }
  }, [currentAccount, packageId, suiClient])

  useEffect(() => {
    if (!autoRefresh) {
      return
    }
    refresh()
  }, [refresh, autoRefresh])

  return {
    allowlists,
    loading,
    error,
    refresh,
  }
}
