<script>
	import { base } from '$app/paths';

	const phases = [
		{
			number: 0,
			name: 'Onboard',
			description: 'Creates an isolated git worktree. Loads task-relevant context via /council. Checks for lembas checkpoints from previous sessions.',
			skill: '/council'
		},
		{
			number: 1,
			name: 'Research',
			description: 'Explores the codebase with subagents. Studies reference implementations. Extracts conventions via /gather-lore.',
			skill: '/gather-lore'
		},
		{
			number: 2,
			name: 'Plan',
			description: 'Enters plan mode. Creates implementation plan with file:line references. Presents plan for your approval.',
			skill: 'plan mode'
		},
		{
			number: 3,
			name: 'Implement',
			description: 'TDD red-green-refactor. Writes failing test, implements, verifies. If stuck: commits partial work, documents blocker, returns to Plan.',
			skill: 'test-driven-development'
		},
		{
			number: 4,
			name: 'Review',
			description: 'Convention review via /warden. Code quality verification. Runs full test suite.',
			skill: '/warden, verification-before-completion'
		},
		{
			number: 5,
			name: 'Complete',
			description: 'Creates PR. Cleans up worktree. Reports completion.',
			skill: 'finishing-a-development-branch'
		}
	];

	const fellowshipSteps = [
		{ label: 'Describe', detail: 'You describe multiple tasks (quests and scouts)' },
		{ label: 'Analyze', detail: 'Gandalf analyzes them, creates task list' },
		{ label: 'Spawn', detail: 'Spawns quest-runner agents in isolated worktrees' },
		{ label: 'Scout', detail: 'Spawns scout agents for research questions' },
		{ label: 'Monitor', detail: 'Palantir monitors progress (at 2+ quests)' },
		{ label: 'Gate', detail: 'Gates surface to you for approval' },
		{ label: 'Status', detail: 'Say "status" anytime for a progress table' },
		{ label: 'Deliver', detail: 'Each quest produces a PR; scouts produce reports' }
	];

	const scoutSteps = [
		{ name: 'Investigate', detail: 'Explores codebase, reads docs, runs searches' },
		{ name: 'Validate', detail: 'Spawns fresh adversarial subagent to challenge findings', optional: true },
		{ name: 'Deliver', detail: 'Structured report with confidence levels' }
	];
</script>

<svelte:head>
	<title>How It Works - Fellowship</title>
	<meta name="description" content="Deep dive into how Fellowship orchestrates quests, fellowships, and scouts through phased lifecycles with structural gate enforcement." />
</svelte:head>

