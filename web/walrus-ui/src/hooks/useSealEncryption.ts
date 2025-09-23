import { useCallback, useRef, useState } from 'react'
import { useSuiClient, useSignPersonalMessage } from '@mysten/dapp-kit'
import {
  EncryptedObject,
  NoAccessError,
  SealClient,
  SessionKey,
  type KeyServerConfig
} from '@mysten/seal'
import { Transaction } from '@mysten/sui/transactions'
import { fromHex, normalizeSuiAddress, toHex } from '@mysten/sui/utils'

const TESTNET_KEY_SERVERS: KeyServerConfig[] = [
  { objectId: '0x73d05d62c18d9374e3ea529e8e0ed6161da1a141a94d3f76ae3fe4e99356db75', weight: 1 },
  { objectId: '0xf5d14a81a982144ae441cd7d64b09027f116a468bd36e7eca494f750591623c8', weight: 1 },
  { objectId: '0x6068c0acb197dddbacd4746a9de7f025b2ed5a5b6c1b1ab44dade4426d141da2', weight: 1 }
]

const DEFAULT_PACKAGE_ID = '0xc5ce2742cac46421b62028557f1d7aea8a4c50f651379a79afdf12cd88628807'
const DEFAULT_MODULE_NAME = 'allowlist'
const DEFAULT_MVR_NAME = '@pkg/seal-demo-1234'
const DEFAULT_THRESHOLD = 2
const SESSION_KEY_TTL_MINUTES = 15

const stripHexPrefix = (value: string): string =>
  value.startsWith('0x') || value.startsWith('0X') ? value.slice(2) : value

const ensureEvenLength = (hex: string): string => (hex.length % 2 === 0 ? hex : `0${hex}`)

const toHexString = (bytes: Uint8Array): string => toHex(bytes)

const parseHex = (value: string): Uint8Array => fromHex(ensureEvenLength(stripHexPrefix(value)))

export interface EncryptionConfig {
  enableEncryption: boolean
  threshold: number
  packageId?: string
  moduleName?: string
  policyId?: string
  mvrName?: string
}

export interface EncryptedFileMetadata {
  isEncrypted: boolean
  threshold?: number
  keyServers?: string[]
  policyPackage?: string
  policyId?: string
  moduleName?: string
  identity?: string
  nonce?: string
  mvrName?: string
  originalFileName?: string
  originalMimeType?: string
}

