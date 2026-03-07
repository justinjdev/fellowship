<script lang="ts">
	import { base } from '$app/paths';
	import CopyButton from '$lib/components/CopyButton.svelte';

	const features = [
		{
			title: 'Phased Lifecycle',
			description: 'Every task flows through Research, Plan, Implement, Review, and Complete. Hard gates between phases prevent drift and keep reasoning sharp.'
		},
		{
			title: 'Parallel Agents',
			description: 'Multiple independent tasks run simultaneously, each in an isolated git worktree. A coordinator routes approvals and tracks progress.'
		},
		{
			title: 'Gate Enforcement',
			description: 'Structurally enforced via plugin hooks \u2014 not just prompts. Tool access is blocked between phases. Compliance jumped from ~33% to ~95%+.'
		},
		{
			title: 'Human in the Loop',
			description: 'All gates surface to you for approval by default. Auto-approve specific gates via config, but you\u2019re always in control. Gandalf never merges.'
		}
	];

	const phases = [
		{ number: 0, name: 'Onboard', description: 'Worktree + context loading' },
		{ number: 1, name: 'Research', description: 'Explore codebase + gather conventions' },
		{ number: 2, name: 'Plan', description: 'Architecture with file:line references' },
		{ number: 3, name: 'Implement', description: 'TDD red-green-refactor' },
		{ number: 4, name: 'Review', description: 'Convention check + verification' },
		{ number: 5, name: 'Complete', description: 'PR creation + cleanup' }
	];
</script>

<svelte:head>
	<title>Fellowship &mdash; Multi-Agent Orchestration for Claude Code</title>
	<meta name="description" content="A Claude Code plugin that orchestrates multi-task workflows through structured research-plan-implement lifecycles." />
</svelte:head>

<!-- Hero -->
<section class="hero">
	<div class="container">
		<h1 class="hero-title">A fellowship of agents, each on their own quest</h1>
		<p class="hero-subtitle">
			A Claude Code plugin that orchestrates multi-task workflows through structured research-plan-implement lifecycles.
		</p>
		<div class="hero-image-wrapper">
			<img
				src="https://github.com/user-attachments/assets/a4fb319e-20ca-4bba-8595-134cbd06f4b6"
				alt="Fellowship plugin showing parallel quest orchestration in Claude Code"
				class="hero-image"
				loading="eager"
			/>
		</div>
		<a href="{base}/getting-started" class="btn-primary">Get Started</a>
	</div>
</section>

<div class="container">
	<div class="divider"><div class="divider-ring"></div></div>
</div>

