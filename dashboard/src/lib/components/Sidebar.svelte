<script lang="ts">
	import { page } from '$app/state';

	let { collapsed = $bindable(false) }: { collapsed: boolean } = $props();

	const navItems = [
		{ path: '/command', label: 'Command', icon: '⚔' },
		{ path: '/herald', label: 'Herald', icon: '📜' },
		{ path: '/autopsies', label: 'Autopsies', icon: '◆' },
		{ path: '/timeline', label: 'Timeline', icon: '▰' },
		{ path: '/config', label: 'Config', icon: '⚙' },
	];

	function isActive(path: string): boolean {
		return page.url.pathname === path || page.url.pathname.startsWith(path + '/');
	}
</script>

<aside class="sidebar" class:collapsed>
	<div class="sidebar-header">
		<div class="logo-mark">F</div>
		{#if !collapsed}
			<span class="logo-text">Fellowship</span>
		{/if}
	</div>

	<nav class="sidebar-nav">
		{#each navItems as item}
			<a
				href={item.path}
				class="nav-item"
				class:active={isActive(item.path)}
				title={collapsed ? item.label : undefined}
			>
				<span class="nav-icon">{item.icon}</span>
				{#if !collapsed}
					<span class="nav-label">{item.label}</span>
				{/if}
			</a>
		{/each}
	</nav>

	<button class="sidebar-toggle" onclick={() => collapsed = !collapsed}>
		{collapsed ? '›' : '‹'}
	</button>
</aside>

<style>
	.sidebar {
		width: var(--sidebar-width-expanded);
		background: #0a0c10;
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		transition: width var(--transition-normal);
		overflow: hidden;
		flex-shrink: 0;
	}

	.sidebar.collapsed {
		width: var(--sidebar-width-collapsed);
	}

	.sidebar-header {
		padding: 16px;
		border-bottom: 1px solid var(--border);
		display: flex;
		align-items: center;
		gap: 10px;
		min-height: 56px;
	}

	.logo-mark {
		width: 28px;
		height: 28px;
		background: linear-gradient(135deg, var(--accent-gold), #a67c2e);
		border-radius: 6px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-family: var(--font-brand);
		font-size: 14px;
		font-weight: 700;
		color: #0a0c10;
		flex-shrink: 0;
	}

	.logo-text {
		font-family: var(--font-brand);
		font-size: 15px;
		font-weight: 600;
		color: var(--accent-gold);
		letter-spacing: 0.05em;
		white-space: nowrap;
	}

	.sidebar-nav {
		flex: 1;
		padding: 12px 8px;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 9px 10px;
		border-radius: var(--radius-md);
		font-size: 13px;
		color: var(--text-muted);
		text-decoration: none;
		transition: all var(--transition-fast);
		white-space: nowrap;
	}

	.nav-item:hover {
		color: var(--text-secondary);
		background: var(--bg-raised);
		text-decoration: none;
	}

	.nav-item.active {
		color: var(--text-secondary);
		background: var(--bg-raised);
	}

	.nav-icon {
		width: 20px;
		text-align: center;
		font-size: 14px;
		flex-shrink: 0;
	}

	.sidebar-toggle {
		padding: 12px;
		border-top: 1px solid var(--border);
		color: var(--text-faint);
		font-size: 16px;
		text-align: center;
		transition: color var(--transition-fast);
	}

	.sidebar-toggle:hover {
		color: var(--text-muted);
	}
</style>
