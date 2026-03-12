<script lang="ts">
	import type { QuestTome } from '$lib/types';

	let { quests }: { quests: QuestTome[] } = $props();

	const phaseColors: Record<string, string> = {
		Onboard: 'var(--accent-blue-dim)',
		Research: 'var(--accent-purple-dim)',
		Plan: 'var(--accent-gold-dim)',
		Implement: 'var(--accent-green-dim)',
		Review: 'var(--accent-gold)',
		Complete: 'var(--accent-green)',
	};

	const phaseTextColors: Record<string, string> = {
		Onboard: 'var(--accent-blue)',
		Research: 'var(--accent-purple)',
		Plan: 'var(--accent-gold)',
		Implement: 'var(--accent-green-text)',
		Review: 'var(--accent-gold)',
		Complete: 'var(--accent-green-text)',
	};

	interface BarSegment {
		phase: string;
		startPct: number;
		widthPct: number;
		duration: string;
	}

	interface QuestRow {
		name: string;
		segments: BarSegment[];
	}

	function formatDuration(ms: number): string {
		const mins = Math.floor(ms / 60000);
		if (mins < 60) return `${mins}m`;
		const hours = Math.floor(mins / 60);
		if (hours < 24) return `${hours}h ${mins % 60}m`;
		return `${Math.floor(hours / 24)}d ${hours % 24}h`;
	}

	let rows = $derived.by(() => {
		if (quests.length === 0) return [];

		let globalMin = Infinity;
		let globalMax = -Infinity;

		for (const q of quests) {
			for (const p of q.phases_completed) {
				const t = new Date(p.timestamp).getTime();
				if (!isNaN(t)) {
					if (t < globalMin) globalMin = t;
					if (t > globalMax) globalMax = t;
				}
			}
			const created = new Date(q.created_at).getTime();
			if (!isNaN(created) && created < globalMin) globalMin = created;
		}

		if (globalMax <= globalMin) globalMax = globalMin + 1;
		const range = globalMax - globalMin;

		const result: QuestRow[] = [];

		for (const q of quests) {
			const phases = [...q.phases_completed].sort(
				(a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
			);
			if (phases.length === 0) continue;

			const segments: BarSegment[] = [];
			const questStart = new Date(q.created_at).getTime();

			for (let i = 0; i < phases.length; i++) {
				const start = i === 0 ? questStart : new Date(phases[i - 1].timestamp).getTime();
				const end = new Date(phases[i].timestamp).getTime();
				const dur = end - start;

				segments.push({
					phase: phases[i].phase,
					startPct: ((start - globalMin) / range) * 100,
					widthPct: Math.max((dur / range) * 100, 0.5),
					duration: formatDuration(dur),
				});
			}

			result.push({ name: q.quest_name, segments });
		}

		return result;
	});

	let hoveredSegment = $state<{ quest: string; phase: string; duration: string; x: number; y: number } | null>(null);

	function onSegmentEnter(e: MouseEvent, questName: string, seg: BarSegment) {
		hoveredSegment = {
			quest: questName,
			phase: seg.phase,
			duration: seg.duration,
			x: e.clientX,
			y: e.clientY,
		};
	}

	function onSegmentLeave() {
		hoveredSegment = null;
	}
</script>

<div class="gantt-chart">
	{#each rows as row}
		<div class="gantt-row">
			<div class="gantt-label">{row.name}</div>
			<div class="gantt-track">
				{#each row.segments as seg}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div
						class="gantt-bar"
						style:left="{seg.startPct}%"
						style:width="{seg.widthPct}%"
						style:background={phaseColors[seg.phase] ?? 'var(--bg-raised)'}
						style:color={phaseTextColors[seg.phase] ?? 'var(--text-muted)'}
						onmouseenter={(e) => onSegmentEnter(e, row.name, seg)}
						onmouseleave={onSegmentLeave}
					></div>
				{/each}
			</div>
		</div>
	{/each}
</div>

{#if hoveredSegment}
	<div class="gantt-tooltip" style:left="{hoveredSegment.x + 12}px" style:top="{hoveredSegment.y - 8}px">
		<span class="tooltip-phase">{hoveredSegment.phase}</span>
		<span class="tooltip-dur">{hoveredSegment.duration}</span>
	</div>
{/if}

<style>
	.gantt-chart {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.gantt-row {
		display: flex;
		align-items: center;
		gap: var(--space-md);
	}

	.gantt-label {
		min-width: 140px;
		max-width: 140px;
		font-size: 12px;
		font-weight: 500;
		color: var(--text-secondary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex-shrink: 0;
	}

	.gantt-track {
		flex: 1;
		position: relative;
		height: 28px;
		background: var(--bg-raised);
		border-radius: var(--radius-sm);
		overflow: hidden;
	}

	.gantt-bar {
		position: absolute;
		top: 2px;
		bottom: 2px;
		border-radius: 3px;
		min-width: 4px;
		cursor: default;
		transition: opacity var(--transition-fast);
	}

	.gantt-bar:hover {
		opacity: 0.85;
	}

	.gantt-tooltip {
		position: fixed;
		background: var(--bg-surface);
		border: 1px solid var(--border-active);
		border-radius: var(--radius-md);
		padding: 6px 10px;
		display: flex;
		gap: var(--space-sm);
		align-items: baseline;
		pointer-events: none;
		z-index: 1000;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
	}

	.tooltip-phase {
		font-size: 12px;
		font-weight: 500;
		color: var(--text-primary);
	}

	.tooltip-dur {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--text-muted);
	}
</style>
