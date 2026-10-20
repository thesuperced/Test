import { client } from '$lib/api/client';
import { readVirtualMenuEvent, VirtualMenuState } from '$lib/virtual-menu';

__INJECT_RETURN = (async () => {
    // Injected into "normal" overlay-tab or SharedJSContext

    if (!window.__sisrCleanup) {
        window.__sisrCleanup = {};
    }
    const steamClientOverlay = window.SteamClient?.Overlay ?? (opener as SteamWindow)?.SteamClient?.Overlay;

    if (window.__sisrCleanup.overlayCallback) {
        window.__sisrCleanup.overlayCallback();
    }

    window.__sisrCleanup.virtualMenuCallback?.();
    const input = window.SteamClient?.Input ?? (opener as SteamWindow)?.SteamClient?.Input;
    const state = new VirtualMenuState();
    let stopped = false;
    let sending = false;
    let revision = 0;
    let lastSent: boolean | undefined;
    let lastRefresh = 0;
    const publish = (force = false) => {
        const now = Date.now();
        const visible = state.visible(now);
        if (!force && visible === lastSent && (!visible || now - lastRefresh < 1000)) return;
        lastSent = visible;
        lastRefresh = now;
        revision++;
        if (sending) return;
        sending = true;
        // One request at a time, coalescing updates to the current state.
        void (async () => {
            try {
                let sentRevision: number;
                do {
                    sentRevision = revision;
                    if (stopped) return;
                    const result = await client.POST('/api/v1/steam/cef/virtual-menu-visibility', {
                        body: { visible: state.visible(Date.now()) },
                        signal: AbortSignal.timeout(2500)
                    });
                    if (result.error) throw new Error('Virtual menu visibility request failed');
                } while (sentRevision !== revision);
            } catch (e) {
                lastSent = undefined; // retry on the next watchdog tick
                console.error('Virtual menu visibility update failed', e);
            } finally {
                sending = false;
            }
        })();
    };
    const registrations: { unregister?: () => void }[] = [];
    // Only enable control if both lifecycle and touch-state notifications exist.
    if (input?.RegisterForTouchMenuMessages && input?.RegisterForTouchMenuInputMessages) {
        registrations.push(input.RegisterForTouchMenuMessages((message: unknown) => {
            const event = readVirtualMenuEvent(message);
            if (!event) return;
            state.lifecycle(event.key, event.created);
            publish(true);
        }));
        registrations.push(input.RegisterForTouchMenuInputMessages(
            (controller, menu, _x, _y, active) => {
                state.input(`${controller}:${menu}`, active, Date.now());
                publish();
            }
        ));
        publish(true);
    }
    const watchdog = setInterval(() => {
        if (registrations.length) publish();
    }, 250);
    window.__sisrCleanup.virtualMenuCallback = () => {
        stopped = true;
        clearInterval(watchdog);
        for (const registration of registrations) registration?.unregister?.();
        delete window.__sisrCleanup?.virtualMenuCallback;
    };

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
