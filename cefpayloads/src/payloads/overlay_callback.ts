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
    // competing native menu consumer or manipulate reusable menu pages.
    window.__sisrCleanup.virtualMenuCallback?.();

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
