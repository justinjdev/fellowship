<script lang="ts">
	let { text }: { text: string } = $props();
	let copied = $state(false);

	async function copy() {
		await navigator.clipboard.writeText(text);
		copied = true;
		setTimeout(() => copied = false, 2000);
	}
</script>

<button onclick={copy} class="copy-btn" aria-label={copied ? 'Copied!' : 'Copy to clipboard'} title="Copy">
	{#if copied}
		<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<polyline points="20 6 9 17 4 12"/>
		</svg>
	{:else}
		<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
			<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
		</svg>
	{/if}
</button>

<style>
	.copy-btn {
		background: none;
		border: 1px solid var(--color-border);
		color: var(--color-text-secondary);
		padding: var(--space-xs);
		border-radius: 4px;
		cursor: pointer;
		display: flex;
		align-items: center;
		transition: color var(--transition-fast), border-color var(--transition-fast);
	}
	.copy-btn:hover {
		color: var(--color-accent);
		border-color: var(--color-accent);
	}
</style>
