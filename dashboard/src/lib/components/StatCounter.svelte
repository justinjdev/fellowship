<script lang="ts">
	let { value, label, color = 'var(--text-primary)' }: {
		value: number;
		label: string;
		color?: string;
	} = $props();
</script>

<div class="stat">
	<div class="stat-value" style:color>{value}</div>
	<div class="stat-label">{label}</div>
</div>

<style>
	.stat {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 14px 16px;
		flex: 1;
	}

	.stat-value {
		font-family: var(--font-heading);
		font-size: 28px;
		font-weight: 600;
		line-height: 1;
	}

	.stat-label {
		font-size: 11px;
		color: var(--text-muted);
		margin-top: 4px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
</style>
