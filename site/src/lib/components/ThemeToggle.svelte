<script lang="ts">
	import { browser } from '$app/environment';

	function getInitialTheme(): 'dark' | 'light' {
		if (!browser) return 'dark';
		const stored = localStorage.getItem('theme');
		if (stored === 'light' || stored === 'dark') return stored;
		if (window.matchMedia('(prefers-color-scheme: light)').matches) return 'light';
		return 'dark';
	}

	let theme = $state(getInitialTheme());

	if (browser) {
		document.documentElement.setAttribute('data-theme', getInitialTheme());
	}

	function toggle() {
		theme = theme === 'dark' ? 'light' : 'dark';
		document.documentElement.setAttribute('data-theme', theme);
		localStorage.setItem('theme', theme);
	}
</script>

<button
	onclick={toggle}
	class="theme-toggle"
	aria-label="Toggle {theme === 'dark' ? 'light' : 'dark'} theme"
	title="Toggle theme"
>
	{#if theme === 'dark'}
		<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<circle cx="12" cy="12" r="5"/>
			<path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/>
		</svg>
	{:else}
		<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
		</svg>
	{/if}
</button>

<style>
	.theme-toggle {
		background: none;
		border: 1px solid var(--color-border);
		color: var(--color-text);
		padding: var(--space-xs) var(--space-sm);
		border-radius: 6px;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: border-color var(--transition-fast), color var(--transition-fast);
	}
	.theme-toggle:hover {
		border-color: var(--color-accent);
		color: var(--color-accent);
	}
</style>
