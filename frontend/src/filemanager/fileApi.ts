import { rest } from '../api/rest'
import type { ConnectionSpec, FileEntry } from '../types'

/** Error thrown by file API wrappers; carries the server error code. */
export class FileApiError extends Error {
  constructor(
    message: string,
    public code: number,
  ) {
    super(message)
    this.name = 'FileApiError'
  }
}

// Thin typed wrappers around the REST file-manager endpoints.
export async function listDir(
  conn: ConnectionSpec,
  path: string,
  transfer = 'sftp',
): Promise<FileEntry[]> {
  const r = await rest.list(conn, path, transfer)
  if (r.code !== 0) throw new FileApiError(r.message, r.code)
  return r.data || []
}

export async function makeDir(
  conn: ConnectionSpec,
  path: string,
  transfer = 'sftp',
): Promise<void> {
  const r = await rest.mkdir(conn, path, transfer)
  if (r.code !== 0) throw new FileApiError(r.message, r.code)
}

export async function removePath(
  conn: ConnectionSpec,
  path: string,
  transfer = 'sftp',
): Promise<void> {
  const r = await rest.remove(conn, path, transfer)
  if (r.code !== 0) throw new FileApiError(r.message, r.code)
}

export async function renamePath(
  conn: ConnectionSpec,
  oldPath: string,
  newPath: string,
  transfer = 'sftp',
): Promise<void> {
  const r = await rest.rename(conn, oldPath, newPath, transfer)
  if (r.code !== 0) throw new FileApiError(r.message, r.code)
}

export function downloadUrl(
  conn: ConnectionSpec,
  path: string,
  transfer = 'sftp',
): string {
  return rest.downloadUrl(conn, path, transfer)
}

export async function uploadFile(
  conn: ConnectionSpec,
  path: string,
  file: File,
  transfer = 'sftp',
): Promise<void> {
  const r = await rest.upload(conn, path, file, transfer)
  if (r.code !== 0) throw new FileApiError(r.message, r.code)
}
