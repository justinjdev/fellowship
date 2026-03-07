<script>
	import '$lib/styles/global.css';
	let { children } = $props();
</script>

<a href="#main-content" class="skip-link">Skip to content</a>
<main id="main-content">
	{@render children()}
</main>
