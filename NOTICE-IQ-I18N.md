# IQ I18N Notice

This repository may contain integration hooks for a proprietary localization overlay.

The AWG Manager base application remains licensed under its original MIT license.

The IQ localization runtime, dictionaries, watermarks, and translation assets are proprietary components and are not part of the public source tree unless explicitly provided under a separate license.

Public repository rules:
- Do not commit translation dictionaries.
- Do not commit `tr.json`, `fa.json`, `zh.json`.
- Do not commit generated proprietary runtime sources.
- The generated `iq-i18n-runtime.min.js` may appear only in release build artifacts.
