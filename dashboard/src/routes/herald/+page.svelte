<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { startWebSocket, stopWebSocket } from '$lib/stores/websocket';
	import { startPolling, stopPolling } from '$lib/stores/quests';
	import { tidings, startHeraldPolling, stopHeraldPolling } from '$lib/stores/herald';
	import HeraldFeed from '$lib/components/HeraldFeed.svelte';
	import type { Tiding } from '$lib/types';

	let searchText = $state('');
	let selectedQuest = $state('');
	let enabledTypes = $state<Record<string, boolean>>({
		gate_submitted: true,
		gate_approved: true,
		gate_rejected: true,
		phase_transition: true,
	});

	const eventTypes = ['gate_submitted', 'gate_approved', 'gate_rejected', 'phase_transition'] as const;

	const eventTypeLabels: Record<string, string> = {
		gate_submitted: 'Gate Submitted',
		gate_approved: 'Gate Approved',
		gate_rejected: 'Gate Rejected',
		phase_transition: 'Phase Transition',
	};

	let allTidings: Tiding[] = $derived($tidings);

	let questNames = $derived(
		[...new Set(allTidings.map((t) => t.quest))].filter(Boolean).sort()
	);

	let filtered = $derived(
		allTidings.filter((t) => {
			if (selectedQuest && t.quest !== selectedQuest) return false;
			const anyTypeEnabled = Object.values(enabledTypes).some(Boolean);
			if (anyTypeEnabled && !enabledTypes[t.type]) return false;
			if (searchText) {
				const q = searchText.toLowerCase();
				const haystack = `${t.quest} ${t.type} ${t.phase} ${t.detail}`.toLowerCase();
				if (!haystack.includes(q)) return false;
			}
			return true;
		})
	);

	onMount(() => {
		startWebSocket();
		startPolling();
		startHeraldPolling();
	});

	onDestroy(() => {
		stopWebSocket();
		stopPolling();
		stopHeraldPolling();
	});
</script>

<div class="herald-view">
	<div class="view-header">
		<h1>Herald</h1>
		<span class="tiding-count">{filtered.length} tidings</span>
	</div>

	<div class="filters">
		<div class="filter-row">
			<input
				type="text"
				class="search-input"
				placeholder="Search tidings..."
				bind:value={searchText}
			/>

			<select class="quest-select" bind:value={selectedQuest}>
				<option value="">All Quests</option>
				{#each questNames as name}
					<option value={name}>{name}</option>
				{/each}
			</select>
		</div>

		<div class="type-filters">
			{#each eventTypes as etype}
				<label class="type-checkbox">
					<input type="checkbox" bind:checked={enabledTypes[etype]} />
					<span class="type-label">{eventTypeLabels[etype]}</span>
				</label>
			{/each}
		</div>
	</div>

	<div class="feed-scroll">
		{#if filtered.length > 0}
			<HeraldFeed tidings={filtered} limit={filtered.length} />
		{:else}
			<div class="empty-state">
				<p>No tidings match the current filters.</p>
			</div>
		{/if}
	</div>
</div>

<style>
	.herald-view {
		padding: var(--space-lg);
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
		height: 100%;
	}

	.view-header {
		display: flex;
		align-items: baseline;
		gap: var(--space-md);
	}

	.view-header h1 {
		font-family: var(--font-heading);
		font-size: 18px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.tiding-count {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
	}

	.filters {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-md);
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
	}

	.filter-row {
		display: flex;
		gap: var(--space-sm);
	}

	.search-input {
		flex: 1;
		background: var(--bg-raised);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text-primary);
		outline: none;
		transition: border-color var(--transition-fast);
	}

	.search-input::placeholder {
		color: var(--text-faint);
	}

	.search-input:focus {
		border-color: var(--border-active);
	}

	.quest-select {
		background: var(--bg-raised);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 12px;
		font-family: var(--font-body);
		font-size: 13px;
		color: var(--text-primary);
		outline: none;
		min-width: 160px;
		cursor: pointer;
	}

	.quest-select option {
		background: var(--bg-raised);
		color: var(--text-primary);
	}

	.type-filters {
		display: flex;
		gap: var(--space-md);
		flex-wrap: wrap;
	}

	.type-checkbox {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		cursor: pointer;
	}

	.type-checkbox input[type='checkbox'] {
		accent-color: var(--accent-gold);
		cursor: pointer;
	}

	.type-label {
		font-size: 12px;
		color: var(--text-secondary);
		user-select: none;
	}

	.feed-scroll {
		flex: 1;
		overflow-y: auto;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-md);
	}

	.empty-state {
		text-align: center;
		padding: var(--space-2xl);
		color: var(--text-muted);
		font-size: 13px;
	}
</style>
