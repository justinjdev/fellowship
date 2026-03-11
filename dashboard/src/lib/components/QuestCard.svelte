<script lang="ts">
	import PhaseTimeline from './PhaseTimeline.svelte';
	import GateActions from './GateActions.svelte';
	import type { QuestStatus, QuestHealth } from '$lib/types';

	let { quest, health }: { quest: QuestStatus; health?: QuestHealth } = $props();
</script>

<a href="/quest/{encodeURIComponent(quest.name)}" class="quest-card" class:gate-pending={quest.gate_pending}>
	<div class="card-header">
		<span class="quest-name">{quest.name}</span>
		{#if health}
			<span class="health-badge {health.health}">{health.health}</span>
		{/if}
	</div>

	<PhaseTimeline phase={quest.phase} compact />

	<div class="card-meta">
		<span class="phase-label">{quest.phase}</span>
		{#if quest.errands_total > 0}
			<span class="errand-count">{quest.errands_done}/{quest.errands_total} errands</span>
		{/if}
	</div>

	{#if quest.gate_pending}
		<div class="gate-row" onclick={(e) => e.preventDefault()}>
			<GateActions worktree={quest.worktree} />
		</div>
	{/if}
</a>

<style>
	.quest-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 14px 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		text-decoration: none;
		color: inherit;
		transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
	}

	.quest-card:hover {
		border-color: var(--border-hover);
		text-decoration: none;
	}

	.quest-card.gate-pending {
		border-color: var(--accent-gold-dim);
		box-shadow: 0 0 12px var(--accent-gold-glow);
	}

	.card-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.quest-name {
		font-size: 13px;
		font-weight: 500;
		color: var(--text-primary);
	}

	.health-badge {
		font-size: 10px;
		padding: 1px 8px;
		border-radius: 10px;
		font-weight: 500;
	}

	.health-badge.working { background: var(--accent-green-dim); color: var(--accent-green-text); }
	.health-badge.stalled { background: var(--accent-gold-dim); color: var(--accent-gold); }
	.health-badge.zombie { background: var(--accent-red-dim); color: var(--accent-red); }
	.health-badge.idle { background: var(--bg-raised); color: var(--text-muted); }
	.health-badge.complete { background: var(--accent-green-dim); color: var(--accent-green-text); }

	.card-meta {
		display: flex;
		gap: 12px;
		font-size: 11px;
		color: var(--text-muted);
	}

	.phase-label {
		color: var(--text-secondary);
	}

	.gate-row {
		border-top: 1px solid var(--border);
		padding-top: 8px;
	}
</style>
