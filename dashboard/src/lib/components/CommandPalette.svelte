<script lang="ts">
	import { goto } from '$app/navigation';
	import { spawnQuest, spawnScout, killQuest, restartQuest } from '$lib/api';
	import { onMount } from 'svelte';

	let { onClose }: { onClose: () => void } = $props();

	interface Action {
		label: string;
		category: string;
		action: () => void | Promise<void>;
		needsInput?: boolean;
		inputPlaceholder?: string;
	}

	const actions: Action[] = [
		{
			label: 'Spawn Quest',
			category: 'Quest Control',
			action: () => {
				inputMode = 'spawn-quest';
			},
			inputPlaceholder: 'Enter task description...'
		},
		{
			label: 'Spawn Scout',
			category: 'Quest Control',
			action: () => {
				inputMode = 'spawn-scout';
			},
			inputPlaceholder: 'Enter question...'
		},
		{
			label: 'Kill Quest',
			category: 'Quest Control',
			action: () => {
				inputMode = 'kill-quest';
			},
			inputPlaceholder: 'Enter quest ID...'
		},
		{
			label: 'Restart Quest',
			category: 'Quest Control',
			action: () => {
				inputMode = 'restart-quest';
			},
			inputPlaceholder: 'Enter quest ID...'
		},
		{
			label: 'Go to Command',
			category: 'Navigation',
			action: () => {
				goto('/command');
				onClose();
			}
		},
		{
			label: 'Go to Herald',
			category: 'Navigation',
			action: () => {
				goto('/herald');
				onClose();
			}
		},
		{
			label: 'Go to Autopsies',
			category: 'Navigation',
			action: () => {
				goto('/autopsies');
				onClose();
			}
		},
		{
			label: 'Go to Timeline',
			category: 'Navigation',
			action: () => {
				goto('/timeline');
				onClose();
			}
		},
		{
			label: 'Go to Config',
			category: 'Navigation',
			action: () => {
				goto('/config');
				onClose();
			}
		}
	];

	let query = $state('');
	let selectedIndex = $state(0);
	let inputMode = $state<'spawn-quest' | 'spawn-scout' | 'kill-quest' | 'restart-quest' | null>(
		null
	);
	let taskInput = $state('');
	let searchInput: HTMLInputElement | undefined = $state();
	let taskInputEl: HTMLInputElement | undefined = $state();

	let filtered = $derived(
		query
			? actions.filter((a) => a.label.toLowerCase().includes(query.toLowerCase()))
			: actions
	);

	let grouped = $derived.by(() => {
		const groups: { category: string; items: Action[] }[] = [];
		const seen = new Set<string>();
		for (const action of filtered) {
			if (!seen.has(action.category)) {
				seen.add(action.category);
				groups.push({ category: action.category, items: [] });
			}
			groups.find((g) => g.category === action.category)!.items.push(action);
		}
		return groups;
	});

	function flatItems(): Action[] {
		return filtered;
	}

	function executeSelected() {
		const items = flatItems();
		if (items[selectedIndex]) {
			items[selectedIndex].action();
		}
	}

	async function submitInput() {
		if (!taskInput.trim()) return;
		try {
			if (inputMode === 'spawn-quest') {
				await spawnQuest(taskInput.trim());
			} else if (inputMode === 'spawn-scout') {
				await spawnScout(taskInput.trim());
			} else if (inputMode === 'kill-quest') {
				await killQuest(taskInput.trim());
			} else if (inputMode === 'restart-quest') {
				await restartQuest(taskInput.trim());
			}
			inputMode = null;
			taskInput = '';
			onClose();
		} catch {
			// Keep palette open so user can retry
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (inputMode) {
			if (e.key === 'Escape') {
				e.preventDefault();
				inputMode = null;
				taskInput = '';
				searchInput?.focus();
			} else if (e.key === 'Enter') {
				e.preventDefault();
				submitInput();
			}
			return;
		}

		if (e.key === 'Escape') {
			e.preventDefault();
			onClose();
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filtered.length > 0) selectedIndex = (selectedIndex + 1) % filtered.length;
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filtered.length > 0) selectedIndex = (selectedIndex - 1 + filtered.length) % filtered.length;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			executeSelected();
		}
	}

	$effect(() => {
		// Reset selected index when filter changes
		query;
		selectedIndex = 0;
	});

	$effect(() => {
		if (inputMode && taskInputEl) {
			taskInputEl.focus();
		}
	});

	onMount(() => {
		searchInput?.focus();
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onkeydown={handleKeydown} onclick={onClose}>
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="panel" onclick={(e) => e.stopPropagation()}>
		{#if inputMode}
			<div class="input-mode-header">
				{inputMode === 'spawn-quest'
					? 'Spawn Quest'
					: inputMode === 'spawn-scout'
						? 'Spawn Scout'
						: inputMode === 'kill-quest'
							? 'Kill Quest'
							: 'Restart Quest'}
			</div>
			<input
				bind:this={taskInputEl}
				bind:value={taskInput}
				class="search-input"
				placeholder={actions.find(
					(a) =>
						a.label.toLowerCase().replace(/\s+/g, '-') ===
						inputMode?.replace('spawn-', 'spawn-').replace('kill-', 'kill-').replace('restart-', 'restart-')
				)?.inputPlaceholder ?? 'Enter value...'}
				type="text"
			/>
		{:else}
			<input
				bind:this={searchInput}
				bind:value={query}
				class="search-input"
				placeholder="Search commands..."
				type="text"
			/>
			<div class="action-list">
				{#each grouped as group}
					<div class="category-header">{group.category}</div>
					{#each group.items as item}
						{@const idx = filtered.indexOf(item)}
						<button
							class="action-item"
							class:selected={idx === selectedIndex}
							onmouseenter={() => (selectedIndex = idx)}
							onclick={() => item.action()}
						>
							{item.label}
						</button>
					{/each}
				{/each}
				{#if filtered.length === 0}
					<div class="no-results">No matching commands</div>
				{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: var(--bg-overlay, rgba(0, 0, 0, 0.6));
		backdrop-filter: blur(4px);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 20vh;
		z-index: 1000;
	}

	.panel {
		background: var(--bg-surface, #1e1e2e);
		border: 1px solid var(--border, #333);
		border-radius: 12px;
		max-width: 560px;
		width: 100%;
		overflow: hidden;
		box-shadow: 0 16px 48px rgba(0, 0, 0, 0.4);
	}

	.search-input {
		width: 100%;
		padding: 14px 16px;
		background: transparent;
		border: none;
		border-bottom: 1px solid var(--border, #333);
		color: var(--text, #e0e0e0);
		font-size: 16px;
		outline: none;
		box-sizing: border-box;
	}

	.search-input::placeholder {
		color: var(--text-faint, #666);
	}

	.input-mode-header {
		padding: 10px 16px 0;
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--text-faint, #666);
	}

	.action-list {
		max-height: 360px;
		overflow-y: auto;
		padding: 8px 0;
	}

	.category-header {
		padding: 8px 16px 4px;
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--text-faint, #666);
	}

	.action-item {
		display: block;
		width: 100%;
		padding: 8px 16px;
		background: transparent;
		border: none;
		color: var(--text, #e0e0e0);
		font-size: 14px;
		text-align: left;
		cursor: pointer;
		transition: background 0.1s;
	}

	.action-item:hover,
	.action-item.selected {
		background: var(--bg-raised, #2a2a3e);
	}

	.no-results {
		padding: 16px;
		text-align: center;
		color: var(--text-faint, #666);
		font-size: 14px;
	}
</style>
