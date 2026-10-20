// CVirtualMenuCreateDestroy: created=1, controller_idx=2, menu_idx=3.
// Protobuf omits default values, including created=false and index zero.
export function readVirtualMenuEvent(message: unknown) {
    let bytes: Uint8Array;
    if (message instanceof ArrayBuffer) bytes = new Uint8Array(message);
    else if (ArrayBuffer.isView(message)) {
        bytes = new Uint8Array(message.buffer, message.byteOffset, message.byteLength);
    } else if (typeof message === 'string') {
        try { bytes = Uint8Array.from(atob(message), (c) => c.charCodeAt(0)); }
        catch { return; }
    } else if (Array.isArray(message) && message.every((v) => Number.isInteger(v) && v >= 0 && v <= 255)) {
        bytes = new Uint8Array(message);
    } else return;

    let offset = 0;
    const fields = new Map<number, number>();
    const readVarint = () => {
        let value = 0;
        for (let shift = 0; shift < 49 && offset < bytes.length; shift += 7) {
            const byte = bytes[offset++];
            value += (byte & 127) * 2 ** shift;
            if (!(byte & 128)) return value;
        }
        throw new Error('truncated protobuf');
    };
    try {
        while (offset < bytes.length) {
            const tag = readVarint();
            const field = Math.floor(tag / 8);
            if (!field) return;
            switch (tag % 8) {
                case 0: fields.set(field, readVarint()); break;
                case 1: offset += 8; break;
                case 2: offset += readVarint(); break;
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
