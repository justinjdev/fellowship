<script lang="ts">
	import { page } from '$app/state';
	import { base } from '$app/paths';
	import ThemeToggle from './ThemeToggle.svelte';

	let mobileOpen = $state(false);

	const links = [
		{ href: `${base}/`, label: 'Home' },
		{ href: `${base}/getting-started`, label: 'Getting Started' },
		{ href: `${base}/skills`, label: 'Skills' },
		{ href: `${base}/agents`, label: 'Agents' },
		{ href: `${base}/how-it-works`, label: 'How It Works' },
		{ href: `${base}/configuration`, label: 'Configuration' },
		{ href: `${base}/changelog`, label: 'Changelog' }
	];

	function isActive(href: string): boolean {
		const current = page.url.pathname;
		if (href === `${base}/`) return current === `${base}/` || current === base;
		return current.startsWith(href);
	}

	function closeMobile() {
		mobileOpen = false;
	}
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && mobileOpen) closeMobile(); }} />

<nav class="nav" aria-label="Main navigation">
	<div class="nav-inner container">
		<a href="{base}/" class="nav-logo" aria-label="Fellowship home">
			<span class="logo-text">Fellowship</span>
		</a>

		<button
			class="mobile-toggle"
			onclick={() => mobileOpen = !mobileOpen}
			aria-expanded={mobileOpen}
			aria-controls="nav-links"
			aria-label="{mobileOpen ? 'Close' : 'Open'} navigation menu"
		>
			<span class="hamburger" class:open={mobileOpen}></span>
		</button>

		<div class="nav-links" id="nav-links" class:open={mobileOpen}>
			{#each links as link (link.href)}
				<a
					href={link.href}
					class="nav-link"
					class:active={isActive(link.href)}
					aria-current={isActive(link.href) ? 'page' : undefined}
					onclick={closeMobile}
				>
					{link.label}
				</a>
			{/each}
			<ThemeToggle />
		</div>
	</div>
</nav>

<style>
	.nav {
		position: sticky;
		top: 0;
		z-index: 100;
		background: var(--color-bg);
		border-bottom: 1px solid var(--color-border);
		height: var(--nav-height);
	}
	.nav-inner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		height: 100%;
	}
	.nav-logo { text-decoration: none; }
	.logo-text {
		font-family: var(--font-heading);
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--color-accent);
		letter-spacing: 0.05em;
	}
	.nav-links {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}
	.nav-link {
		font-family: var(--font-body);
		font-size: 0.95rem;
		color: var(--color-text-secondary);
		padding: var(--space-xs) var(--space-sm);
		border-radius: 4px;
		transition: color var(--transition-fast);
	}
	.nav-link:hover, .nav-link.active { color: var(--color-accent); }
	.nav-link.active { font-weight: 600; }
	.mobile-toggle {
		display: none;
		background: none;
		border: none;
		cursor: pointer;
		padding: var(--space-sm);
	}
	.hamburger {
		display: block;
		width: 24px;
		height: 2px;
		background: var(--color-text);
		position: relative;
		transition: background var(--transition-fast);
	}
	.hamburger::before, .hamburger::after {
		content: '';
		position: absolute;
		width: 24px;
		height: 2px;
		background: var(--color-text);
		transition: transform var(--transition-fast);
	}
	.hamburger::before { top: -7px; }
	.hamburger::after { top: 7px; }
	.hamburger.open { background: transparent; }
	.hamburger.open::before { transform: rotate(45deg); top: 0; }
	.hamburger.open::after { transform: rotate(-45deg); top: 0; }
	@media (max-width: 900px) {
		.mobile-toggle { display: block; }
		.nav-links {
			display: none;
			position: absolute;
			top: var(--nav-height);
			left: 0;
			right: 0;
			background: var(--color-bg);
			border-bottom: 1px solid var(--color-border);
			flex-direction: column;
			padding: var(--space-md);
			gap: var(--space-xs);
		}
		.nav-links.open { display: flex; }
		.nav-link {
			width: 100%;
			padding: var(--space-sm) var(--space-md);
		}
	}
</style>
