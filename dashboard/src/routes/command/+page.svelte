<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { startWebSocket, stopWebSocket } from '$lib/stores/websocket';
	import { dashboardStatus, questHealths, problems, activeQuests, pendingGates, activeScouts, refreshAll, startPolling, stopPolling } from '$lib/stores/quests';
	import { tidings, startHeraldPolling, stopHeraldPolling } from '$lib/stores/herald';
	import ConnectionBanner from '$lib/components/ConnectionBanner.svelte';
	import StatCounter from '$lib/components/StatCounter.svelte';
	import QuestCard from '$lib/components/QuestCard.svelte';
	import HeraldFeed from '$lib/components/HeraldFeed.svelte';

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

	function getHealth(questName: string) {
		return $questHealths.find((h) => h.name === questName);
	}
</script>

<ConnectionBanner />

<div class="command-view">
	<div class="view-header">
		<h1>Command</h1>
	</div>

	<div class="stats-row">
		<StatCounter value={$activeQuests.length} label="Active Quests" color="var(--accent-green-text)" />
		<StatCounter value={$pendingGates.length} label="Gates Pending" color="var(--accent-gold)" />
		<StatCounter value={$activeScouts.length} label="Scouts Active" color="var(--accent-blue)" />
		<StatCounter value={$problems.length} label="Alerts" color={$problems.length > 0 ? 'var(--accent-red)' : 'var(--accent-purple)'} />
	</div>

	<div class="content-split">
		<div class="quest-grid">
			{#if $dashboardStatus}
				{#each $dashboardStatus.quests as quest (quest.name)}
					<QuestCard {quest} health={getHealth(quest.name)} />
				{/each}
				{#each $dashboardStatus.scouts as scout (scout.name)}
					<div class="scout-card">
						<div class="scout-header">
							<span class="scout-icon">📡</span>
							<span class="scout-name">{scout.name}</span>
						</div>
						<p class="scout-question">{scout.question}</p>
					</div>
				{/each}
			{:else}
				<div class="empty-state">
					<p>Waiting for fellowship data...</p>
				</div>
			{/if}
		</div>

		<div class="herald-panel">
			<div class="panel-header">Herald</div>
			<HeraldFeed tidings={$tidings} />
		</div>
	</div>
</div>

<style>
	.command-view {
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

	.stats-row {
		display: flex;
		gap: 12px;
	}

	.content-split {
		display: flex;
		gap: var(--space-lg);
		flex: 1;
		min-height: 0;
	}

	.quest-grid {
		flex: 2;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 12px;
		align-content: start;
	}

	.herald-panel {
		flex: 1;
		min-width: 240px;
		max-width: 320px;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 14px;
		overflow-y: auto;
	}

	.panel-header {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--text-faint);
		margin-bottom: 12px;
	}

	.scout-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 14px 16px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.scout-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.scout-name {
		font-size: 13px;
		font-weight: 500;
		color: var(--accent-blue);
	}

	.scout-question {
		font-size: 12px;
		color: var(--text-muted);
		line-height: 1.4;
	}

	.empty-state {
		grid-column: 1 / -1;
		text-align: center;
		padding: var(--space-2xl);
		color: var(--text-muted);
	}
</style>
