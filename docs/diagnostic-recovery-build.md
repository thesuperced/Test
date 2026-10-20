# Rollback and diagnostic build

Restores the behavior before revision 6e4bae1 (baseline a3593a2).
The new menu touch gate and five-second watchdog have been removed.
This is a recovery/diagnostic build, not a confirmed fix for the original
duplicate-keyboard or radial-menu problems.

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
menu lifecycle metadata, requested visibility changes, foreground process
names/window classes, overlay mouse handling, controller-layout resets, and
input event types (key/button down/up and touchpad down/up, without identities).
It does not record typed text, key contents, clipboard data, window titles,
page contents, screenshots, or raw Steam message payloads.
The file stays on your PC until you choose to send it.

The uninstaller removes the portable program files, leaving diagnostic logs
available to send. Delete the SISR-diagnostics folder manually when finished.
