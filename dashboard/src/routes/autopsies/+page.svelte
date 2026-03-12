<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchAutopsies } from '$lib/api';
	import type { Autopsy } from '$lib/types';

	let autopsies = $state<Autopsy[]>([]);
	let searchText = $state('');
	let expandedIndex = $state<number | null>(null);
	let loading = $state(true);
	let loadError = $state(false);

	let filtered = $derived(
		autopsies.filter((a) => {
			if (!searchText) return true;
			const q = searchText.toLowerCase();
			const haystack = `${a.quest} ${a.task} ${a.what_failed} ${a.trigger} ${(a.tags ?? []).join(' ')}`.toLowerCase();
			return haystack.includes(q);
		})
	);

	function formatDate(ts: string): string {
		try {
			return new Date(ts).toLocaleDateString('en-US', {
				month: 'short',
				day: 'numeric',
				year: 'numeric',
				hour: '2-digit',
				minute: '2-digit',
			});
		} catch {
			return ts;
		}
	}

	function toggle(idx: number) {
		expandedIndex = expandedIndex === idx ? null : idx;
	}

	onMount(async () => {
		try {
			autopsies = (await fetchAutopsies()) as Autopsy[];
		} catch {
			loadError = true;
		} finally {
			loading = false;
		}
	});
</script>

<div class="autopsies-view">
	<div class="view-header">
		<h1>Autopsies</h1>
		<span class="count">{filtered.length} records</span>
	</div>

	<div class="search-bar">
		<input
			type="text"
			class="search-input"
			placeholder="Search by quest, task, failure, trigger, or tag..."
			bind:value={searchText}
		/>
	</div>

	<div class="list-scroll">
		{#if loading}
			<div class="empty-state">Loading autopsies...</div>
		{:else if loadError}
			<div class="empty-state error">Failed to load autopsies. Is the server running?</div>
		{:else if filtered.length === 0}
			<div class="empty-state">No autopsies found.</div>
		{:else}
			{#each filtered as autopsy, idx}
				<button class="autopsy-row" class:expanded={expandedIndex === idx} onclick={() => toggle(idx)}>
					<div class="autopsy-summary">
						<span class="autopsy-quest">{autopsy.quest}</span>
						<span class="autopsy-date">{formatDate(autopsy.ts)}</span>
						<span class="autopsy-failure">{autopsy.what_failed}</span>
						<span class="autopsy-trigger">{autopsy.trigger}</span>
						<span class="expand-icon">{expandedIndex === idx ? '−' : '+'}</span>
					</div>

					{#if expandedIndex === idx}
						<div class="autopsy-detail">
							<div class="detail-section">
								<div class="detail-label">Task</div>
								<div class="detail-value">{autopsy.task}</div>
							</div>
							<div class="detail-section">
								<div class="detail-label">Phase</div>
								<div class="detail-value">{autopsy.phase}</div>
							</div>
							<div class="detail-section">
								<div class="detail-label">Resolution</div>
								<div class="detail-value detail-resolution">{autopsy.resolution}</div>
							</div>
							{#if autopsy.files.length > 0}
								<div class="detail-section">
									<div class="detail-label">Files</div>
									<div class="detail-value">
										{#each autopsy.files as file}
											<code class="file-path">{file}</code>
										{/each}
									</div>
								</div>
							{/if}
							{#if autopsy.modules.length > 0}
								<div class="detail-section">
									<div class="detail-label">Modules</div>
									<div class="detail-value">{autopsy.modules.join(', ')}</div>
								</div>
							{/if}
							{#if autopsy.tags.length > 0}
								<div class="detail-section">
									<div class="detail-label">Tags</div>
									<div class="tag-list">
										{#each autopsy.tags as tag}
											<span class="tag">{tag}</span>
										{/each}
									</div>
								</div>
							{/if}
						</div>
					{/if}
				</button>
			{/each}
		{/if}
	</div>
</div>

<style>
	.autopsies-view {
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

	.count {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-muted);
	}

	.search-bar {
		display: flex;
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

	.list-scroll {
		flex: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.autopsy-row {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 0;
		cursor: pointer;
		text-align: left;
		width: 100%;
		font-family: var(--font-body);
		color: var(--text-primary);
		transition: border-color var(--transition-fast);
	}

	.autopsy-row:hover {
		border-color: var(--border-hover);
	}

	.autopsy-row.expanded {
		border-color: var(--border-active);
	}

	.autopsy-summary {
		display: flex;
		align-items: center;
		gap: var(--space-md);
		padding: 12px var(--space-md);
	}

	.autopsy-quest {
		font-size: 13px;
		font-weight: 500;
		color: var(--accent-red);
		min-width: 120px;
		flex-shrink: 0;
	}

	.autopsy-date {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-faint);
		min-width: 140px;
		flex-shrink: 0;
	}

	.autopsy-failure {
		flex: 1;
		font-size: 12px;
		color: var(--text-secondary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.autopsy-trigger {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-muted);
		background: var(--bg-raised);
		padding: 2px 8px;
		border-radius: var(--radius-sm);
		flex-shrink: 0;
	}

	.expand-icon {
		color: var(--text-faint);
		font-size: 14px;
		flex-shrink: 0;
		width: 16px;
		text-align: center;
	}

	.autopsy-detail {
		padding: 0 var(--space-md) var(--space-md);
		border-top: 1px solid var(--border);
		margin-top: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding-top: var(--space-md);
	}

	.detail-section {
		display: flex;
		gap: var(--space-md);
	}

	.detail-label {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--text-faint);
		min-width: 80px;
		flex-shrink: 0;
		padding-top: 2px;
	}

	.detail-value {
		font-size: 12px;
		color: var(--text-secondary);
		line-height: 1.5;
	}

	.detail-resolution {
		white-space: pre-wrap;
	}

	.file-path {
		display: block;
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--accent-blue);
		padding: 1px 0;
	}

	.tag-list {
		display: flex;
		gap: var(--space-xs);
		flex-wrap: wrap;
	}

	.tag {
		font-size: 11px;
		padding: 2px 8px;
		border-radius: var(--radius-sm);
		background: var(--accent-purple-dim);
		color: var(--accent-purple);
	}

	.empty-state {
		text-align: center;
		padding: var(--space-2xl);
		color: var(--text-muted);
		font-size: 13px;
	}

	.empty-state.error {
		color: var(--accent-red);
	}
</style>
