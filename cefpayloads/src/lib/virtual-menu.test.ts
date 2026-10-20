import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readVirtualMenuEvent, VirtualMenuState } from './virtual-menu.ts';

test('protobuf defaults represent destroy and zero indices', () => {
    assert.deepEqual(readVirtualMenuEvent(new Uint8Array([16, 2])), { created: false, key: '2:0' });
    assert.deepEqual(readVirtualMenuEvent(new Uint8Array()), { created: false, key: '0:0' });
    assert.deepEqual(readVirtualMenuEvent(new Uint8Array([8, 1, 16, 2, 24, 3])), { created: true, key: '2:3' });
    assert.deepEqual(readVirtualMenuEvent('EAI='), { created: false, key: '2:0' });
    assert.deepEqual(readVirtualMenuEvent([16, 2]), { created: false, key: '2:0' });
    assert.equal(readVirtualMenuEvent('not base64!'), undefined);
});
test('rejects truncated fields instead of interpreting them as destroy', () => {
    for (const bytes of [[8], [8, 128], [34, 5, 0], [9, 0], [0]]) {
        assert.equal(readVirtualMenuEvent(new Uint8Array(bytes)), undefined);
    }
});
test('four-set cycle cancels old touch and rearms only after release', () => {
    const s = new VirtualMenuState();
    s.lifecycle('0:1', true);
    assert.equal(s.visible(0), false);
    s.input('0:1', true, 0);
    assert.equal(s.visible(0), true);
    s.lifecycle('0:1', false);
    assert.equal(s.visible(0), false);
    s.lifecycle('0:2', true);
    s.lifecycle('0:2', false);
    s.lifecycle('0:3', true);
    s.lifecycle('0:3', false);
    s.lifecycle('0:1', true);
    s.input('0:1', true, 10);
    assert.equal(s.visible(10), false);
    s.input('0:1', false, 11);
    s.input('0:1', true, 12);
    assert.equal(s.visible(12), true);
    s.input('0:1', false, 13);
    assert.equal(s.visible(13), false);
});
test('watchdog expires missing release; stale active cannot revive menu', () => {
    const s = new VirtualMenuState();
    s.lifecycle('0:1', true);
    s.input('0:1', true, 0);
    assert.equal(s.visible(4999), true);
    assert.equal(s.visible(5000), false);
    s.input('0:1', true, 6000);
    assert.equal(s.visible(6000), false);
    s.input('0:1', false, 6001);
    s.input('0:1', true, 6002);
    assert.equal(s.visible(6002), true);
});
