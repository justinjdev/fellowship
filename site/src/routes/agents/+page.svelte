<script lang="ts">
	import { base } from '$app/paths';
</script>

<svelte:head>
	<title>Agents &mdash; Fellowship</title>
	<meta name="description" content="Specialized subprocesses that Fellowship spawns to handle monitoring, execution, and research roles." />
</svelte:head>

<section class="page-header">
	<div class="container">
		<h1>Agents</h1>
		<p class="intro">
			Agents are specialized subprocesses that Fellowship spawns to handle specific roles.
			Unlike skills (which are prompts), agents run as independent Claude Code instances with
			their own context windows.
		</p>
	</div>
</section>

<div class="container">
	<div class="divider"><div class="divider-ring"></div></div>
</div>

<section class="agents">
	<div class="container">
		<h2 class="sr-only">The Fellowship's Agents</h2>
		<div class="agents-grid">
			<!-- Palantir -->
			<div class="agent-card animate-in" style="animation-delay: 0ms">
				<div class="agent-header">
					<div class="agent-icon palantir-icon">
						<svg viewBox="0 0 48 48" width="48" height="48" fill="none" xmlns="http://www.w3.org/2000/svg">
							<circle cx="24" cy="24" r="20" stroke="currentColor" stroke-width="1.5" opacity="0.3" />
							<circle cx="24" cy="24" r="13" stroke="currentColor" stroke-width="1.5" opacity="0.5" />
							<circle cx="24" cy="24" r="6" fill="currentColor" opacity="0.8" />
							<circle cx="24" cy="24" r="2.5" fill="var(--color-bg)" />
							<line x1="24" y1="4" x2="24" y2="11" stroke="currentColor" stroke-width="1" opacity="0.2" />
							<line x1="24" y1="37" x2="24" y2="44" stroke="currentColor" stroke-width="1" opacity="0.2" />
							<line x1="4" y1="24" x2="11" y2="24" stroke="currentColor" stroke-width="1" opacity="0.2" />
							<line x1="37" y1="24" x2="44" y2="24" stroke="currentColor" stroke-width="1" opacity="0.2" />
						</svg>
					</div>
					<div>
						<h3 class="agent-name">Palantir</h3>
						<span class="agent-role">Background Monitor</span>
					</div>
				</div>
				<p class="agent-desc">
					Watches quest progress via task metadata during fellowship execution.
					Detects stuck quests (no phase change in extended periods), scope drift
					(implementation diverging from plan), and file conflicts between parallel quests.
					Reports issues to Gandalf for intervention.
				</p>
				<div class="agent-note config-note">
					Enabled by default. Spawns at 2+ active quests. Configure via
					<code>palantir.enabled</code> and <code>palantir.minQuests</code> in settings.
				</div>
			</div>

			<!-- Quest Runner -->
			<div class="agent-card animate-in" style="animation-delay: 100ms">
				<div class="agent-header">
					<div class="agent-icon runner-icon">
						<svg viewBox="0 0 48 48" width="48" height="48" fill="none" xmlns="http://www.w3.org/2000/svg">
							<path d="M24 6 L28 20 L24 18 L20 20 Z" fill="currentColor" opacity="0.8" />
							<path d="M20 20 L16 38 L24 34 L32 38 L28 20 L24 18 Z" stroke="currentColor" stroke-width="1.5" fill="currentColor" opacity="0.3" />
							<line x1="24" y1="6" x2="24" y2="34" stroke="currentColor" stroke-width="1.5" opacity="0.5" />
							<circle cx="24" cy="42" r="3" stroke="currentColor" stroke-width="1.5" opacity="0.4" />
						</svg>
					</div>
					<div>
						<h3 class="agent-name">Quest Runner</h3>
						<span class="agent-role">Quest Executor</span>
					</div>
				</div>
				<p class="agent-desc">
					The workhorse agent that executes the quest lifecycle. Uses the fellowship CLI
					binary for gate management, status checks, and phase transitions. Each quest
					runner operates in its own isolated git worktree, ensuring parallel quests never
					interfere with each other.
				</p>
				<div class="agent-note">
					Spawned by Gandalf during fellowships. Uses <code>/quest</code> skill internally.
				</div>
			</div>

			<!-- Balrog -->
			<div class="agent-card animate-in" style="animation-delay: 200ms">
				<div class="agent-header">
					<div class="agent-icon balrog-icon">
						<svg viewBox="0 0 48 48" width="48" height="48" fill="none" xmlns="http://www.w3.org/2000/svg">
							<circle cx="24" cy="24" r="18" stroke="currentColor" stroke-width="1.5" opacity="0.25" />
							<line x1="24" y1="24" x2="6" y2="8"  stroke="currentColor" stroke-width="1.5" opacity="0.7" />
							<line x1="24" y1="24" x2="42" y2="10" stroke="currentColor" stroke-width="1.5" opacity="0.7" />
							<line x1="24" y1="24" x2="40" y2="40" stroke="currentColor" stroke-width="1.5" opacity="0.7" />
							<line x1="24" y1="24" x2="8"  y2="42" stroke="currentColor" stroke-width="1.5" opacity="0.7" />
							<line x1="24" y1="24" x2="24" y2="6"  stroke="currentColor" stroke-width="1.5" opacity="0.5" />
							<line x1="24" y1="24" x2="42" y2="24" stroke="currentColor" stroke-width="1.5" opacity="0.5" />
							<circle cx="24" cy="24" r="3.5" fill="currentColor" opacity="0.9" />
						</svg>
					</div>
					<div>
						<h3 class="agent-name">Balrog</h3>
						<span class="agent-role">Adversarial Validator</span>
					</div>
				</div>
				<p class="agent-desc">
					Attacks the implementation before it reaches review. Analyzes the quest diff for
					failure modes, writes targeted test cases using the project's existing test framework,
					runs them, and delivers a severity-ranked findings report. Critical and High findings
					block the Review gate until addressed.
				</p>
				<div class="agent-note">
					Spawned by quest between Implement and Review. No gates, no commits — findings only.
				</div>
			</div>

			<!-- Scout -->
			<div class="agent-card animate-in" style="animation-delay: 300ms">
				<div class="agent-header">
					<div class="agent-icon scout-icon">
						<svg viewBox="0 0 48 48" width="48" height="48" fill="none" xmlns="http://www.w3.org/2000/svg">
							<circle cx="24" cy="24" r="3" fill="currentColor" opacity="0.9" />
							<path d="M24 4 L26 20 L24 21 L22 20 Z" fill="currentColor" opacity="0.6" />
							<path d="M24 44 L22 28 L24 27 L26 28 Z" fill="currentColor" opacity="0.6" />
							<path d="M4 24 L20 22 L21 24 L20 26 Z" fill="currentColor" opacity="0.6" />
							<path d="M44 24 L28 26 L27 24 L28 22 Z" fill="currentColor" opacity="0.6" />
							<path d="M9 9 L19 21 L20 22 L19 20 Z" fill="currentColor" opacity="0.25" />
							<path d="M39 39 L29 27 L28 26 L29 28 Z" fill="currentColor" opacity="0.25" />
							<path d="M39 9 L27 19 L26 20 L28 19 Z" fill="currentColor" opacity="0.25" />
							<path d="M9 39 L21 29 L22 28 L20 29 Z" fill="currentColor" opacity="0.25" />
							<circle cx="24" cy="24" r="18" stroke="currentColor" stroke-width="1" opacity="0.15" />
						</svg>
					</div>
					<div>
						<h3 class="agent-name">Scout</h3>
						<span class="agent-role">Research Agent</span>
					</div>
				</div>
				<p class="agent-desc">
					Autonomous research agent for questions that need investigation but not code
					changes. Explores the codebase, reads documentation, and optionally spawns a
					fresh adversarial validator subagent to verify findings. Produces structured
					reports with confidence levels.
				</p>
				<div class="agent-note">
					Spawned via <code>scout: &lt;question&gt;</code> in fellowship descriptions.
					No gates, no hooks, no commits.
				</div>
			</div>
		</div>
	</div>
