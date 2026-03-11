<script lang="ts">
	import { page } from '$app/state';
	import { dashboardStatus, questHealths } from '$lib/stores/quests';
	import { fetchErrands, fetchTome } from '$lib/api';
	import PhaseTimeline from '$lib/components/PhaseTimeline.svelte';
	import GateActions from '$lib/components/GateActions.svelte';
	import ErrandList from '$lib/components/ErrandList.svelte';
	import HeraldFeed from '$lib/components/HeraldFeed.svelte';
	import { tidings } from '$lib/stores/herald';
	import type { QuestStatus, QuestTome, QuestErrandList } from '$lib/types';

	let questName = $derived(decodeURIComponent(page.params.id));
	let quest = $derived($dashboardStatus?.quests.find((q) => q.name === questName));
	let health = $derived($questHealths.find((h) => h.name === questName));
	let questTidings = $derived($tidings.filter((t) => t.quest === questName));

	let errandList = $state<QuestErrandList | null>(null);
	let tomeData = $state<QuestTome | null>(null);
	let activeTab = $state<'errands' | 'files' | 'tome' | 'herald' | 'logs'>('errands');

	const phases = ['Onboard', 'Research', 'Plan', 'Implement', 'Review', 'Complete'];

	let dataLoaded = $state(false);

	$effect(() => {
		if (quest && !dataLoaded) {
			dataLoaded = true;
			Promise.all([
				fetchErrands(quest.worktree),
				fetchTome(questName),
			]).then(([e, t]) => {
				errandList = e as QuestErrandList;
				tomeData = t as QuestTome;
			}).catch(() => {
				// Failed to load quest data
				dataLoaded = false;
			});
		}
	});

	function phaseStatus(p: string): 'done' | 'current' | 'pending' {
		if (!quest) return 'pending';
		const ci = phases.indexOf(quest.phase);
		const pi = phases.indexOf(p);
		if (pi < ci) return 'done';
		if (pi === ci) return 'current';
		return 'pending';
	}
</script>

