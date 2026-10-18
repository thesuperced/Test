import { client } from '$lib/api/client';

__INJECT_RETURN = (async () => {
    // Injected into "normal" overlay-tab or SharedJSContext

    if (!window.__sisrCleanup) {
        window.__sisrCleanup = {};
    }
    const steamClientOverlay = window.SteamClient?.Overlay ?? (opener as SteamWindow)?.SteamClient?.Overlay;

    if (window.__sisrCleanup.overlayCallback) {
        window.__sisrCleanup.overlayCallback();
    }

    let virtualMenuGeneration = 0;
    let virtualMenuTimer: ReturnType<typeof setTimeout> | undefined;
    const readCreatedField = (message: unknown): boolean | undefined => {
        let bytes: Uint8Array;
        if (message instanceof Uint8Array) {
            bytes = message;
        } else if (message instanceof ArrayBuffer) {
            bytes = new Uint8Array(message);
        } else if (ArrayBuffer.isView(message)) {
            bytes = new Uint8Array(message.buffer, message.byteOffset, message.byteLength);
        } else {
            return undefined;
        }
        for (let offset = 0; offset < bytes.length;) {
            let tag = 0;
            let shift = 0;
            while (offset < bytes.length) {
                const value = bytes[offset++];
                tag |= (value & 0x7f) << shift;
                if ((value & 0x80) === 0) break;
                shift += 7;
                if (shift > 28) return undefined;
            }
            const field = tag >>> 3;
            const wire = tag & 7;
            if (wire === 0) {
                let value = 0;
                shift = 0;
                while (offset < bytes.length) {
                    const byte = bytes[offset++];
                    value |= (byte & 0x7f) << shift;
                    if ((byte & 0x80) === 0) break;
                    shift += 7;
                    if (shift > 28) return undefined;
                }
                if (field === 1) return value !== 0;
            } else if (wire === 1) {
                offset += 8;
            } else if (wire === 2) {
                let length = 0;
                shift = 0;
                while (offset < bytes.length) {
                    const byte = bytes[offset++];
                    length |= (byte & 0x7f) << shift;
                    if ((byte & 0x80) === 0) break;
                    shift += 7;
                    if (shift > 28) return undefined;
                }
                offset += length;
            } else if (wire === 5) {
                offset += 4;
            } else {
                return undefined;
            }
        }
        return undefined;
    };

    const input = window.SteamClient?.Input ?? (opener as SteamWindow)?.SteamClient?.Input;
    const virtualMenuRegistration = input?.RegisterForTouchMenuMessages?.((message: unknown) => {
        const created = readCreatedField(message);
        if (created === undefined) return;
        const generation = ++virtualMenuGeneration;
        if (virtualMenuTimer) clearTimeout(virtualMenuTimer);
        if (!created) {
            virtualMenuTimer = setTimeout(() => {
                if (generation !== virtualMenuGeneration) return;
                void client.POST('/api/v1/steam/cef/clear-virtual-menus')
                    .catch((e) => console.error('Error clearing stale virtual menus', e));
            }, 300);
        }
    });

    if (virtualMenuRegistration) {
        window.__sisrCleanup.virtualMenuCallback = () => {
            if (virtualMenuTimer) clearTimeout(virtualMenuTimer);
            virtualMenuRegistration.unregister?.();
            delete window.__sisrCleanup?.virtualMenuCallback;
        };
    }

    if (steamClientOverlay) {
        const unregister = await steamClientOverlay.RegisterForOverlayActivated(
            (_a, _b, overlayOpen) => {
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
