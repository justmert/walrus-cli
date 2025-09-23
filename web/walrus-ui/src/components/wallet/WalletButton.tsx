import { useState } from 'react'
import { useCurrentAccount, useConnectWallet, useDisconnectWallet, useWallets } from '@mysten/dapp-kit'
import { useWalletBalance } from './useWalletBalance'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from '@/components/ui/dialog'
import { Wallet, LogOut, Copy, Check, ChevronDown, AlertCircle, Coins } from 'lucide-react'

export function WalletButton() {
  const currentAccount = useCurrentAccount()
  const { mutate: connect } = useConnectWallet()
  const { mutate: disconnect } = useDisconnectWallet()
  const wallets = useWallets()
  const { balanceInSui, isLoading: balanceLoading } = useWalletBalance()
  const [isConnecting, setIsConnecting] = useState(false)
  const [copiedAddress, setCopiedAddress] = useState(false)
  const [walletDialogOpen, setWalletDialogOpen] = useState(false)
  const [connectionError, setConnectionError] = useState<string | null>(null)

  const handleConnect = async (walletName: string) => {
    setIsConnecting(true)
    setConnectionError(null)
    try {
      const wallet = wallets.find(w => w.name === walletName)
      if (wallet) {
        connect(
          { wallet },
          {
            onSuccess: () => {
              setWalletDialogOpen(false)
              setIsConnecting(false)
              setConnectionError(null)
            },
            onError: (error) => {
              console.error('Failed to connect wallet:', error)
              setConnectionError(
                error?.message ||
                'Failed to connect wallet. Please make sure your wallet is unlocked and try again.'
              )
              setIsConnecting(false)
            }
          }
        )
      } else {
        setConnectionError(`Wallet "${walletName}" not found. Please make sure it's installed.`)
        setIsConnecting(false)
      }
    } catch (error) {
      console.error('Wallet connection error:', error)
      setConnectionError('An unexpected error occurred while connecting to the wallet.')
      setIsConnecting(false)
    }
  }

  const handleDisconnect = () => {
    disconnect()
  }

  const copyAddress = () => {
    if (currentAccount?.address) {
      navigator.clipboard.writeText(currentAccount.address)
      setCopiedAddress(true)
      setTimeout(() => setCopiedAddress(false), 2000)
    }
  }

  const formatAddress = (address: string) => {
    return `${address.slice(0, 6)}...${address.slice(-4)}`
  }

  // If not connected, show connect button and dialog
  if (!currentAccount) {
    return (
      <Dialog open={walletDialogOpen} onOpenChange={setWalletDialogOpen}>
        <DialogTrigger asChild>
          <Button variant="outline" size="sm" disabled={isConnecting}>
            <Wallet className="w-4 h-4 mr-2" />
            {isConnecting ? 'Connecting...' : 'Connect Wallet'}
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Connect Wallet</DialogTitle>
            <DialogDescription>
              Choose a wallet to connect to Walrus Storage
            </DialogDescription>
          </DialogHeader>

          {connectionError && (
            <div className="bg-destructive/10 border border-destructive/20 rounded-lg p-3">
              <div className="flex items-center gap-2 text-destructive">
                <AlertCircle className="w-4 h-4" />
                <p className="text-sm">{connectionError}</p>
              </div>
            </div>
          )}

          <div className="grid gap-3">
            {wallets.length > 0 ? (
              wallets.map((wallet) => (
                <Button
                  key={wallet.name}
                  variant="outline"
                  className="justify-start h-12"
                  onClick={() => handleConnect(wallet.name)}
                  disabled={isConnecting}
                >
                  <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500 to-blue-600 flex items-center justify-center mr-3">
                    {wallet.icon ? (
                      <img src={wallet.icon} alt={wallet.name} className="w-6 h-6 rounded" />
                    ) : (
                      <Wallet className="w-4 h-4 text-white" />
                    )}
                  </div>
                  <div className="text-left">
                    <div className="font-medium">{wallet.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {wallet.version ? `Version ${wallet.version}` : 'Sui Wallet'}
                    </div>
                  </div>
                </Button>
              ))
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                <Wallet className="w-12 h-12 mx-auto mb-4 opacity-50" />
                <p className="text-sm">No Sui wallets detected</p>
                <p className="text-xs mt-1">Please install a Sui wallet extension</p>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    )
  }

  // If connected, show dropdown with account info
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-2">
          <div className="w-4 h-4 rounded-full bg-gradient-to-br from-green-500 to-green-600" />
          <span className="font-mono text-xs">
            {formatAddress(currentAccount.address)}
          </span>
          <ChevronDown className="w-3 h-3" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel className="font-normal">
          <div className="flex flex-col space-y-1">
            <p className="text-sm font-medium leading-none">Connected Wallet</p>
            <p className="text-xs leading-none text-muted-foreground">
              {currentAccount.address}
            </p>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="cursor-default hover:bg-transparent focus:bg-transparent">
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              <Coins className="w-4 h-4 text-muted-foreground" />
              <span className="text-sm">Balance</span>
            </div>
            <span className="text-sm font-mono">
              {balanceLoading ? '...' : `${balanceInSui} SUI`}
            </span>
          </div>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={copyAddress}
        >
          {copiedAddress ? (
            <Check className="mr-2 h-4 w-4 text-green-500" />
          ) : (
            <Copy className="mr-2 h-4 w-4" />
          )}
          {copiedAddress ? 'Copied!' : 'Copy Address'}
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={handleDisconnect}
        >
          <LogOut className="mr-2 h-4 w-4" />
          Disconnect
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}