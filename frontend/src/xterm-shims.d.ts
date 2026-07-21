// Ambient declarations for OPTIONAL @xterm addons. They are loaded at
// runtime via dynamic import (with @vite-ignore) so the build does not
// require them to be installed. When present, inline image support and font
// ligatures activate automatically.
//
// NOTE: @xterm/addon-webgl is now a formal dependency and imported
// statically from XTermView.tsx, so it no longer needs an ambient shim.

declare module '@xterm/addon-image' {
  export class ImageAddon {
    constructor()
    activate(terminal: unknown): void
    dispose(): void
  }
}

declare module '@xterm/addon-ligatures' {
  export class LigaturesAddon {
    constructor(font?: unknown)
    activate(terminal: unknown): void
    dispose(): void
  }
}
