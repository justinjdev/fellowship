import { writable, derived } from 'svelte/store';
import { lastEvent } from './websocket';
import type { DashboardStatus, QuestHealth, Problem } from '$lib/types';

export const dashboardStatus = writable<DashboardStatus | null>(null);
export const questHealths = writable<QuestHealth[]>([]);
export const problems = writable<Problem[]>([]);

export const activeQuests = derived(dashboardStatus, ($status) =>
	$status?.quests.filter((q) => q.status === 'active') ?? []
);

export const pendingGates = derived(dashboardStatus, ($status) =>
	$status?.quests.filter((q) => q.gate_pending) ?? []
);

export const activeScouts = derived(dashboardStatus, ($status) =>
	$status?.scouts ?? []
);

async function fetchStatus() {
	try {
		const res = await fetch('/api/status');
		if (res.ok) dashboardStatus.set(await res.json());
	} catch { /* offline */ }
}

async function fetchHealth() {
	try {
		const res = await fetch('/api/eagles');
		if (res.ok) {
			const data = await res.json();
			questHealths.set(Array.isArray(data?.quests) ? data.quests : []);
		}
	} catch { /* offline */ }
}

async function fetchProblems() {
	try {
		const res = await fetch('/api/herald/problems');
		if (res.ok) problems.set(await res.json());
	} catch { /* offline */ }
}

export async function refreshAll() {
	await Promise.all([fetchStatus(), fetchHealth(), fetchProblems()]);
}

let unsubscribe: (() => void) | null = null;

export function startPolling() {
	if (unsubscribe) return;
	refreshAll();
	unsubscribe = lastEvent.subscribe((event) => {
		if (!event) return;
		switch (event.type) {
			case 'quest-changed':
			case 'gate-submitted':
			case 'gate-resolved':
			case 'command-completed':
				refreshAll();
				break;
			case 'alert':
				fetchProblems();
				break;
			case 'herald-event':
				break;
		}
	});
}

export function stopPolling() {
	unsubscribe?.();
	unsubscribe = null;
}
