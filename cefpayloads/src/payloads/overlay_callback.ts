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

    let virtualMenuGeneration = 0;
    let virtualMenuTimer: ReturnType<typeof setTimeout> | undefined;
    const readVirtualMenuEvent = (message: unknown): {
        created: boolean;
        controllerIndex: number;
        menuIndex: number;
        key: string;
    } | undefined => {
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
        const fields = new Map<number, number>();
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
                fields.set(field, value);
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
        const created = fields.get(1);
        if (created === undefined) return undefined;
        const controllerIndex = fields.get(2) ?? 0;
        const menuIndex = fields.get(3) ?? 0;
        return {
            created: created !== 0,
            controllerIndex,
            menuIndex,
            key: `${controllerIndex}:${menuIndex}`
        };
    };

    const input = window.SteamClient?.Input ?? (opener as SteamWindow)?.SteamClient?.Input;
    const activeVirtualMenus = new Set<string>();
    const virtualMenuRegistration = input?.RegisterForTouchMenuMessages?.((message: unknown) => {
        const event = readVirtualMenuEvent(message);
        diagnostic({
            kind: 'menu_message', parsed: !!event,
            messageType: typeof message,
            length: typeof message === 'string' ? message.length
                : ArrayBuffer.isView(message) ? message.byteLength
                : message instanceof ArrayBuffer ? message.byteLength : 0
        });
        if (!event) return;
        diagnostic({ kind: 'menu_lifecycle', controller: event.controllerIndex, menu: event.menuIndex, created: event.created });
        const generation = ++virtualMenuGeneration;
        if (virtualMenuTimer) clearTimeout(virtualMenuTimer);
        if (event.created) {
            activeVirtualMenus.add(event.key);
            void client.POST('/api/v1/steam/cef/virtual-menu-visibility', {
                body: { visible: true }
            }).catch((e) => console.error('Error restoring virtual menu visibility', e));
        } else {
            // Steam removes the menu on an action-set change without clearing
            // the selected radial segment. That stale selection consumes the
            // later release and leaves the menu visible but produces no output.
            input?.SetVirtualMenuKeySelected?.(event.controllerIndex, event.menuIndex, -1);
            activeVirtualMenus.delete(event.key);
            if (activeVirtualMenus.size > 0) return;
            virtualMenuTimer = setTimeout(() => {
                if (generation !== virtualMenuGeneration) return;
                void client.POST('/api/v1/steam/cef/virtual-menu-visibility', {
                    body: { visible: false }
                })
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
                diagnostic({ kind: 'overlay', open: overlayOpen });
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
