<script lang="ts">
	let { phase, compact = false }: { phase: string; compact?: boolean } = $props();

	const phases = ['Onboard', 'Research', 'Plan', 'Implement', 'Review', 'Complete'];

	function getStatus(p: string): 'done' | 'current' | 'pending' {
		const currentIdx = phases.indexOf(phase);
		const pIdx = phases.indexOf(p);
		if (pIdx < currentIdx) return 'done';
		if (pIdx === currentIdx) return 'current';
		return 'pending';
	}
</script>

<div class="timeline" class:compact>
	{#each phases as p}
		{@const status = getStatus(p)}
		<div class="phase-bar {status}" title={p}></div>
	{/each}
</div>

<style>
	.timeline {
		display: flex;
		gap: 3px;
	}

	.phase-bar {
		flex: 1;
		height: 4px;
		border-radius: 2px;
		background: var(--border);
		transition: background var(--transition-fast);
	}

	.phase-bar.done {
		background: var(--accent-green);
	}

	.phase-bar.current {
		background: var(--accent-gold);
	}

	.compact .phase-bar {
		height: 3px;
	}
</style>
