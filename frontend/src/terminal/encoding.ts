// Encoding helpers used to bridge the WebSocket envelope (base64 binary
// payloads) and the xterm byte stream, plus a small encoding registry.

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

export function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64)
  const out = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    out[i] = binary.charCodeAt(i)
  }
  return out
}

export function textToBytes(text: string, encoding = 'utf-8'): Uint8Array {
  try {
    return new TextEncoder().encode(text)
  } catch {
    return new TextEncoder().encode(text)
  }
}

export function bytesToText(bytes: Uint8Array, encoding = 'utf-8'): string {
  // xterm handles UTF-8 natively; we only decode for optional previews.
  try {
    return new TextDecoder(encoding).decode(bytes)
  } catch {
    return new TextDecoder().decode(bytes)
  }
}

// Common terminal encodings exposed in the settings UI.
export const ENCODINGS = [
  'utf-8',
  'gbk',
  'gb2312',
  'big5',
  'euc-jp',
  'euc-kr',
  'iso-8859-1',
  'windows-1251',
  'koi8-r',
] as const
