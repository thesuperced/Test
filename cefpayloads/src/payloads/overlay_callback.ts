import { client } from '$lib/api/client';

__INJECT_RETURN = (async () => {
    // Diagnostic side channel: metadata only, never keys or protobuf contents.
    const diagnostic = (body: Record<string, unknown>) => {
        void fetch('goTmpl(".SISRAPIURL")' + '/api/v1/steam/cef/diagnostics', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
            signal: AbortSignal.timeout(2000)
        }).catch(() => {});
    };
    diagnostic({ kind: 'injection' });
    // Injected into "normal" overlay-tab or SharedJSContext

    if (!window.__sisrCleanup) {
        window.__sisrCleanup = {};
    }
    const steamClientOverlay = window.SteamClient?.Overlay ?? (opener as SteamWindow)?.SteamClient?.Overlay;

    if (window.__sisrCleanup.overlayCallback) {
        window.__sisrCleanup.overlayCallback();
    }

    // Steam owns menu creation, selection and release. Do not register a
    // competing native menu consumer or manipulate reusable menu pages. We do
    // observe lifecycle edges only: the edge itself is the completed action.
    window.__sisrCleanup.virtualMenuCallback?.();
    const readVirtualMenuEvent = (message: unknown) => {
        let bytes: Uint8Array;
        if (message instanceof Uint8Array) bytes = message;
        else if (message instanceof ArrayBuffer) bytes = new Uint8Array(message);
        else if (ArrayBuffer.isView(message)) bytes = new Uint8Array(message.buffer, message.byteOffset, message.byteLength);
        else return;
        let offset = 0;
        const fields = new Map<number, number>();
        const varint = () => {
            let value = 0;
            for (let shift = 0; shift < 35 && offset < bytes.length; shift += 7) {
                const b = bytes[offset++]; value += (b & 127) * 2 ** shift;
                if (!(b & 128)) return value;
            }
            throw new Error('invalid protobuf');
        };
        try {
            while (offset < bytes.length) {
                const tag = varint(); const field = Math.floor(tag / 8);
                switch (tag % 8) {
                    case 0: fields.set(field, varint()); break;
                    case 1: offset += 8; break;
                    case 2: offset += varint(); break;
                    case 5: offset += 4; break;
                    default: return;
                }
                if (offset > bytes.length) return;
            }
        } catch { return; }
        if (!fields.has(1)) return;
        return { created: fields.get(1) !== 0, controller: fields.get(2) ?? 0, menu: fields.get(3) ?? 0 };
    };
    const input = window.SteamClient?.Input ?? (opener as SteamWindow)?.SteamClient?.Input;
    const activeVirtualMenus = new Set<string>();
    const menuRegistration = input?.RegisterForTouchMenuMessages?.((message: unknown) => {
        const event = readVirtualMenuEvent(message);
        diagnostic({ kind: 'menu_message', parsed: !!event, messageType: typeof message,
            length: ArrayBuffer.isView(message) ? message.byteLength : message instanceof ArrayBuffer ? message.byteLength : 0 });
        if (!event) return;
        diagnostic({ kind: 'menu_lifecycle', controller: event.controller, menu: event.menu, created: event.created });
        const key = `${event.controller}:${event.menu}`;
        if (event.created) activeVirtualMenus.add(key);
        else activeVirtualMenus.delete(key);
        void client.POST('/api/v1/steam/cef/virtual-menu-visibility', {
            body: { visible: activeVirtualMenus.size > 0 }
        }).catch((e) => console.error('Error updating virtual menu visibility', e));
    });
    // This callback is the completed action-set transition. Hide any old
    // reusable menu immediately; a subsequent create edge restores it.
    const configRegistration = input?.RegisterForActiveConfigLoadedMessages?.(() => {
        diagnostic({ kind: 'action_set_loaded' });
        activeVirtualMenus.clear();
        void client.POST('/api/v1/steam/cef/virtual-menu-visibility', {
            body: { visible: false }
        }).catch((e) => console.error('Error hiding menu after action-set load', e));
    });
    if (menuRegistration || configRegistration) {
        window.__sisrCleanup.virtualMenuCallback = () => {
            menuRegistration?.unregister?.();
            configRegistration?.unregister?.();
            delete window.__sisrCleanup?.virtualMenuCallback;
        };
    }

    if (steamClientOverlay) {
        const routing = window as typeof window & {
            __sisrNativeKeyboardOpen?: boolean;
            __sisrKeyboardGameID?: string;
        };
        const unregister = await steamClientOverlay.RegisterForOverlayActivated(
            (_a, _b, overlayOpen) => {
                diagnostic({ kind: 'overlay', open: overlayOpen });
                if (overlayOpen && routing.__sisrNativeKeyboardOpen && routing.__sisrKeyboardGameID) {
                    steamClientOverlay.SetOverlayState?.(routing.__sisrKeyboardGameID, 0);
                    return;
                }
                void client.POST('/api/v1/steam/cef/overlay-state-changed', {
                    body: {
                        open: overlayOpen
                    }
                }).catch((e) => console.error('Error sending overlay state change', e));
            }
        ) as { unregister: () => void } ;

        window.__sisrCleanup.overlayCallback = () => {
            unregister.unregister?.();
            delete window.__sisrCleanup?.overlayCallback;
        };
    } else {
        // Injected into "Gaming Mode", no overlay tab exists,
        // but we can query focus of the big picture menu ;)

        const focusListener = () => {
            void client.POST('/api/v1/steam/cef/overlay-state-changed', {
                body: {
                    open: document.hasFocus()
                }
            }).catch((e) => console.error('Error sending overlay state change', e));
        };
        const focusOutListener = () => {
            void client.POST('/api/v1/steam/cef/overlay-state-changed', {
                body: {
                    open: false
                }
            }).catch((e) => console.error('Error sending overlay state change', e));
        };

        window.addEventListener('focus', focusListener);
        window.addEventListener('focusout', focusOutListener);
        window.__sisrCleanup.overlayCallback = () => {
            window.removeEventListener('focus', focusListener);
            window.removeEventListener('focusout', focusOutListener);
            delete window.__sisrCleanup?.overlayCallback;
        };
    }
})();
