import { writable } from 'svelte/store';
import { lastEvent } from './websocket';
import type { DashboardError } from '$lib/types';

export const errors = writable<DashboardError[]>([]);

export async function fetchErrors() {
	try {
		const res = await fetch('/api/errors');
		if (res.ok) errors.set(await res.json());
	} catch { /* offline */ }
}

export async function clearErrors() {
	try {
		const res = await fetch('/api/errors', { method: 'DELETE' });
		if (res.ok) errors.set([]);
	} catch { /* offline */ }
}

let unsubscribe: (() => void) | null = null;

export function startErrorPolling() {
	if (unsubscribe) return;
	fetchErrors();
	unsubscribe = lastEvent.subscribe((event) => {
		if (!event) return;
		if (event.type === 'error-logged') {
			fetchErrors();
		}
	});
}

export function stopErrorPolling() {
	unsubscribe?.();
	unsubscribe = null;
}
