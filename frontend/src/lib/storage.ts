// Centralized localStorage key names. The project was renamed from "WebSSH" to
// "go-Term", so every persisted key moved from the `webssh_*` / `webssh-*`
// prefix to `goterm_*` / `goterm-*`. getItemWithMigration transparently copies
// the legacy key to the new one on first read so existing sessions, settings,
// history and quick commands survive the rename without data loss.

export const tokenKey = 'goterm_token'
export const authPersistKey = 'goterm-auth'
export const settingsPersistKey = 'goterm-settings'
export const historyKey = 'goterm-history'
export const quickKey = 'goterm-quick'

// legacyOf maps a goterm_* key back to its webssh_* form. All keys share the
// same prefix length, so a plain prefix swap covers both `_` and `-` variants.
function legacyOf(name: string): string {
  return name.startsWith('goterm') ? 'webssh' + name.slice('goterm'.length) : name
}

// getItemWithMigration reads from the new key, falling back to (and migrating
// from) the legacy webssh_* key when the new one is absent.
export function getItemWithMigration(key: string): string | null {
  try {
    const v = localStorage.getItem(key)
    if (v != null) return v
    const old = legacyOf(key)
    if (old !== key) {
      const ov = localStorage.getItem(old)
      if (ov != null) {
        localStorage.setItem(key, ov)
        localStorage.removeItem(old)
        return ov
      }
    }
  } catch {
    /* localStorage unavailable (e.g. SSR) — treat as empty */
  }
  return null
}

// migratedStorage is a StateStorage-compatible adapter for zustand's persist
// middleware; it applies the same legacy-key migration on read.
export const migratedStorage = {
  getItem: (name: string) => getItemWithMigration(name),
  setItem: (name: string, value: string) => {
    try {
      localStorage.setItem(name, value)
    } catch {
      /* ignore */
    }
  },
  removeItem: (name: string) => {
    try {
      localStorage.removeItem(name)
    } catch {
      /* ignore */
    }
  },
}

// migrateStorageKeys sweeps all known keys proactively so the legacy keys are
// cleaned up even before the first lazy read. Safe to call once at startup.
export function migrateStorageKeys(): void {
  for (const key of [tokenKey, authPersistKey, settingsPersistKey, historyKey, quickKey]) {
    getItemWithMigration(key)
  }
}
