import { useCurrentAccount, useCurrentWallet, useSuiClient } from '@mysten/dapp-kit'
import { Wallet, Globe, Wifi, WifiOff } from 'lucide-react'
import { useEffect, useState } from 'react'

interface WalletStatusProps {
  networkName?: string
}

export function WalletStatus({ networkName = 'testnet' }: WalletStatusProps) {
  const currentAccount = useCurrentAccount()
  const { currentWallet } = useCurrentWallet()
  const suiClient = useSuiClient()
  const [isOnChain, setIsOnChain] = useState(false)
  const [checking, setChecking] = useState(false)

  const formatAddress = (address: string) => {
    return `${address.slice(0, 6)}...${address.slice(-4)}`
  }

  // Periodically check if wallet is truly connected to blockchain
  useEffect(() => {
    if (!currentAccount) {
      setIsOnChain(false)
      return
    }

    const checkConnection = async () => {
      setChecking(true)
      try {
        // Try to fetch coins to verify on-chain connection
        await suiClient.getCoins({
          owner: currentAccount.address,
          limit: 1
        })
        setIsOnChain(true)
      } catch (error) {
        console.error('Failed to verify on-chain connection:', error)
        setIsOnChain(false)
      } finally {
        setChecking(false)
      }
    }

    checkConnection()
    // Re-check every 30 seconds
    const interval = setInterval(checkConnection, 30000)

    return () => clearInterval(interval)
  }, [currentAccount, suiClient])

  return (
    <>
      <div className="flex items-center gap-2 text-sm">
        <Globe className="w-4 h-4 text-muted-foreground" />
        <span className="font-medium capitalize">{networkName}</span>
      </div>
      <div className="flex items-center gap-2 text-sm">
        <Wallet className="w-4 h-4 text-muted-foreground" />
        {currentAccount ? (
          <div className="flex items-center gap-2">
            <div
              className={`w-2 h-2 rounded-full ${
                checking ? 'bg-yellow-500 animate-pulse' :
                isOnChain ? 'bg-green-500' : 'bg-red-500'
              }`}
              title={
                checking ? 'Checking connection...' :
                isOnChain ? 'Connected to blockchain' : 'Not connected to blockchain'
              }
            />
            <span className="font-mono text-xs">
              {formatAddress(currentAccount.address)}
            </span>
            {currentWallet && (
              <span className="text-xs text-muted-foreground">
                ({currentWallet.name})
              </span>
            )}
            {isOnChain && (
              <Wifi className="w-3 h-3 text-green-500" aria-label="On-chain verified" />
            )}
            {!isOnChain && !checking && (
              <WifiOff className="w-3 h-3 text-red-500" aria-label="Off-chain" />
            )}
          </div>
        ) : (
          <span className="text-muted-foreground">Not Connected</span>
        )}
      </div>
    </>
  )
}
