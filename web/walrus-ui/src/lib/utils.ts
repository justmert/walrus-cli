import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

export function formatWAL(walInput: number | null | undefined): string {
  const wal = Number(walInput ?? 0)

  if (!Number.isFinite(wal) || wal === 0) {
    return '0'
  }

  if (wal >= 1) {
    return wal.toFixed(6)
  }

  if (wal >= 0.001) {
    return wal.toFixed(9)
  }

  return wal.toExponential(3)
}

export function formatWALWithUSD(wal: number, walPriceUSD: number = 0.425): string {
  // WAL price in USD (default is approximate, should be fetched from API)
  const usdValue = wal * walPriceUSD

  const walFormatted = formatWAL(wal)

  if (usdValue >= 0.01) {
    return `${walFormatted} WAL (~$${usdValue.toFixed(2)})`
  } else {
    return `${walFormatted} WAL (~$${usdValue.toFixed(4)})`
  }
}
