<script lang="ts">
	import type { DashboardError } from '$lib/types';
	import { clearErrors } from '$lib/stores/errors';

	let { errors }: { errors: DashboardError[] } = $props();

	let expanded = $state(false);

	function formatTime(ts: string): string {
		const date = new Date(ts);
		if (isNaN(date.getTime())) return '?';
		const diff = Date.now() - date.getTime();
		const mins = Math.floor(diff / 60000);
		if (mins < 1) return 'now';
		if (mins < 60) return `${mins}m ago`;
		const hours = Math.floor(mins / 60);
		if (hours < 24) return `${hours}h ago`;
		return `${Math.floor(hours / 24)}d ago`;
	}

	function sourceLabel(source: string): string {
		switch (source) {
			case 'api': return 'API';
			case 'websocket': return 'WS';
			case 'polling': return 'Poll';
			default: return source;
		}
	}
</script>

{#if errors.length > 0}
	<div class="error-banner">
		<button class="error-summary" onclick={() => (expanded = !expanded)}>
			<span class="error-icon">!</span>
			<span class="error-count">{errors.length} server error{errors.length !== 1 ? 's' : ''}</span>
			<span class="error-toggle">{expanded ? '▾' : '▸'}</span>
		</button>

		{#if expanded}
			<div class="error-list">
				{#each errors as error}
					<div class="error-item">
						<span class="error-source">{sourceLabel(error.source)}</span>
						<span class="error-handler">{error.handler}</span>
						<span class="error-message">{error.message}</span>
						<span class="error-time">{formatTime(error.timestamp)}</span>
					</div>
				{/each}
			</div>
			<button class="error-clear" onclick={clearErrors}>Clear all</button>
		{/if}
	</div>
{/if}

<style>
	.error-banner {
		background: var(--accent-red-dim);
		border: 1px solid var(--accent-red);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.error-summary {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 10px 14px;
		background: none;
		border: none;
		color: var(--accent-red);
		font-family: var(--font-body);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		text-align: left;
	}

	.error-summary:hover {
		background: rgba(248, 81, 73, 0.08);
	}

	.error-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: var(--accent-red);
		color: var(--bg-base);
		font-size: 11px;
		font-weight: 700;
		flex-shrink: 0;
	}

	.error-count {
		flex: 1;
	}

	.error-toggle {
		font-size: 11px;
		color: var(--text-muted);
	}

	.error-list {
		display: flex;
		flex-direction: column;
		border-top: 1px solid var(--accent-red);
		max-height: 200px;
		overflow-y: auto;
	}

	.error-item {
		display: flex;
		align-items: baseline;
		gap: 8px;
		padding: 8px 14px;
		font-size: 12px;
		border-bottom: 1px solid rgba(248, 81, 73, 0.15);
	}

	.error-item:last-child {
		border-bottom: none;
	}

	.error-source {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--accent-red);
		background: rgba(248, 81, 73, 0.12);
		padding: 1px 5px;
		border-radius: var(--radius-sm);
		flex-shrink: 0;
	}

	.error-handler {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-secondary);
		flex-shrink: 0;
	}

	.error-message {
		flex: 1;
		color: var(--text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.error-time {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--text-faint);
		flex-shrink: 0;
	}

	.error-clear {
		display: block;
		width: 100%;
		padding: 8px;
		background: none;
		border: none;
		border-top: 1px solid rgba(248, 81, 73, 0.15);
		color: var(--text-muted);
		font-family: var(--font-body);
		font-size: 11px;
		cursor: pointer;
		transition: color var(--transition-fast);
	}

	.error-clear:hover {
		color: var(--accent-red);
	}
</style>
