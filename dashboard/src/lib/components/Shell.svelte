<script lang="ts">
	import Sidebar from './Sidebar.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
	let collapsed = $state(false);
</script>

<div class="shell">
	<Sidebar bind:collapsed />
	<main class="main-content">
		{@render children()}
	</main>
</div>

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
