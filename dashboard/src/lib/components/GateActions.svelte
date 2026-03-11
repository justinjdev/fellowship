<script lang="ts">
	import { approveGate, rejectGate } from '$lib/api';

	let { worktree }: { worktree: string } = $props();
	let loading = $state(false);

	async function approve() {
		loading = true;
		await approveGate(worktree);
		loading = false;
	}

	async function reject() {
		loading = true;
		await rejectGate(worktree);
		loading = false;
	}
</script>

<div class="gate-actions">
	<button class="gate-btn approve" onclick={approve} disabled={loading}>
		{loading ? '...' : 'Approve'}
	</button>
	<button class="gate-btn reject" onclick={reject} disabled={loading}>
		{loading ? '...' : 'Reject'}
	</button>
</div>

<style>
	.gate-actions {
		display: flex;
		gap: 6px;
	}

	.gate-btn {
		padding: 4px 12px;
		border-radius: var(--radius-sm);
		font-size: 11px;
		font-weight: 500;
		transition: opacity var(--transition-fast);
	}

	.gate-btn:disabled {
		opacity: 0.5;
	}

	.approve {
		background: var(--accent-green-dim);
		color: var(--accent-green-text);
	}

	.reject {
		background: var(--accent-red-dim);
		color: var(--accent-red);
	}
</style>
