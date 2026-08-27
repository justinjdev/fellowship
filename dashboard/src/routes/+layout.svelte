<script>
	import '$lib/styles/global.css';
	import Shell from '$lib/components/Shell.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<title>Fellowship Dashboard</title>
</svelte:head>

<Shell>
	{@render children()}
</Shell>
