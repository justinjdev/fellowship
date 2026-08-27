<script lang="ts">
	import { approveGate, rejectGate } from '$lib/api';

	let { worktree }: { worktree: string } = $props();
	let loading = $state(false);
	let submitting = false;

	async function approve() {
		if (submitting) return;
		submitting = true;
		loading = true;
		try {
			await approveGate(worktree);
		} finally {
			submitting = false;
			loading = false;
		}
	}

	async function reject() {
		if (submitting) return;
		submitting = true;
		loading = true;
		try {
			await rejectGate(worktree);
		} finally {
			submitting = false;
			loading = false;
		}
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
