<script lang="ts">
	import Sidebar from './Sidebar.svelte';
	import CommandPalette from './CommandPalette.svelte';
	import ErrorBanner from './ErrorBanner.svelte';
	import type { Snippet } from 'svelte';
	import { onMount, onDestroy } from 'svelte';
	import { errors, startErrorPolling, stopErrorPolling } from '$lib/stores/errors';

	let { children }: { children: Snippet } = $props();
	let collapsed = $state(false);
	let paletteOpen = $state(false);

	function handleKeydown(e: KeyboardEvent) {
		if (e.key.toLowerCase() === 'k' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			paletteOpen = !paletteOpen;
		}
	}

	onMount(() => {
		window.addEventListener('keydown', handleKeydown);
		startErrorPolling();
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown);
		stopErrorPolling();
	});
</script>

<div class="shell">
	<Sidebar bind:collapsed />
	<main class="main-content">
		{#if $errors.length > 0}
			<div class="error-banner-wrapper">
				<ErrorBanner errors={$errors} />
			</div>
		{/if}
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

	.error-banner-wrapper {
		padding: var(--space-md) var(--space-lg) 0;
	}
</style>