<article class="how-it-works container">

	<header class="page-header">
		<h1>How It Works</h1>
	</header>

	<!-- Section 1: Single Task -->
	<section class="section" id="quest">
		<h2>Single Task &mdash; /quest</h2>
		<p class="section-intro">
			Run <code>/quest</code> for any non-trivial task. It walks through six phases with hard gates between each.
		</p>

		<div class="timeline">
			{#each phases as phase, i (phase.number)}
				<div class="timeline-step">
					<div class="timeline-track">
						<div class="timeline-marker">
							<span class="marker-number">{phase.number}</span>
						</div>
						{#if i < phases.length - 1}
							<div class="timeline-line"></div>
						{/if}
					</div>
					<div class="timeline-content">
						<h3 class="phase-name">Phase {phase.number}: {phase.name}</h3>
						<p class="phase-desc">{phase.description}</p>
						<span class="skill-tag">{phase.skill}</span>
					</div>
				</div>

				{#if i < phases.length - 1}
					<div class="gate-indicator">
						<div class="gate-track">
							<div class="gate-line"></div>
						</div>
						<div class="gate-badge">
							<svg class="gate-lock" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
								<rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
								<path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
							</svg>
							<span>GATE &mdash; requires approval</span>
						</div>
					</div>
				{/if}
			{/each}
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Section 2: Multiple Tasks -->
	<section class="section" id="fellowship">
		<h2>Multiple Tasks &mdash; /fellowship</h2>
		<p class="section-intro">
			Run <code>/fellowship</code> for multiple independent tasks. Gandalf coordinates parallel agents.
		</p>

		<div class="flow-grid">
			{#each fellowshipSteps as step, i (step.label)}
				<div class="flow-card">
					<div class="flow-number">{i + 1}</div>
					<h3 class="flow-label">{step.label}</h3>
					<p class="flow-detail">{step.detail}</p>
				</div>
				{#if i < fellowshipSteps.length - 1}
					<div class="flow-arrow">
						<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M5 12h14M12 5l7 7-7 7"></path>
						</svg>
					</div>
				{/if}
			{/each}
		</div>

		<div class="parallel-diagram">
			<div class="parallel-header">
				<span class="parallel-label">Gandalf</span>
				<span class="parallel-sublabel">coordinator</span>
			</div>
			<div class="parallel-branches">
				<div class="branch quest-branch">
					<div class="branch-line"></div>
					<div class="branch-tag">Quest 1</div>
					<div class="branch-detail">worktree &rarr; phases 0-5 &rarr; PR</div>
				</div>
				<div class="branch quest-branch">
					<div class="branch-line"></div>
					<div class="branch-tag">Quest 2</div>
					<div class="branch-detail">worktree &rarr; phases 0-5 &rarr; PR</div>
				</div>
				<div class="branch scout-branch">
					<div class="branch-line"></div>
					<div class="branch-tag">Scout</div>
					<div class="branch-detail">investigate &rarr; validate &rarr; report</div>
				</div>
				<div class="branch monitor-branch">
					<div class="branch-line"></div>
					<div class="branch-tag">Palantir</div>
					<div class="branch-detail">monitors all active quests</div>
				</div>
			</div>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Section 3: Research -->
	<section class="section" id="scout">
		<h2>Research &mdash; /scout</h2>
		<p class="section-intro">
			For questions that need investigation but not code changes.
		</p>

		<div class="scout-flow">
			{#each scoutSteps as step, i (step.name)}
				<div class="scout-step">
					<div class="scout-marker">
						<span class="scout-number">{i + 1}</span>
					</div>
					<div class="scout-content">
						<h3 class="scout-name">
							{step.name}
							{#if step.optional}
								<span class="optional-badge">optional</span>
							{/if}
						</h3>
						<p class="scout-detail">{step.detail}</p>
					</div>
				</div>
				{#if i < scoutSteps.length - 1}
					<div class="scout-connector">
						<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M12 5v14M5 12l7 7 7-7"></path>
						</svg>
					</div>
				{/if}
			{/each}
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Section 4: Gate Enforcement -->
	<section class="section" id="gate-enforcement">
		<h2>Gate Enforcement</h2>
		<p class="section-intro">
			Gates aren't just prompts &mdash; they're structurally enforced.
		</p>

		<div class="enforcement-grid">
			<div class="enforcement-card">
				<h3>Hook-Blocked Tools</h3>
				<p>Plugin hooks block tool access (Edit, Write, Bash, etc.) after gate submission.</p>
			</div>
			<div class="enforcement-card">
				<h3>Lead Approval Required</h3>
				<p>Tools stay blocked until the lead agent approves by writing to the quest state file.</p>
			</div>
			<div class="enforcement-card">
				<h3>Verified Prerequisites</h3>
				<p>Running /lembas and updating task metadata are verified before gate submission.</p>
			</div>
			<div class="enforcement-card">
				<h3>No Self-Approval</h3>
				<p>Self-approval is structurally impossible. The agent that submits the gate cannot approve it.</p>
			</div>
		</div>

		<div class="compliance-callout">
			<div class="compliance-comparison">
				<div class="compliance-before">
					<span class="compliance-label">Prompt-only compliance</span>
					<span class="compliance-value dim">~33%</span>
				</div>
				<div class="compliance-arrow">
					<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M5 12h14M12 5l7 7-7 7"></path>
					</svg>
				</div>
				<div class="compliance-after">
					<span class="compliance-label">Hook-enforced compliance</span>
					<span class="compliance-value gold">~95%+</span>
				</div>
			</div>
		</div>
	</section>

</article>

<style>
	.how-it-works {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-2xl);
	}

	.page-header {
		margin-bottom: var(--space-xl);
	}

	.page-header h1 {
		margin-bottom: var(--space-sm);
	}

	.section {
		margin-bottom: var(--space-lg);
	}

	.section h2 {
		margin-bottom: var(--space-md);
	}

	.section-intro {
		font-size: 1.15rem;
		color: var(--color-text-secondary);
		margin-bottom: var(--space-xl);
		max-width: 40rem;
		line-height: 1.7;
	}

	/* ===== Timeline (Quest Phases) ===== */
	.timeline {
		max-width: 42rem;
		margin: 0 auto;
	}

	.timeline-step {
		display: flex;
		gap: var(--space-md);
	}

	.timeline-track {
		display: flex;
		flex-direction: column;
		align-items: center;
		flex-shrink: 0;
		width: 3rem;
	}

	.timeline-marker {
		width: 3rem;
		height: 3rem;
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
		font-size: 1rem;
		color: var(--color-accent);
		font-weight: 700;
	}

	.timeline-line {
		width: 2px;
		flex: 1;
		background: var(--color-accent);
		opacity: 0.4;
	}

	.timeline-content {
		padding: 0.25rem 0 var(--space-md) 0;
		flex: 1;
	}

	.phase-name {
		font-size: 1.2rem;
		margin-bottom: var(--space-xs);
		color: var(--color-heading);
	}

	.phase-desc {
		color: var(--color-text-secondary);
		line-height: 1.6;
		margin-bottom: var(--space-xs);
	}

	.skill-tag {
		display: inline-block;
		font-family: var(--font-code, monospace);
		font-size: 0.8rem;
		background: var(--color-code-bg, var(--color-bg-elevated));
		color: var(--color-accent);
		padding: 0.15em 0.5em;
		border-radius: 4px;
		border: 1px solid var(--color-border);
	}

	/* Gate indicator between phases */
	.gate-indicator {
		display: flex;
		gap: var(--space-md);
		padding: var(--space-sm) 0;
	}

	.gate-track {
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 3rem;
		flex-shrink: 0;
	}

	.gate-line {
		width: 2px;
		height: 100%;
		min-height: 1.5rem;
		border-left: 2px dashed var(--color-accent);
		opacity: 0.5;
	}

	.gate-badge {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		font-family: var(--font-heading);
		font-size: 0.75rem;
		letter-spacing: 0.08em;
		color: var(--color-accent);
		border: 1px dashed var(--color-accent);
		border-radius: 4px;
		padding: var(--space-xs) var(--space-sm);
		background: var(--color-bg-card);
		white-space: nowrap;
	}

	.gate-lock {
		color: var(--color-accent);
		flex-shrink: 0;
	}

	/* ===== Fellowship Flow ===== */
	.flow-grid {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: var(--space-sm);
		margin-bottom: var(--space-xl);
	}

	.flow-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: var(--space-md);
		width: 10rem;
		text-align: center;
		transition: border-color var(--transition-normal);
	}

	.flow-card:hover {
		border-color: var(--color-accent);
	}

	.flow-number {
		font-family: var(--font-heading);
		font-size: 0.85rem;
		font-weight: 700;
		color: var(--color-accent);
		width: 1.75rem;
		height: 1.75rem;
		border: 2px solid var(--color-accent);
		border-radius: 50%;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		margin-bottom: var(--space-xs);
	}

	.flow-label {
		font-size: 1rem;
		margin-bottom: var(--space-xs);
	}

	.flow-detail {
		font-size: 0.85rem;
		color: var(--color-text-secondary);
		line-height: 1.5;
	}

	.flow-arrow {
		color: var(--color-accent);
		opacity: 0.5;
		flex-shrink: 0;
	}

	/* Parallel branch diagram */
	.parallel-diagram {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: var(--space-lg);
		max-width: 42rem;
		margin: 0 auto;
	}

	.parallel-header {
		text-align: center;
		margin-bottom: var(--space-lg);
		padding-bottom: var(--space-md);
		border-bottom: 2px solid var(--color-accent);
	}

	.parallel-label {
		font-family: var(--font-heading);
		font-size: 1.3rem;
		color: var(--color-accent);
		display: block;
	}

	.parallel-sublabel {
		font-size: 0.85rem;
		color: var(--color-text-secondary);
	}

	.parallel-branches {
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}

	.branch {
		display: flex;
		align-items: center;
		gap: var(--space-md);
		padding: var(--space-sm) var(--space-md);
		border-radius: 6px;
		background: var(--color-bg-elevated);
	}

	.branch-line {
		width: 3px;
		height: 2rem;
		border-radius: 2px;
		flex-shrink: 0;
	}

	.quest-branch .branch-line {
		background: var(--color-accent);
	}

	.scout-branch .branch-line {
		background: #6ba3be;
	}

	.monitor-branch .branch-line {
		background: #9b8ec4;
	}

	.branch-tag {
		font-family: var(--font-heading);
		font-size: 0.95rem;
		font-weight: 700;
		color: var(--color-heading);
		min-width: 5rem;
	}

	.branch-detail {
		font-size: 0.9rem;
		color: var(--color-text-secondary);
	}

	/* ===== Scout Flow ===== */
	.scout-flow {
		max-width: 28rem;
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	.scout-step {
		display: flex;
		align-items: flex-start;
		gap: var(--space-md);
		width: 100%;
	}

	.scout-marker {
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 50%;
		border: 2px solid var(--color-accent);
		background: var(--color-bg);
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.scout-number {
		font-family: var(--font-heading);
		font-size: 0.9rem;
		color: var(--color-accent);
		font-weight: 700;
	}

	.scout-content {
		padding-top: 0.25rem;
	}

	.scout-name {
		font-size: 1.15rem;
		margin-bottom: var(--space-xs);
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.optional-badge {
		font-family: var(--font-body);
		font-size: 0.7rem;
		font-weight: 400;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--color-text-secondary);
		border: 1px solid var(--color-border);
		border-radius: 3px;
		padding: 0.1em 0.4em;
	}

	.scout-detail {
		color: var(--color-text-secondary);
		line-height: 1.6;
	}

	.scout-connector {
		color: var(--color-accent);
		opacity: 0.4;
		padding: var(--space-xs) 0;
		display: flex;
		justify-content: center;
		width: 2.5rem;
	}

	/* ===== Gate Enforcement ===== */
	.enforcement-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-md);
		margin-bottom: var(--space-xl);
	}

	.enforcement-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		padding: var(--space-lg);
		border-radius: 8px;
		transition: border-color var(--transition-normal);
	}

	.enforcement-card:hover {
		border-color: var(--color-accent);
	}

	.enforcement-card h3 {
		font-size: 1.1rem;
		margin-bottom: var(--space-sm);
	}

	.enforcement-card p {
		color: var(--color-text-secondary);
		line-height: 1.6;
		font-size: 0.95rem;
	}

	/* Compliance callout */
	.compliance-callout {
		background: var(--color-bg-card);
		border: 2px solid var(--color-accent);
		border-radius: 12px;
		padding: var(--space-xl);
	}

	.compliance-comparison {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-xl);
	}

	.compliance-before,
	.compliance-after {
		text-align: center;
	}

	.compliance-label {
		display: block;
		font-family: var(--font-heading);
		font-size: 0.85rem;
		letter-spacing: 0.04em;
		color: var(--color-text-secondary);
		margin-bottom: var(--space-sm);
	}

	.compliance-value {
		display: block;
		font-family: var(--font-heading);
		font-size: 3rem;
		font-weight: 700;
		line-height: 1;
	}

	.compliance-value.dim {
		color: var(--color-text-secondary);
		opacity: 0.6;
	}

	.compliance-value.gold {
		color: var(--color-accent);
	}

	.compliance-arrow {
		color: var(--color-accent);
		flex-shrink: 0;
	}

	/* ===== Responsive ===== */
	@media (max-width: 768px) {
		.flow-grid {
			flex-direction: column;
		}

		.flow-card {
			width: 100%;
		}

		.flow-arrow {
			transform: rotate(90deg);
		}

		.enforcement-grid {
			grid-template-columns: 1fr;
		}

		.compliance-comparison {
			flex-direction: column;
			gap: var(--space-md);
		}

		.compliance-value {
			font-size: 2.25rem;
		}

		.compliance-arrow {
			transform: rotate(90deg);
		}

		.parallel-diagram {
			padding: var(--space-md);
		}

		.branch {
			flex-direction: column;
			align-items: flex-start;
			gap: var(--space-xs);
		}

		.branch-line {
			width: 2rem;
			height: 3px;
		}
	}
</style>
