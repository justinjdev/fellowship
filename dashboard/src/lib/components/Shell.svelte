<script lang="ts">
	import Sidebar from './Sidebar.svelte';
	import CommandPalette from './CommandPalette.svelte';
	import type { Snippet } from 'svelte';
	import { onMount, onDestroy } from 'svelte';

	let { children }: { children: Snippet } = $props();
	let collapsed = $state(false);
	let paletteOpen = $state(false);

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			paletteOpen = !paletteOpen;
		}
	}

	onMount(() => {
		window.addEventListener('keydown', handleKeydown);
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown);
	});
</script>

<div class="shell">
	<Sidebar bind:collapsed />
	<main class="main-content">
		{@render children()}
	</main>
</div>

{#if paletteOpen}
	<CommandPalette onClose={() => (paletteOpen = false)} />
{/if}

<style>
	.shell {
		display: flex;
		height: 100vh;
		overflow: hidden;
	}

	.main-content {
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden;
	}
</style>
