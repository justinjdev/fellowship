<script lang="ts">
	import { base } from '$app/paths';

	const skills = [
		{
			name: '/quest',
			summary: 'Full Research \u2192 Plan \u2192 Implement lifecycle for non-trivial tasks.',
			details: 'The hub skill that orchestrates everything. Takes a task description, creates an isolated worktree, and walks through six phases: Onboard, Research, Plan, Implement, Review, Complete. Each phase has a hard gate requiring approval before proceeding. Uses /council for context, /gather-lore for conventions, /lembas for context compression between phases, and /warden for pre-PR review.'
		},
		{
			name: '/fellowship',
			summary: 'Multi-task orchestrator. Spawns parallel agent teammates.',
			details: 'For multiple independent tasks, Gandalf (the coordinator) spawns quest and scout teammates. Quests run in isolated worktrees and produce PRs. Scouts research questions and deliver findings. Say \u2018status\u2019 during a fellowship for a progress table. Gates surface to you for approval by default.'
		},
		{
			name: '/scout',
			summary: 'Research & analysis workflow. No code, no PRs, no commits.',
			details: 'Autonomous research agent that investigates questions with configurable depth. For complex questions, spawns a fresh adversarial validator subagent to verify findings. Produces a structured report with confidence levels. Use alongside /quest in a fellowship for research questions that don\u2019t need code changes.'
		},
		{
			name: '/council',
			summary: 'Context-aware onboarding at session start.',
			details: 'Loads task-relevant files, conventions, and architecture. Checks for lembas checkpoints from previous sessions and offers to resume. Scopes to the relevant package in monorepos. Run at the start of any session or quest.'
		},
		{
			name: '/gather-lore',
			summary: 'Studies reference files to extract conventions.',
			details: 'Analyzes your codebase to extract patterns before writing code. Examines existing implementations to understand naming conventions, file organization, testing patterns, and architectural decisions. Prevents \u2018wrong approach\u2019 rework by learning from what\u2019s already there.'
		},
		{
			name: '/lembas',
			summary: 'Context compression between phases.',
			details: 'Compacts the conversation context at phase transitions. Keeps the context window in the reasoning sweet spot by summarizing what\u2019s been done and what needs to happen next. Invoked automatically at all four phase transitions during a quest.'
		},
		{
			name: '/warden',
			summary: 'Pre-PR convention review.',
			details: 'Compares your changes against reference files and documented patterns in CLAUDE.md. Catches convention violations before they reach PR review. Checks naming, file organization, testing patterns, and architectural consistency.'
		},
		{
			name: '/chronicle',
			summary: 'One-time codebase bootstrapping.',
			details: 'Walks through your project to extract conventions into CLAUDE.md. Run once when setting up Fellowship in a new codebase. Examines your code structure, testing patterns, naming conventions, and documents them for future quests.'
		},
		{
			name: '/red-book',
			summary: 'Post-PR convention capture.',
			details: 'After a PR review, extracts conventions from reviewer comments and adds them to CLAUDE.md. Closes the convention learning loop \u2014 reviewer feedback becomes documented patterns that future quests will follow.'
		},
		{
			name: '/retro',
			summary: 'Post-fellowship retrospective analysis.',
			details: 'Analyzes a completed fellowship\u2019s gate history, palantir alerts, and quest metrics to surface patterns. Identifies which gates added value, which phases caused delays, and interactively recommends configuration changes like auto-approving gates with zero rejection rates.'
		},
		{
			name: '/settings',
			summary: 'View or edit fellowship settings.',
			details: 'Interactive setup for all configuration options in ~/.claude/fellowship.json. View current settings, edit individual values, or reset to defaults.'
		}
	];
</script>

<svelte:head>
	<title>Skills | Fellowship</title>
	<meta name="description" content="Fellowship slash commands that orchestrate different parts of the workflow." />
</svelte:head>

<div class="container page">
	<h1>Skills</h1>
	<p class="intro">
		Fellowship skills are slash commands that orchestrate different parts of the workflow.
		Each skill is a structured prompt — no runtime code.
	</p>

	<div class="divider"><span class="divider-ring"></span></div>

	<div class="skills-list">
		{#each skills as skill, i (skill.name)}
			<details class="skill-card animate-in" style="animation-delay: {i * 100}ms">
				<summary>
					<span class="chevron" aria-hidden="true"></span>
					<code class="skill-name">{skill.name}</code>
					<span class="skill-summary">{skill.summary}</span>
				</summary>
				<p class="skill-details">{skill.details}</p>
			</details>
		{/each}
	</div>
</div>

<style>
	.page {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-2xl);
	}

	h1 {
		margin-bottom: var(--space-md);
	}

	.intro {
		font-size: 1.15rem;
		color: var(--color-text-secondary);
		max-width: 42em;
		line-height: 1.7;
	}

	.skills-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}

	.skill-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: var(--space-lg);
		transition: border-color var(--transition-normal), transform var(--transition-normal), box-shadow var(--transition-normal);
	}

	.skill-card:hover {
		border-color: var(--color-accent);
		transform: translateY(-2px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}

	.skill-card summary {
		display: flex;
		align-items: baseline;
		gap: var(--space-sm);
		cursor: pointer;
		list-style: none;
		font-family: var(--font-body);
		font-size: 1.1rem;
		line-height: 1.5;
	}

	.skill-card summary::-webkit-details-marker {
		display: none;
	}

	.chevron {
		display: inline-block;
		width: 0;
		height: 0;
		border-left: 6px solid var(--color-accent);
		border-top: 4px solid transparent;
		border-bottom: 4px solid transparent;
		flex-shrink: 0;
		transition: transform 0.2s ease;
		position: relative;
		top: 0.05em;
	}

	.skill-card[open] .chevron {
		transform: rotate(90deg);
	}

	.skill-name {
		color: var(--color-accent);
		font-weight: 700;
		white-space: nowrap;
	}

	.skill-summary {
		color: var(--color-text);
	}

	.skill-details {
		margin-top: var(--space-md);
		padding-left: calc(6px + var(--space-sm));
		color: var(--color-text-secondary);
		line-height: 1.7;
		max-width: 60em;
	}
</style>
