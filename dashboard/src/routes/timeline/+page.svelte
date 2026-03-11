<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { startWebSocket, stopWebSocket } from '$lib/stores/websocket';
	import { dashboardStatus, startPolling, stopPolling } from '$lib/stores/quests';
	import { fetchTome } from '$lib/api';
	import GanttChart from '$lib/components/GanttChart.svelte';
	import type { QuestTome } from '$lib/types';

	let tomes = $state<QuestTome[]>([]);
	let loading = $state(true);

	async function loadTomes() {
		const status = $dashboardStatus;
		if (!status) return;

		const names = status.quests.map((q) => q.name);
		const results = await Promise.allSettled(names.map((n) => fetchTome(n)));
		tomes = results
			.filter((r): r is PromiseFulfilledResult<unknown> => r.status === 'fulfilled')
			.map((r) => r.value)
			.filter((t): t is QuestTome => t != null) as QuestTome[];
		loading = false;
	}

	let unsub: (() => void) | null = null;

	onMount(() => {
		startWebSocket();
		startPolling();

		unsub = dashboardStatus.subscribe((status) => {
			if (status) loadTomes();
		});
	});

	onDestroy(() => {
		stopWebSocket();
		stopPolling();
		unsub?.();
	});
</script>

<div class="timeline-view">
	<div class="view-header">
		<h1>Timeline</h1>
	</div>

	<div class="chart-area">
		{#if loading}
			<div class="empty-state">Loading quest timeline data...</div>
		{:else if tomes.length === 0}
			<div class="empty-state">No quest timeline data available.</div>
		{:else}
			<GanttChart quests={tomes} />
		{/if}
	</div>
</div>

<style>
	.timeline-view {
		padding: var(--space-lg);
		display: flex;
		flex-direction: column;
		gap: var(--space-lg);
		height: 100%;
	}

	.view-header h1 {
		font-family: var(--font-heading);
		font-size: 18px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.chart-area {
		flex: 1;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-lg);
		overflow-y: auto;
	}

	.empty-state {
		text-align: center;
		padding: var(--space-2xl);
		color: var(--text-muted);
		font-size: 13px;
	}
</style>
