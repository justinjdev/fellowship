import { writable } from 'svelte/store';
import { lastEvent } from './websocket';
import type { Tiding } from '$lib/types';

export const tidings = writable<Tiding[]>([]);

export async function fetchTidings() {
	try {
		const res = await fetch('/api/herald');
		if (res.ok) tidings.set(await res.json());
	} catch { /* offline */ }
}

let unsubscribe: (() => void) | null = null;

export function startHeraldPolling() {
	if (unsubscribe) return;
	fetchTidings();
	unsubscribe = lastEvent.subscribe((event) => {
		if (!event) return;
		if (event.type === 'herald-event' || event.type === 'gate-resolved' || event.type === 'quest-changed') {
			fetchTidings();
		}
	});
}

export function stopHeraldPolling() {
	unsubscribe?.();
	unsubscribe = null;
}