</section>

<div class="container">
	<div class="divider"><div class="divider-ring"></div></div>
</div>

<!-- Interaction Diagram -->
<section class="diagram-section">
	<div class="container">
		<h2 class="section-title">How They Work Together</h2>

		<div class="diagram">
			<!-- Top row: You -->
			<div class="diagram-row top-row">
				<div class="diagram-node node-you">
					<span class="node-label">You</span>
				</div>
			</div>

			<!-- Connector: You -> Gandalf -->
			<div class="connector connector-vertical">
				<div class="connector-line"></div>
				<span class="connector-label left-label">task descriptions, gate approvals</span>
			</div>

			<!-- Middle row: Gandalf + Palantir -->
			<div class="diagram-row middle-row">
				<div class="palantir-watcher">
					<div class="diagram-node node-palantir">
						<span class="node-label">Palantir</span>
					</div>
					<div class="connector connector-horizontal dotted">
						<div class="connector-line"></div>
					</div>
					<span class="connector-label palantir-label">alerts (stuck, drift, conflicts)</span>
				</div>
				<div class="diagram-node node-gandalf">
					<span class="node-label">Gandalf</span>
					<span class="node-subtitle">Coordinator</span>
				</div>
			</div>

			<!-- Connectors: Gandalf -> agents -->
			<div class="diagram-row branch-row">
				<div class="branch-connector">
					<div class="branch-line left-branch"></div>
					<div class="branch-line right-branch"></div>
					<div class="branch-stem"></div>
				</div>
			</div>

			<!-- Bottom row: Quest Runners + Scouts -->
			<div class="diagram-row bottom-row">
				<div class="agent-column">
					<div class="diagram-node node-runner">
						<span class="node-label">Quest Runners</span>
					</div>
					<span class="connector-label">spawn with task</span>
					<span class="connector-label return-label">gate submissions, status updates</span>
					<!-- Balrog sub-row -->
					<div class="balrog-sub">
						<div class="balrog-connector-line"></div>
						<div class="diagram-node node-balrog">
							<span class="node-label">Balrog</span>
						</div>
						<span class="connector-label">spawn pre-review</span>
						<span class="connector-label return-label">findings report</span>
					</div>
				</div>
				<div class="agent-column">
					<div class="diagram-node node-scout">
						<span class="node-label">Scouts</span>
					</div>
					<span class="connector-label">spawn with question</span>
					<span class="connector-label return-label">research findings</span>
				</div>
			</div>
		</div>
	</div>
