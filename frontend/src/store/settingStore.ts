import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import { rest } from '../api/rest'
import { migratedStorage, settingsPersistKey } from '../lib/storage'
import type { UserSettings } from '../types'

export type ThemeName = string

// SETTINGS_DEFAULTS mirrors the backend's store.DefaultSettingsJSON() (C5):
// code constants are the source of defaults, the user's saved JSON overrides
// individual fields. Keep these two in sync.
export const SETTINGS_DEFAULTS: UserSettings = {
  theme: 'default-dark',
  fontSize: 14,
  fontFamily: 'Menlo, Monaco, Consolas, "Courier New", monospace',
  encoding: 'utf-8',
  cursorBlink: true,
  cursorStyle: 'block',
  scrollback: 10000,
  webgl: false,
  lineHeight: 1.0,
  letterSpacing: 0,
  defaultProtocol: 'ssh',
  defaultAuthType: 'password',
  defaultTransfer: 'sftp',
  recvAutoDownload: false,
  strictHostKeyChecking: false,
  connectTimeoutSec: 15,
}

interface SettingState extends UserSettings {
  /** Fetch the current user's settings from GET /api/settings, overriding defaults. */
  load: () => Promise<void>
  /** Update one or more fields locally and persist to the backend (PUT /api/settings). */
  set: (patch: Partial<UserSettings>) => void
  /** Persist the full current settings object to the backend. */
  save: () => Promise<void>
}

// settingStore keeps settings in zustand (persisted to localStorage as a
// local fallback) while treating GET/PUT /api/settings as the authoritative
// per-user store when authentication is enabled.
export const useSettingStore = create<SettingState>()(
  persist(
    (set, get) => ({
      ...SETTINGS_DEFAULTS,

      load: async () => {
        try {
          const r = await rest.getSettings()
          if (r.code === 0 && r.data) {
            set({ ...r.data })
          }
        } catch {
          /* Backend unavailable (e.g. auth disabled) -> keep local defaults. */
        }
      },

      set: (patch) => {
        set(patch)
        void get().save()
      },

      save: async () => {
        const s = get()
        const payload: Partial<UserSettings> = {
          theme: s.theme,
          fontSize: s.fontSize,
          fontFamily: s.fontFamily,
          encoding: s.encoding,
          cursorBlink: s.cursorBlink,
          cursorStyle: s.cursorStyle,
          scrollback: s.scrollback,
          webgl: s.webgl,
          lineHeight: s.lineHeight,
          letterSpacing: s.letterSpacing,
          defaultProtocol: s.defaultProtocol,
          defaultAuthType: s.defaultAuthType,
          defaultTransfer: s.defaultTransfer,
          recvAutoDownload: s.recvAutoDownload,
          strictHostKeyChecking: s.strictHostKeyChecking,
          connectTimeoutSec: s.connectTimeoutSec,
        }
        try {
          await rest.putSettings(payload)
        } catch {
          /* Non-fatal: e.g. auth disabled where PUT is rejected; localStorage fallback holds. */
        }
      },
    }),
    {
      name: settingsPersistKey,
      storage: createJSONStorage(() => migratedStorage),
    },
  ),
)
