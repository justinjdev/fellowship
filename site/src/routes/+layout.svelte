<script>
	import '$lib/styles/global.css';
	import Nav from '$lib/components/Nav.svelte';
	import Footer from '$lib/components/Footer.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<meta name="description" content="Fellowship - A Claude Code plugin for multi-task workflow orchestration" />
</svelte:head>

<a href="#main-content" class="skip-link">Skip to content</a>
<Nav />
<main id="main-content">
	{@render children()}
</main>
<Footer />
