# Native keyboard routing and menu ownership test build

Based on the recorded failures in diagnostic build de10c97:

- The standalone keyboard remains open after the in-game overlay closes.
  The previous duplicate-page closer never fires: only one standalone keyboard
  target is observed. Closing arbitrary keyboard targets cannot fix this case.
- Overlay-open changes the invisible SISR host from click-through to mouse
  interception with keyboard/mouse emulation disabled.
- Menu messages decode successfully, and the patch sends visibility changes
  and selection resets across action-set transitions. These mutations and the
  competing native menu subscription are removed.

This build yields SISR's forced controller layout while the native keyboard is
open, hides only SISR's game overlay, and restores the mapping after a confirmed
keyboard close. User-selected layout changes take precedence. Desktop mode
keeps the hidden host click-through; keyboard/mouse emulation still captures.
Steam owns radial-menu creation, selection and release again. The old global
CSS watchdog, touch gate, selection resets and menu callbacks are absent.

Tests validate routing, ownership, cancellation, transient CEF failure, and
layout-generation guards. Physical controller behavior still needs validation.

1. Fully exit every SISR copy.
2. Extract the entire ZIP to a new writable folder.
3. Reboot once to clear the previous injected Steam callbacks and windows.
4. Start Steam/controller and then this SISR.exe.
5. Reproduce one keyboard open/close cycle, followed by one radial-menu
   action-set cycle. Quit SISR.
6. Open the SISR-diagnostics folder next to SISR.exe and send the newest
   SISR-*.jsonl file in this chat. Each launch gets a separate file, so
   also send the previous file if you restarted SISR during the test.

If the application folder is not writable, logs go to
%LOCALAPPDATA%\SISR\diagnostics instead. A log is capped at about 8 MB;
restart SISR to create a new recording if the cap is reached.

The dedicated log records build/version, keyboard/menu target identifiers,
native keyboard routing, foreground process
names/window classes, overlay mouse handling, controller-layout resets, and
input event types (key/button down/up and touchpad down/up, without identities).
It does not record typed text, key contents, clipboard data, window titles,
page contents, screenshots, or raw Steam message payloads.
The file stays on your PC until you choose to send it.

New events include native_keyboard_route and keyboard_layout_suspended.
Native menu callbacks are no longer subscribed to just for logging; menu
target metadata remains visible in cef_targets.

The uninstaller removes the portable program files, leaving diagnostic logs
available to send. Delete the SISR-diagnostics folder manually when finished.
