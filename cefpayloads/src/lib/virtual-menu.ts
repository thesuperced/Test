// CVirtualMenuCreateDestroy: created=1, controller_idx=2, menu_idx=3.
// Protobuf omits default values, including created=false and index zero.
export function readVirtualMenuEvent(message: unknown) {
    let bytes: Uint8Array;
    if (message instanceof ArrayBuffer) bytes = new Uint8Array(message);
    else if (ArrayBuffer.isView(message))
        bytes = new Uint8Array(message.buffer, message.byteOffset, message.byteLength);
    else if (typeof message === 'string') {
        // jspb also accepts base64 strings from native Steam callbacks.
        try { bytes = Uint8Array.from(atob(message), (c) => c.charCodeAt(0)); }
        catch { return; }
    } else if (Array.isArray(message) && message.every((v) => Number.isInteger(v) && v >= 0 && v <= 255)) {
        bytes = new Uint8Array(message);
    }
    else return;
    let offset = 0;
    const readVarint = () => {
        let value = 0;
        for (let shift = 0; shift < 49 && offset < bytes.length; shift += 7) {
            const byte = bytes[offset++];
            value += (byte & 127) * 2 ** shift;
            if (!(byte & 128)) return value;
        }
        throw new Error('Truncated or oversized varint');
    };
    const fields = new Map<number, number>();
    try {
        while (offset < bytes.length) {
            const tag = readVarint();
            const field = Math.floor(tag / 8);
            if (!field) return;
            switch (tag % 8) {
                case 0: fields.set(field, readVarint()); break;
                case 1: offset += 8; break;
                case 2: { const length = readVarint(); offset += length; break; }
                case 5: offset += 4; break;
                default: return;
            }
            if (offset > bytes.length) return;
        }
    } catch { return; }
    return {
        created: (fields.get(1) ?? 0) !== 0,
        key: `${fields.get(2) ?? 0}:${fields.get(3) ?? 0}`
    };
}

// A menu's existence is not evidence of active touch input.
export class VirtualMenuState {
    known = new Set<string>();
    active = new Map<string, number>();
    blocked = new Set<string>();
    lifecycle(key: string, created: boolean) {
        // Cancel every visible menu at a lifecycle boundary, including ones
        // whose destroy notification was missed. Require release before reuse.
        for (const activeKey of this.active.keys()) this.blocked.add(activeKey);
        this.active.clear();
        if (created) this.known.add(key);
        else this.known.delete(key);
    }
    input(key: string, active: boolean, now: number) {
        if (!active) {
            this.active.delete(key);
            this.blocked.delete(key);
        } else if (this.known.has(key) && !this.blocked.has(key)) {
            this.active.set(key, now);
        }
    }
    visible(now: number) {
        for (const [key, lastInput] of this.active) {
            if (now - lastInput >= 5000) {
                this.active.delete(key);
                this.blocked.add(key);
            }
        }
        return this.active.size > 0;
    }
}
