# Keyboard/menu recovery test build

This revision fixes identified defects in the previous experimental patch.
It has automated regression coverage, but needs testing on the affected Windows
machine with its Steam Controller configuration.

- Duplicate overlay-open/close callbacks no longer overwrite the saved mouse
  and webview state.
- Startup no longer briefly forces the desktop layout before the SISR layout.
- Keyboard observation no longer arbitrarily closes one of multiple CEF pages.
  Target-count transitions are logged without keyboard contents.
- Menu lifecycle decoding accepts protobuf defaults, base64 and byte payloads;
  malformed/truncated messages are ignored.
- Menu creation alone cannot show a menu. Fresh active touch is required.
  Lifecycle changes cancel visible menus; interrupted touches must be released
  before they can show a menu again. Reinjection unregisters old listeners.
- The patch no longer calls Steam's selection setter with an invented reset.
- Visibility updates are serialized and coalesced, and CDP results are checked.
  An inactivity watchdog and a five-second lease in the menu page bound stale
  displays when notifications/updates stop. Normal shutdown removes overrides.

The watchdog can hide a stationary held menu after five seconds without input
notifications. Release and touch again to reopen it. Continuous stale active
notifications or a nonresponsive Steam renderer cannot be distinguished or
repaired reliably with these hooks: this is not a guarantee for every Steam
state, nor proof that the first-use duplicate keyboard is resolved.

## Hardware acceptance test

1. Quit all SISR copies, extract the new ZIP into a separate folder, and reboot.
2. Start Steam/controller, then only this SISR.exe. Open the keyboard once.
   Check there is one keyboard, trackpads work, and dismissal restores input.
3. Repeat keyboard open/close several times and type into Notepad.
4. Hold a radial menu open, cycle through all action sets (including one without
   a menu), release the pad, return and touch again. Confirm it hides on leaving,
   reopens, sends the selected key to Notepad, then disappears on release.
5. Repeat fast cycling and test both pads. Check the inactivity fallback by
   holding still for over five seconds, then releasing and touching again.
6. Quit this build and verify ordinary Steam input still works.

If the duplicate keyboard recurs, retain the SISR log including startup and
the first open/close cycle. The screenshot alone does not identify which Steam
context owns either keyboard.
