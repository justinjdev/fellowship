<script lang="ts">
	import type { Errand } from '$lib/types';

	let { errands }: { errands: Errand[] } = $props();
</script>

<div class="errand-list">
	{#each errands as errand (errand.id)}
		<div class="errand-item">
			<div class="errand-check {errand.status}">
				{#if errand.status === 'done'}✓{:else if errand.status === 'active'}●{/if}
			</div>
			<span class="errand-text {errand.status}">{errand.description}</span>
		</div>
	{/each}
</div>

<style>
	.errand-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.errand-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 12px;
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		font-size: 13px;
	}

	.errand-check {
		width: 18px;
		height: 18px;
		border-radius: var(--radius-sm);
		border: 1.5px solid var(--border-active);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 10px;
		flex-shrink: 0;
	}

	.errand-check.done {
		background: var(--accent-green-dim);
		border-color: var(--accent-green);
		color: var(--accent-green-text);
	}

	.errand-check.active {
		border-color: var(--accent-gold-dim);
		background: rgba(212, 168, 67, 0.1);
		color: var(--accent-gold);
	}

	.errand-text {
		color: var(--text-muted);
	}

	.errand-text.done {
		color: var(--text-faint);
		text-decoration: line-through;
	}

	.errand-text.active {
		color: var(--text-primary);
	}
</style>
