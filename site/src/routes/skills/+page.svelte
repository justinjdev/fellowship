<script lang="ts">
	import { base } from '$app/paths';

	const skills = [
		{
			name: '/quest',
			summary: 'Full Research → Plan → Implement lifecycle for non-trivial tasks.',
			details: 'The hub skill that orchestrates everything. Takes a task description and walks through four phases: Research, Plan, Implement, Review. A hard gate leaves each of the first three and needs approval; nothing leaves Review, where a balrog agent attacks the implementation, /warden checks conventions, the work is verified, and the PR is opened. Research provisions the isolated worktree, loads task context, and studies the reference files for conventions; /lembas compacts context between every phase. Supports plan-driven mode: provide a pre-existing plan file and the quest skips Research and Plan, starting at Implement with one gate. Supports promoted mode: a quest promoted from a scout validates and supplements the scout findings instead of researching from scratch. Includes a bulletin board for cross-quest knowledge sharing: quests scan it at the start of Research and post discoveries during Research and Implement, shared across all worktrees via the main repo root.'
		},
		{
			name: '/fellowship',
			summary: 'Multi-task orchestrator. Spawns parallel agent teammates.',
			details: 'For multiple independent tasks, Gandalf (the coordinator) spawns quest and scout teammates. Quests run in isolated worktrees and produce PRs. Scouts research questions and deliver findings. Say \'status\' during a fellowship for a progress table. Gates surface to you for approval by default. Supports plan-driven quests: provide a plan file and Gandalf spawns quests that skip to Implement. For large plans, Gandalf can fan out into multiple parallel quests after confirming the split with you. Supports scout-to-quest promotion: say \'promote scout-X to a quest\' and Gandalf spawns a quest pre-loaded with the scout\'s findings. The bulletin board enables cross-quest knowledge sharing — cleared automatically on disband.'
		},
		{
			name: '/scout',
			summary: 'Research & analysis workflow. No code, no PRs, no commits.',
			details: 'Autonomous research agent that investigates questions with configurable depth. For complex questions, spawns a fresh adversarial validator subagent to verify findings. Produces a structured report with confidence levels. Writes findings to a namespaced file (.fellowship/scout-findings-{name}.md) that can be promoted into a quest. Use alongside /quest in a fellowship for research questions that don’t need code changes.'
		},
		{
			name: '/council',
			summary: 'Context-aware onboarding at session start.',
			details: 'Loads task-relevant files, conventions, and architecture, scoped to the relevant package in monorepos. Invoke it yourself at the start of a session; quest inlines the same orientation as its Research step 2, so it does not call this. It does not look for checkpoints — quest and /rekindle handle resuming.'
		},
		{
			name: '/gather-lore',
			summary: 'Studies reference files to extract conventions.',
			details: 'Analyzes your codebase to extract patterns before writing code. Examines existing implementations to understand naming conventions, file organization, testing patterns, and architectural decisions. Prevents ‘wrong approach’ rework by learning from what’s already there. Quest inlines the same extraction as its Research step 4, so this is for when you want it on its own.'
		},
		{
			name: '/lembas',
			summary: 'Context compression between phases.',
			details: 'Writes a structured checkpoint at a phase transition and continues from that summary instead of the full history, keeping the context window in the reasoning sweet spot. The checkpoint at .fellowship/checkpoint.md is also what a crashed quest resumes from. Invoked at every phase transition during a quest, where the hooks verify it before a gate can be submitted.'
		},
		{
			name: '/warden',
			summary: 'Pre-PR convention review.',
			details: 'Compares your changes against reference files and documented patterns in CLAUDE.md. Catches convention violations before they reach PR review. Checks naming, file organization, testing patterns, and architectural consistency.'
		},
		{
			name: '/retro',
			summary: 'Post-fellowship retrospective analysis.',
			details: 'Analyzes a completed fellowship’s gate history, palantir alerts, and quest metrics to surface patterns. Identifies which gates added value, which phases caused delays, and interactively recommends configuration changes like auto-approving gates with zero rejection rates.'
		},
		{
			name: '/missive',
			summary: 'Fetch GitHub issue context for quest spawning.',
			details: 'Pulls structured context from GitHub issues via gh CLI — title, body, labels, and recent comments. Returns a package with issue context, a suggested branch name incorporating the issue number, and PR closing keywords (Closes #N). Gandalf invokes it automatically when issue references (#N) are detected in quest descriptions. Also usable standalone: /missive 42 to preview what context a quest would receive.'
		},
		{
			name: '/lorebook',
			summary: 'Load phase-specific guidance from a quest template.',
			details: 'Invoked at the start of each quest phase when your spawn prompt includes a TEMPLATE: assignment. Resolves the template from the project (.claude/fellowship-templates/), user (~/.claude/fellowship-templates/), or built-in directory, reads the section matching the current phase — one each for Research, Plan, Implement, and Review — and applies it as advisory context that never waives a gate. Fellowship ships one example template to copy; /scribe writes real ones.'
		}
	];

	const commands = [
		{
			name: '/chronicle',
			summary: 'One-time codebase bootstrapping.',
			details: 'Walks through your project to extract conventions into CLAUDE.md. Run once when setting up Fellowship in a new codebase. Examines your code structure, testing patterns, naming conventions, and documents them for future quests.'
		},
		{
			name: '/dashboard',
			summary: 'Live web dashboard for the current fellowship.',
			details: 'Starts a local HTTP server (default http://localhost:3000) in the background and prints its URL. Shows every quest\'s phase and pending gates, scouts, companies (with one-click "Approve All"), the recent event stream, the bulletin board, and eagles health — with gate approve/reject directly from the page.'
		},
		{
			name: '/red-book',
			summary: 'Post-PR convention capture.',
			details: 'After a PR review, extracts conventions from reviewer comments and adds them to CLAUDE.md. Closes the convention learning loop — reviewer feedback becomes documented patterns that future quests will follow.'
		},
		{
			name: '/settings',
			summary: 'View or edit fellowship settings.',
			details: 'Interactive setup for all configuration options in ~/.claude/fellowship.json. View current settings, edit individual values, or reset to defaults.'
		},
		{
			name: '/scribe',
			summary: 'Create reusable quest templates from codebase conventions.',
			details: 'Analyzes your project to create quest templates that encode project-specific knowledge — conventions Claude wouldn’t know, domain rules that aren’t in code, team workflows that matter. A template carries one guidance section per quest phase (Research, Plan, Implement, Review), loaded by /lorebook as each phase starts. Stored in .claude/fellowship-templates/ (project) or ~/.claude/fellowship-templates/ (personal).'
		},
		{
			name: '/guide',
			summary: 'Learn fellowship by doing.',
			details: 'Interactive walkthrough that teaches fellowship by running a real task on your codebase. Walks you through research, planning, and implementation with natural-language checkpoints at each stage. Produces a real PR. Afterward, introduces /quest and /fellowship for continued use. Designed for people new to agent orchestration.'
		},
		{
			name: '/rekindle',
			summary: 'Recover a fellowship after a session crash.',
			details: 'Scans worktrees and state files to reconstruct fellowship state from on-disk artifacts. Presents a recovery dashboard showing which quests are resumable (have checkpoints), stale (no checkpoint), or already complete (branch merged). On confirmation, re-spawns Gandalf and quest runners with recovered context.'
		}
	];
