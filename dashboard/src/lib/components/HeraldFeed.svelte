<script lang="ts">
	import type { Tiding } from '$lib/types';

	let { tidings, limit = 20 }: { tidings: Tiding[]; limit?: number } = $props();

	const typeIcons: Record<string, { icon: string; color: string }> = {
		gate_submitted: { icon: '◆', color: 'var(--accent-gold)' },
		gate_approved: { icon: '✓', color: 'var(--accent-green-text)' },
		gate_rejected: { icon: '✗', color: 'var(--accent-red)' },
		phase_transition: { icon: '→', color: 'var(--accent-green-text)' },
		lembas_completed: { icon: '◎', color: 'var(--accent-blue)' },
		metadata_updated: { icon: '◎', color: 'var(--text-muted)' },
		quest_held: { icon: '⏸', color: 'var(--accent-gold)' },
		quest_unheld: { icon: '▶', color: 'var(--accent-green-text)' },
	};

	function formatTime(ts: string): string {
		const diff = Date.now() - new Date(ts).getTime();
		const mins = Math.floor(diff / 60000);
		if (mins < 1) return 'now';
		if (mins < 60) return `${mins}m`;
		const hours = Math.floor(mins / 60);
		if (hours < 24) return `${hours}h`;
		return `${Math.floor(hours / 24)}d`;
	}

	function formatDetail(t: Tiding): string {
		switch (t.type) {
			case 'gate_submitted': return `${t.quest} submitted ${t.phase} gate`;
			case 'gate_approved': return `${t.quest} ${t.phase} gate approved`;
			case 'gate_rejected': return `${t.quest} ${t.phase} gate rejected`;
			case 'phase_transition': return `${t.quest} entered ${t.phase}`;
			default: return t.detail || `${t.quest} ${t.type}`;
		}
	}
</script>

<div class="herald-feed">
	{#each tidings.slice(0, limit) as tiding}
		{@const meta = typeIcons[tiding.type] ?? { icon: '·', color: 'var(--text-muted)' }}
		<div class="herald-item">
			<span class="herald-icon" style:color={meta.color}>{meta.icon}</span>
			<span class="herald-text">{formatDetail(tiding)}</span>
			<span class="herald-time">{formatTime(tiding.timestamp)}</span>
		</div>
	{/each}
</div>

<style>
	.herald-feed {
		display: flex;
		flex-direction: column;
	}

	.herald-item {
		display: flex;
		align-items: baseline;
		gap: 8px;
		padding: 8px 0;
		border-bottom: 1px solid var(--border);
		font-size: 12px;
	}

	.herald-item:last-child {
		border-bottom: none;
	}

	.herald-icon {
		flex-shrink: 0;
		width: 14px;
		text-align: center;
	}

	.herald-text {
		flex: 1;
		color: var(--text-muted);
	}

	.herald-time {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--text-faint);
		flex-shrink: 0;
	}
</style>