<!-- Features -->
<section class="features">
	<div class="container">
		<h2 class="section-title">Why Fellowship</h2>
		<div class="features-grid">
			{#each features as feature, i (feature.title)}
				<div class="feature-card animate-in" style="animation-delay: {i * 100}ms">
					<h3>{feature.title}</h3>
					<p>{feature.description}</p>
				</div>
			{/each}
		</div>
	</div>
</section>

<div class="container">
	<div class="divider"><div class="divider-ring"></div></div>
</div>

<!-- Quest Lifecycle -->
<section class="lifecycle">
	<div class="container">
		<h2 class="section-title">The Quest Lifecycle</h2>
		<div class="timeline">
			{#each phases as phase, i (phase.number)}
				<div class="timeline-step animate-in" style="animation-delay: {i * 100}ms">
					<div class="timeline-marker">
						<span class="marker-number">{phase.number}</span>
					</div>
					<div class="timeline-content">
						<h3 class="phase-name">{phase.name}</h3>
						<p class="phase-desc">{phase.description}</p>
					</div>
				</div>
			{/each}
		</div>
	</div>
</section>

<div class="container">
	<div class="divider"><div class="divider-ring"></div></div>
</div>

<!-- Quick Install -->
<section class="install">
	<div class="container">
		<h2 class="section-title">Quick Install</h2>
		<div class="install-steps">
			<div class="code-block">
				<pre><code>/plugin marketplace add justinjdev/claude-plugins</code></pre>
				<CopyButton text="/plugin marketplace add justinjdev/claude-plugins" />
			</div>
			<div class="code-block">
				<pre><code>/plugin install fellowship@justinjdev</code></pre>
				<CopyButton text="/plugin install fellowship@justinjdev" />
			</div>
		</div>
		<a href="{base}/getting-started" class="setup-link">View full setup guide &rarr;</a>
	</div>
</section>

<style>
	/* Hero */
	.hero {
		padding: var(--space-2xl) 0 var(--space-xl);
		text-align: center;
	}

	.hero-title {
		font-size: 3rem;
		max-width: 48rem;
		margin: 0 auto var(--space-md);
		background: linear-gradient(135deg, var(--color-accent), var(--color-accent-hover));
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}

	.hero-subtitle {
		font-size: 1.25rem;
		color: var(--color-text-secondary);
		max-width: 40rem;
		margin: 0 auto var(--space-xl);
		line-height: 1.6;
	}

	.hero-image-wrapper {
		margin: 0 auto var(--space-xl);
		max-width: 56rem;
		border-radius: 12px;
		overflow: hidden;
		border: 1px solid var(--color-border);
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
	}

	.hero-image {
		width: 100%;
		height: auto;
		display: block;
	}

	.btn-primary {
		display: inline-block;
		font-family: var(--font-heading);
		font-size: 1.1rem;
		letter-spacing: 0.04em;
		color: var(--color-bg);
		background: var(--color-accent);
		padding: var(--space-sm) var(--space-lg);
		border-radius: 6px;
		text-decoration: none;
		transition: background var(--transition-fast), transform var(--transition-fast);
	}

	.btn-primary:hover {
		background: var(--color-accent-hover);
		color: var(--color-bg);
		transform: translateY(-1px);
	}

	/* Section titles */
	.section-title {
		text-align: center;
		margin-bottom: var(--space-xl);
	}

	/* Features */
	.features {
		padding: var(--space-lg) 0;
	}

	.features-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-lg);
	}

	.feature-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		padding: var(--space-lg);
		border-radius: 8px;
		transition: border-color var(--transition-normal), transform var(--transition-normal), box-shadow var(--transition-normal);
	}

	.feature-card:hover {
		border-color: var(--color-accent);
		transform: translateY(-2px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}

	.feature-card h3 {
		margin-bottom: var(--space-sm);
	}

	.feature-card p {
		color: var(--color-text-secondary);
		line-height: 1.6;
	}

	/* Lifecycle timeline */
	.lifecycle {
		padding: var(--space-lg) 0;
	}

	.timeline {
		max-width: 36rem;
		margin: 0 auto;
		position: relative;
	}

	.timeline::before {
		content: '';
		position: absolute;
		left: 1.25rem;
		top: 1.25rem;
		bottom: 1.25rem;
		width: 2px;
		background: var(--color-border);
	}

	.timeline-step {
		display: flex;
		align-items: flex-start;
		gap: var(--space-md);
		padding-bottom: var(--space-lg);
		position: relative;
	}

	.timeline-step:last-child {
		padding-bottom: 0;
	}

	.timeline-marker {
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 50%;
		border: 2px solid var(--color-accent);
		background: var(--color-bg);
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		position: relative;
		z-index: 1;
	}

	.marker-number {
		font-family: var(--font-heading);
		font-size: 0.85rem;
		color: var(--color-accent);
		font-weight: 700;
	}

	.timeline-content {
		padding-top: 0.35rem;
	}

	.phase-name {
		font-size: 1.15rem;
		margin-bottom: 0.15rem;
	}

	.phase-desc {
		color: var(--color-text-secondary);
		font-size: 1rem;
	}

	/* Install */
	.install {
		padding: var(--space-lg) 0;
	}

	.install-steps {
		max-width: 40rem;
		margin: 0 auto var(--space-lg);
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}

	.code-block {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.code-block pre {
		flex: 1;
		margin: 0;
	}

	.setup-link {
		display: block;
		text-align: center;
		font-size: 1.05rem;
		color: var(--color-accent);
	}

	.setup-link:hover {
		color: var(--color-accent-hover);
	}

	/* Responsive */
	@media (max-width: 768px) {
		.hero {
			padding: var(--space-xl) 0 var(--space-lg);
		}

		.hero-title {
			font-size: 2rem;
		}

		.hero-subtitle {
			font-size: 1.1rem;
		}

		.features-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