</section>

<style>
	/* Page header */
	.page-header {
		padding: var(--space-2xl) 0 var(--space-lg);
		text-align: center;
	}

	.page-header h1 {
		margin-bottom: var(--space-md);
	}

	.intro {
		font-size: 1.15rem;
		color: var(--color-text-secondary);
		max-width: 44rem;
		margin: 0 auto;
		line-height: 1.7;
	}

	.section-title {
		text-align: center;
		margin-bottom: var(--space-xl);
	}

	/* Agent cards */
	.agents {
		padding: var(--space-lg) 0;
	}

	.agents-grid {
		display: grid;
		grid-template-columns: 1fr;
		gap: var(--space-lg);
		max-width: 48rem;
		margin: 0 auto;
	}

	.agent-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		padding: var(--space-lg);
		border-radius: 8px;
		transition: border-color var(--transition-normal), transform var(--transition-normal), box-shadow var(--transition-normal);
	}

	.agent-card:hover {
		border-color: var(--color-accent);
		transform: translateY(-2px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}

	.agent-header {
		display: flex;
		align-items: center;
		gap: var(--space-md);
		margin-bottom: var(--space-md);
	}

	.agent-icon {
		width: 48px;
		height: 48px;
		flex-shrink: 0;
		color: var(--color-accent);
	}

	.agent-name {
		font-size: 1.35rem;
		margin-bottom: 0.1rem;
	}

	.agent-role {
		font-family: var(--font-body);
		font-size: 0.9rem;
		color: var(--color-text-secondary);
		font-style: italic;
	}

	.agent-desc {
		color: var(--color-text-secondary);
		line-height: 1.7;
		margin-bottom: var(--space-sm);
	}

	.agent-note {
		font-size: 0.9rem;
		color: var(--color-text-secondary);
		border-top: 1px solid var(--color-border);
		padding-top: var(--space-sm);
		line-height: 1.6;
	}

	.agent-note code {
		font-size: 0.85rem;
		background: var(--color-bg);
		padding: 0.1em 0.4em;
		border-radius: 3px;
		border: 1px solid var(--color-border);
	}

	/* Diagram */
	.diagram-section {
		padding: var(--space-lg) 0 var(--space-2xl);
	}

	.diagram {
		max-width: 40rem;
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0;
	}

	.diagram-row {
		display: flex;
		justify-content: center;
		align-items: center;
		width: 100%;
		position: relative;
	}

	/* Nodes */
	.diagram-node {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-sm) var(--space-lg);
		border-radius: 8px;
		border: 2px solid var(--color-border);
		background: var(--color-bg-card);
		min-width: 8rem;
		text-align: center;
	}

	.node-label {
		font-family: var(--font-heading);
		font-size: 1rem;
		color: var(--color-heading);
		letter-spacing: 0.03em;
	}

	.node-subtitle {
		font-size: 0.8rem;
		color: var(--color-text-secondary);
		font-style: italic;
	}

	.node-you {
		border-color: var(--color-text-secondary);
		background: var(--color-bg);
	}

	.node-gandalf {
		border-color: var(--color-accent);
		box-shadow: 0 0 16px rgba(218, 165, 32, 0.15);
	}

	.node-gandalf .node-label {
		color: var(--color-accent);
	}

	.node-palantir {
		border-style: dashed;
		border-color: var(--color-text-secondary);
		opacity: 0.85;
		min-width: 6.5rem;
		padding: var(--space-xs) var(--space-md);
	}

	.node-runner {
		border-color: color-mix(in srgb, var(--color-accent) 60%, transparent);
	}

	.node-scout {
		border-color: color-mix(in srgb, var(--color-accent) 60%, transparent);
	}

	/* Vertical connector */
	.connector-vertical {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		position: relative;
		padding: 0;
	}

	.connector-vertical .connector-line {
		width: 2px;
		height: 2.5rem;
		background: var(--color-border);
	}

	.connector-label {
		font-size: 0.75rem;
		color: var(--color-text-secondary);
		white-space: nowrap;
	}

	.connector-vertical .left-label {
		position: absolute;
		left: calc(50% + 12px);
		top: 50%;
		transform: translateY(-50%);
	}

	/* Middle row with Palantir */
	.middle-row {
		gap: 0;
		position: relative;
	}

	.palantir-watcher {
		display: flex;
		align-items: center;
		position: absolute;
		right: calc(50% + 7rem);
		gap: 0;
	}

	.connector-horizontal {
		display: flex;
		align-items: center;
	}

	.connector-horizontal .connector-line {
		width: 3rem;
		height: 2px;
		background: var(--color-border);
	}

	.connector-horizontal.dotted .connector-line {
		background: none;
		border-top: 2px dashed var(--color-text-secondary);
		opacity: 0.5;
		height: 0;
	}

	.palantir-label {
		position: absolute;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		text-align: center;
		font-size: 0.7rem;
	}

	/* Branch connectors */
	.branch-row {
		height: 3rem;
	}

	.branch-connector {
		position: relative;
		width: 16rem;
		height: 100%;
	}

	.branch-stem {
		position: absolute;
		left: 50%;
		top: 0;
		width: 2px;
		height: 50%;
		background: var(--color-border);
		transform: translateX(-50%);
	}

	.branch-line {
		position: absolute;
		top: 50%;
		height: 2px;
		background: var(--color-border);
	}

	.left-branch {
		left: 0;
		width: calc(50% + 1px);
	}

	.right-branch {
		right: 0;
		width: calc(50% + 1px);
	}

	.left-branch::after,
	.right-branch::after {
		content: '';
		position: absolute;
		top: 0;
		width: 2px;
		height: 1.5rem;
		background: var(--color-border);
	}

	.left-branch::after {
		left: 0;
	}

	.right-branch::after {
		right: 0;
	}

	/* Bottom row */
	.bottom-row {
		gap: var(--space-xl);
	}

	.agent-column {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.3rem;
	}

	.agent-column .connector-label {
		font-size: 0.7rem;
	}

	.return-label {
		opacity: 0.65;
		font-style: italic;
	}

	/* Balrog sub-node (spawned by Quest Runner) */
	.balrog-sub {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.3rem;
		margin-top: 0.2rem;
	}

	.balrog-connector-line {
		width: 2px;
		height: 1.5rem;
		background: var(--color-border);
		opacity: 0.6;
	}

	.node-balrog {
		border-color: color-mix(in srgb, var(--color-accent) 40%, transparent);
		border-style: dashed;
		opacity: 0.9;
	}

	/* Responsive */
	@media (max-width: 768px) {
		.page-header {
			padding: var(--space-xl) 0 var(--space-md);
		}

		.intro {
			font-size: 1.05rem;
		}

		.palantir-watcher {
			position: relative;
			right: auto;
			flex-direction: column;
			margin-bottom: var(--space-md);
		}

		.middle-row {
			flex-direction: column;
			gap: var(--space-sm);
		}

		.connector-horizontal .connector-line {
			width: 2px;
			height: 2rem;
		}

		.connector-horizontal.dotted .connector-line {
			border-top: none;
			border-left: 2px dashed var(--color-text-secondary);
			width: 0;
			height: 2rem;
		}

		.palantir-label {
			position: relative;
			top: auto;
		}

		.branch-connector {
			width: 12rem;
		}

		.bottom-row {
			gap: var(--space-md);
		}

		.diagram-node {
			min-width: 6rem;
			padding: var(--space-xs) var(--space-sm);
		}

		.node-label {
			font-size: 0.85rem;
		}
	}
</style>