export function useSealEncryption() {
  const suiClient = useSuiClient()
  const { mutate: signPersonalMessage } = useSignPersonalMessage()

  const sealClientRef = useRef<SealClient | null>(null)
  const sessionKeyRef = useRef<SessionKey | null>(null)
  const sessionKeySignedRef = useRef<boolean>(false)

  const [isEncrypting, setIsEncrypting] = useState(false)
  const [isDecrypting, setIsDecrypting] = useState(false)

  const getSealClient = useCallback(async () => {
    if (!sealClientRef.current) {
      sealClientRef.current = new SealClient({
        serverConfigs: TESTNET_KEY_SERVERS,
        verifyKeyServers: false,
        suiClient
      })
    }
    return sealClientRef.current
  }, [suiClient])

  const ensureSessionKey = useCallback(
    async (packageId: string, address: string, mvrName?: string) => {
      const normalizedAddress = normalizeSuiAddress(address)
      const existing = sessionKeyRef.current
      if (existing && !existing.isExpired() && normalizeSuiAddress(existing.getAddress()) === normalizedAddress) {
        return existing
      }

      const sessionKey = await SessionKey.create({
        address: normalizedAddress,
        packageId,
        mvrName,
        ttlMin: SESSION_KEY_TTL_MINUTES,
        suiClient
      })
      sessionKeyRef.current = sessionKey
      sessionKeySignedRef.current = false
      return sessionKey
    },
    [suiClient]
  )

  const ensureSessionKeySignature = useCallback(
    async (sessionKey: SessionKey) => {
      if (sessionKeySignedRef.current) {
        return
      }

      await new Promise<void>((resolve, reject) => {
        signPersonalMessage(
          { message: sessionKey.getPersonalMessage() },
          {
            onSuccess: async ({ signature }) => {
              try {
                await sessionKey.setPersonalMessageSignature(signature)
                sessionKeySignedRef.current = true
                resolve()
              } catch (err) {
                reject(err)
              }
            },
            onError: reject
          }
        )
      })
    },
    [signPersonalMessage]
  )

  const buildSealApproveTx = useCallback(
    async (packageId: string, moduleName: string, policyId: string, identity: string) => {
      const tx = new Transaction()
      tx.moveCall({
        target: `${packageId}::${moduleName}::seal_approve`,
        arguments: [tx.pure.vector('u8', parseHex(identity)), tx.object(policyId)]
      })
      return tx.build({ client: suiClient, onlyTransactionKind: true })
    },
    [suiClient]
  )

  const encryptFile = useCallback(
    async (
      data: Uint8Array,
      config: EncryptionConfig
    ): Promise<{ encryptedData: Uint8Array; metadata: EncryptedFileMetadata }> => {
      if (!config.enableEncryption) {
        return { encryptedData: data, metadata: { isEncrypted: false } }
      }

      const packageId = config.packageId ?? DEFAULT_PACKAGE_ID
      const moduleName = config.moduleName ?? DEFAULT_MODULE_NAME
      const threshold = config.threshold || DEFAULT_THRESHOLD
      const policyId = config.policyId

      if (!policyId) {
        throw new Error('Seal policy ID is required to encrypt files.')
      }

      const sealClient = await getSealClient()

      setIsEncrypting(true)
      try {
        const policyBytes = parseHex(policyId)
        const cryptoSource = globalThis.crypto ?? (typeof window !== 'undefined' ? window.crypto : undefined)
        if (!cryptoSource) {
          throw new Error('Secure random generator unavailable in this environment')
        }
        const nonce = cryptoSource.getRandomValues(new Uint8Array(5))
        const identityBytes = new Uint8Array([...policyBytes, ...nonce])
        const identity = toHexString(identityBytes)

        const { encryptedObject } = await sealClient.encrypt({
          threshold,
          packageId,
          id: identity,
          data
        })

        return {
          encryptedData: new Uint8Array(encryptedObject),
          metadata: {
            isEncrypted: true,
            threshold,
            keyServers: TESTNET_KEY_SERVERS.map((server) => server.objectId),
            policyPackage: packageId,
            policyId,
            moduleName,
            identity,
            nonce: toHexString(nonce),
            mvrName: config.mvrName ?? DEFAULT_MVR_NAME
          }
        }
      } finally {
        setIsEncrypting(false)
      }
    },
    [getSealClient]
  )

  const decryptFile = useCallback(
    async (encryptedData: Uint8Array, metadata: EncryptedFileMetadata, userAddress: string) => {
      if (!metadata?.isEncrypted) {
        return encryptedData
      }

      const packageId = metadata.policyPackage ?? DEFAULT_PACKAGE_ID
      const moduleName = metadata.moduleName ?? DEFAULT_MODULE_NAME
      const policyId = metadata.policyId

      if (!policyId) {
        throw new Error('Missing Seal policy metadata; cannot decrypt file.')
      }

      const sealClient = await getSealClient()
      setIsDecrypting(true)

      try {
        const sessionKey = await ensureSessionKey(packageId, userAddress, metadata.mvrName ?? DEFAULT_MVR_NAME)
        await ensureSessionKeySignature(sessionKey)

        const encryptedObject = EncryptedObject.parse(encryptedData)
        const identity = metadata.identity ?? encryptedObject.id

        const txBytes = await buildSealApproveTx(packageId, moduleName, policyId, identity)

        await sealClient.fetchKeys({
          ids: [identity],
          txBytes,
          sessionKey,
          threshold: metadata.threshold ?? DEFAULT_THRESHOLD
        })

        const plaintext = await sealClient.decrypt({
          data: encryptedData,
          sessionKey,
          txBytes
        })

        return new Uint8Array(plaintext)
      } catch (err) {
        if (err instanceof NoAccessError) {
          throw new Error('Seal access denied. Ensure you are authorized for this policy.')
        }
        throw err
      } finally {
        setIsDecrypting(false)
      }
    },
    [
      buildSealApproveTx,
      ensureSessionKey,
      ensureSessionKeySignature,
      getSealClient
    ]
  )

  const checkAccess = useCallback(
    async (metadata: EncryptedFileMetadata, userAddress: string) => {
      if (!metadata?.isEncrypted) {
        return true
      }

      try {
        await ensureSessionKey(
          metadata.policyPackage ?? DEFAULT_PACKAGE_ID,
          userAddress,
          metadata.mvrName ?? DEFAULT_MVR_NAME
        )
        return true
      } catch {
        return false
      }
    },
    [ensureSessionKey]
  )

  return {
    encryptFile,
    decryptFile,
    checkAccess,
    isEncrypting,
    isDecrypting,
    keyServers: TESTNET_KEY_SERVERS
  }
}
