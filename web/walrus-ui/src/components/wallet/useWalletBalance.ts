import { useCurrentAccount, useSuiClientQuery } from '@mysten/dapp-kit'
import { MIST_PER_SUI } from '@mysten/sui/utils'

export function useWalletBalance() {
  const currentAccount = useCurrentAccount()

  const { data: balance, isLoading, error, refetch } = useSuiClientQuery(
    'getBalance',
    {
      owner: currentAccount?.address || '',
    },
    {
      enabled: !!currentAccount?.address,
    }
  )

  const formatBalance = (balance?: string) => {
    if (!balance) return '0'
    const balanceInSui = Number(balance) / Number(MIST_PER_SUI)
    return balanceInSui.toFixed(4)
  }

  return {
    balance: balance?.totalBalance,
    balanceInSui: formatBalance(balance?.totalBalance),
    isLoading,
    error,
    refetch,
    hasBalance: balance && Number(balance.totalBalance) > 0,
  }
}