</script>

<svelte:head>
	<title>Skills & Commands | Fellowship</title>
	<meta name="description" content="Fellowship skills and commands that orchestrate different parts of the workflow." />
</svelte:head>

<div class="container page">
	<h1>Skills & Commands</h1>
	<p class="intro">
		Fellowship ships two kinds of slash-typeable prompts. <strong>Skills</strong> are invoked automatically
		by Claude at the right moment in a workflow — you can also run them directly with their name.
		<strong>Commands</strong> only run when you type them yourself; Claude never invokes them on its own.
		Each is a structured prompt — no runtime code.
	</p>

	<div class="divider"><span class="divider-ring"></span></div>

	<h2 class="group-heading">Skills (auto-invoked)</h2>
	<p class="group-intro">Loaded automatically by Claude as part of a quest, fellowship, or scout workflow.</p>

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

	<h2 class="group-heading">Commands (user-invoked)</h2>
	<p class="group-intro">Run only when you type them — no automatic invocation, no base context cost.</p>

	<div class="skills-list">
		{#each commands as command, i (command.name)}
			<details class="skill-card animate-in" style="animation-delay: {i * 100}ms">
				<summary>
					<span class="chevron" aria-hidden="true"></span>
					<code class="skill-name">{command.name}</code>
					<span class="skill-summary">{command.summary}</span>
				</summary>
				<p class="skill-details">{command.details}</p>
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

	.group-heading {
		margin-top: var(--space-xl);
		margin-bottom: var(--space-xs);
	}

	.group-intro {
		color: var(--color-text-secondary);
		max-width: 42em;
		line-height: 1.6;
		margin-bottom: var(--space-md);
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