<div class="detail-view">
	<div class="detail-header">
		<div class="breadcrumb">
			<a href="/command">Command</a>
			<span class="sep">/</span>
			<span>{questName}</span>
		</div>

		{#if quest}
			<div class="title-row">
				<h1>{quest.name}</h1>
				<span class="phase-badge">{quest.phase}</span>
				{#if quest.gate_pending}
					<GateActions worktree={quest.worktree} />
				{/if}
				{#if health}
					<span class="health-dot {health.health}"></span>
					<span class="health-label">{health.health}</span>
				{/if}
			</div>

			<div class="phase-steps">
				{#each phases as p}
					<div class="phase-step {phaseStatus(p)}">
						<span class="phase-step-label">{p}</span>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<div class="tabs">
		<button class="tab" class:active={activeTab === 'errands'} onclick={() => activeTab = 'errands'}>
			Errands
			{#if errandList}
				<span class="tab-badge">{errandList.items.filter(e => e.status === 'done').length}/{errandList.items.length}</span>
			{/if}
		</button>
		<button class="tab" class:active={activeTab === 'files'} onclick={() => activeTab = 'files'}>Files</button>
		<button class="tab" class:active={activeTab === 'tome'} onclick={() => activeTab = 'tome'}>Tome</button>
		<button class="tab" class:active={activeTab === 'herald'} onclick={() => activeTab = 'herald'}>Herald</button>
		<button class="tab" class:active={activeTab === 'logs'} onclick={() => activeTab = 'logs'}>Logs</button>
	</div>

	<div class="detail-body">
		<div class="detail-content">
			{#if activeTab === 'errands' && errandList}
				<ErrandList errands={errandList.items} />
			{:else if activeTab === 'files' && tomeData}
				<div class="files-list">
					{#each tomeData.files_touched as file}
						<div class="file-item">{file}</div>
					{/each}
				</div>
			{:else if activeTab === 'tome' && tomeData}
				<div class="tome-content">
					<h3>Gate History</h3>
					{#each tomeData.gate_history as gate}
						<div class="tome-entry">
							<span class="tome-action {gate.action}">{gate.action}</span>
							<span class="tome-phase">{gate.phase}</span>
							<span class="tome-time">{gate.timestamp}</span>
						</div>
					{/each}
				</div>
			{:else if activeTab === 'herald'}
				<HeraldFeed tidings={questTidings} limit={50} />
			{:else if activeTab === 'logs'}
				<div class="logs-placeholder">
					<p class="empty">Quest logs not yet available. Raw output logging is a future enhancement.</p>
				</div>
			{:else}
				<p class="empty">No data available</p>
			{/if}
		</div>

		{#if quest}
			<div class="detail-meta">
				<div class="meta-card">
					<div class="meta-title">Metadata</div>
					<div class="meta-row">
						<span>Branch</span>
						<span class="meta-value mono">{quest.worktree.split('/').pop()}</span>
					</div>
					<div class="meta-row">
						<span>Worktree</span>
						<span class="meta-value mono truncate" title={quest.worktree}>{quest.worktree}</span>
					</div>
					<div class="meta-row">
						<span>Status</span>
						<span class="meta-value">{quest.status}</span>
					</div>
					{#if health}
						<div class="meta-row">
							<span>Eagles</span>
							<span class="meta-value {health.health}">{health.health}</span>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>

<style>
	.detail-view {
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	.detail-header {
		padding: var(--space-md) var(--space-lg) 0;
		border-bottom: 1px solid var(--border);
	}

	.breadcrumb {
		font-size: 12px;
		color: var(--text-muted);
		margin-bottom: 10px;
	}

	.breadcrumb a {
		color: var(--text-faint);
	}

	.sep { margin: 0 6px; color: var(--border-active); }

	.title-row {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 14px;
	}

	.title-row h1 {
		font-family: var(--font-heading);
		font-size: 18px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.phase-badge {
		padding: 3px 10px;
		border-radius: 12px;
		font-size: 11px;
		font-weight: 500;
		background: var(--accent-green-dim);
		color: var(--accent-green-text);
	}

	.health-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		margin-left: auto;
	}

	.health-dot.working { background: var(--accent-green-text); }
	.health-dot.stalled { background: var(--accent-gold); }
	.health-dot.zombie { background: var(--accent-red); }

	.health-label {
		font-size: 12px;
		color: var(--accent-green-text);
	}

	.phase-steps {
		display: flex;
		margin-bottom: -1px;
	}

	.phase-step {
		flex: 1;
		text-align: center;
		padding: 10px 0 12px;
		font-size: 11px;
		color: var(--text-faint);
		border-bottom: 2px solid transparent;
	}

	.phase-step.done { color: var(--text-muted); border-bottom-color: var(--accent-green); }
	.phase-step.current { color: var(--accent-gold); border-bottom-color: var(--accent-gold); }

	.tabs {
		display: flex;
		padding: 0 var(--space-lg);
		border-bottom: 1px solid var(--border);
	}

	.tab {
		padding: 10px 16px;
		font-size: 12px;
		color: var(--text-muted);
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		transition: all var(--transition-fast);
	}

	.tab.active {
		color: var(--text-secondary);
		border-bottom-color: var(--accent-gold);
	}

	.tab-badge {
		font-size: 10px;
		padding: 0px 5px;
		border-radius: 8px;
		background: var(--accent-gold-dim);
		color: var(--accent-gold);
		margin-left: 5px;
	}

	.detail-body {
		flex: 1;
		display: flex;
		gap: var(--space-lg);
		padding: var(--space-lg);
		overflow: hidden;
	}

	.detail-content {
		flex: 2;
		overflow-y: auto;
	}

	.detail-meta {
		flex: 1;
		max-width: 280px;
	}

	.meta-card {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 14px;
	}

	.meta-title {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--text-faint);
		margin-bottom: 10px;
	}

	.meta-row {
		display: flex;
		justify-content: space-between;
		font-size: 12px;
		padding: 5px 0;
		color: var(--text-muted);
		border-bottom: 1px solid var(--border);
	}

	.meta-row:last-child { border-bottom: none; }

	.meta-value {
		color: var(--text-secondary);
		font-size: 11px;
		max-width: 160px;
	}

	.meta-value.working { color: var(--accent-green-text); }
	.meta-value.stalled { color: var(--accent-gold); }

	.files-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.file-item {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
		padding: 6px 10px;
		background: var(--bg-surface);
		border-radius: var(--radius-sm);
	}

	.tome-content h3 {
		font-family: var(--font-heading);
		font-size: 14px;
		color: var(--text-primary);
		margin-bottom: var(--space-md);
	}

	.tome-entry {
		display: flex;
		gap: 12px;
		padding: 8px 0;
		border-bottom: 1px solid var(--border);
		font-size: 12px;
	}

	.tome-action {
		font-weight: 500;
		width: 70px;
	}

	.tome-action.approved { color: var(--accent-green-text); }
	.tome-action.rejected { color: var(--accent-red); }
	.tome-action.submitted { color: var(--accent-gold); }

	.tome-phase { color: var(--text-secondary); }
	.tome-time { color: var(--text-faint); font-family: var(--font-mono); font-size: 11px; margin-left: auto; }

	.empty {
		color: var(--text-muted);
		text-align: center;
		padding: var(--space-xl);
	}
</style>
