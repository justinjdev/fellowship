<script lang="ts">
	import { base } from '$app/paths';
</script>

<svelte:head>
	<title>Changelog | Fellowship</title>
	<meta name="description" content="Fellowship release history and version changelog." />
</svelte:head>

<div class="container page">
	<h1>Changelog</h1>

	<!-- v1.6.0 -->
	<section class="version" id="v1-6-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-6-0">v1.6.0</a></h2>
		<ul class="changes">
			<li>
				<strong><code>/scout</code> skill</strong> — Research &amp; analysis workflow for lightweight research teammates alongside code quests. Autonomous (no gates/hooks), optional adversarial validation via fresh subagent.
			</li>
			<li>
				<strong>Fellowship scouts</strong> — Gandalf learns to spawn scouts via <code>"scout: &lt;question&gt;"</code> alongside code quests, with status tracking and optional routing to other teammates.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.5.1 -->
	<section class="version" id="v1-5-1">
		<h2 class="version-heading"><a href="{base}/changelog#v1-5-1">v1.5.1</a></h2>
		<ul class="changes">
			<li>
				<strong>Go CLI</strong> — <code>fellowship</code> binary replaces bash hook scripts. Handles hook logic, gate approval/rejection, install/uninstall, and status. Distributed via GitHub releases, auto-downloaded on first use.
			</li>
			<li>
				<strong>Plugin subfolder</strong> — Plugin files moved to <code>plugin/</code> for clean installs via marketplace <code>git-subdir</code>. Go source, CI, and build config stay at repo root.
			</li>
			<li>
				<strong>Quest runner agent</strong> — <code>agents/quest-runner.md</code> for CLI-driven quest execution.
			</li>
			<li class="breaking">
				<strong>BREAKING</strong> — Bash hook scripts replaced by Go CLI binary. <code>jq</code> no longer required.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.5.0 -->
	<section class="version" id="v1-5-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-5-0">v1.5.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Gate state machine</strong> — Structural enforcement of quest phase gates via plugin hooks. Teammate tools are blocked after gate submission until the lead approves. Prerequisites (lembas + metadata) are verified before submission. Self-approval is structurally impossible. Observed compliance: ~33% with prompt-only to ~95%+ with hooks.
			</li>
			<li>
				<strong>Hook scripts</strong> — 4 plugin hooks (<code>gate-guard</code>, <code>gate-submit</code>, <code>gate-prereq</code>, <code>metadata-track</code>) with test suite.
			</li>
			<li>
				<strong><code>jq</code> dependency</strong> — Required for gate enforcement. Hooks fail-closed if jq is missing.
			</li>
			<li class="breaking">
				<strong>BREAKING</strong> — Plugin now ships executable bash scripts (<code>hooks/scripts/</code>). Previously pure markdown only.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.4.0 -->
	<section class="version" id="v1-4-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-4-0">v1.4.0</a></h2>
		<ul class="changes">
			<li>
				<strong><code>gather-lore</code> rewrite</strong> — Simplified to study-only (pattern extraction). Code generation and diff checking removed as redundant with quest Implement + warden Review phases.
			</li>
			<li>
				<strong><code>/red-book</code> skill</strong> — New skill for capturing conventions from PR reviewer feedback into <code>CLAUDE.md</code>. Closes the convention learning loop.
			</li>
			<li>
				<strong>Quest recovery</strong> — Phase 3 now has explicit recovery procedure: when implementation hits a wall, stop, commit partial work, document the blocker, return to Plan phase.
			</li>
			<li>
				<strong>Quest resume</strong> — Failed/dead quests can be respawned into their existing worktree. Council finds the lembas checkpoint and offers to resume.
			</li>
			<li>
				<strong>Palantir fix</strong> — Spawned as <code>fellowship:palantir</code> (custom agent with restricted tools) instead of <code>general-purpose</code>.
			</li>
			<li>
				<strong>Palantir cadence</strong> — Event-driven monitoring triggered by Gandalf after gate transitions and quest spawns, instead of unbounded.
			</li>
			<li>
				<strong>Worktree ownership</strong> — Quest Phase 0 owns worktree creation. Fellowship no longer passes <code>isolation: "worktree"</code>, eliminating double-worktree conflicts.
			</li>
			<li>
				<strong>Config schema dedup</strong> — Canonical schema lives in <code>/settings</code>. Fellowship references it instead of duplicating.
			</li>
			<li>
				<strong><code>branchPrefix</code> removed</strong> — Deprecated key fully removed from all skills and config.
			</li>
			<li>
				<strong>Escape hatch criteria</strong> — Concrete heuristics (single file, &lt; 50 lines, no new patterns, familiar area) replace "use judgment".
			</li>
			<li>
				<strong>Monorepo conditional</strong> — Council package scope step now skips for single-package repos.
			</li>
			<li>
				<strong>Nested subagent worktrees removed</strong> — If plan subtasks have file conflicts, fix the plan.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.3.0 -->
	<section class="version" id="v1-3-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-3-0">v1.3.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Branch name patterns</strong> — <code>branch.pattern</code> config with flexible template system. Supports <code>{'{slug}'}</code>, <code>{'{ticket}'}</code>, and <code>{'{author}'}</code> placeholders for team-specific branch naming conventions. <span class="breaking-inline">Breaking:</span> removed <code>branchPrefix</code> (deprecated). Use <code>branch.pattern</code> instead.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.2.0 -->
	<section class="version" id="v1-2-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-2-0">v1.2.0</a></h2>
		<ul class="changes">
			<li>
				<strong><code>/config</code> command</strong> — Interactive skill to view, edit, and reset fellowship settings.
			</li>
			<li>
				<strong>Config moved to personal directory</strong> — <code>~/.claude/fellowship.json</code> loaded from user's personal Claude directory instead of project root.
			</li>
			<li>
				<strong>Custom worktree directory</strong> — <code>worktree.directory</code> config option.
			</li>
			<li>
				<strong>Removed <code>superpowers:using-git-worktrees</code> dependency</strong> — Quest now uses <code>EnterWorktree</code> directly.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.1.0 -->
	<section class="version" id="v1-1-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-1-0">v1.1.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Config file support</strong> — <code>~/.claude/fellowship.json</code> for customizing branch prefixes, gate auto-approval, PR defaults, worktree strategy, and palantir settings.
			</li>
			<li>
				<strong>Palantir rewrite</strong> — Rewrote from dead code into functional monitoring agent.
			</li>
			<li>
				<strong>Progress tracking</strong> — Teammates report current phase via task metadata; say "status" for a progress table.
			</li>
			<li>
				<strong>Gate blocking fix</strong> — Replaced ineffective "WAIT" instruction with explicit turn-ending.
			</li>
			<li>
				<strong>Lembas compaction at all transitions</strong> — Added missing <code>/lembas</code> invocations.
			</li>
			<li>
				<strong>Steward removed</strong> — Deleted dead agent; logic was already inlined.
			</li>
			<li>
				<strong>Gate discipline</strong> — Gandalf must never combine or skip gate approvals.
			</li>
			<li>
				<strong>Conventional commits</strong> — Spawn prompt and quest guidelines now enforce conventional commit format.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.0.0 -->
	<section class="version" id="v1-0-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-0-0">v1.0.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Initial release</strong> — Quest lifecycle, fellowship orchestration, council, gather-lore, lembas, warden, chronicle.
			</li>
		</ul>
	</section>
</div>

<style>
	.page {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-2xl);
	}

	h1 {
		margin-bottom: var(--space-lg);
	}

	.version {
		padding: var(--space-sm) 0;
	}

	.version-heading {
		font-family: var(--font-heading);
		font-size: 1.6rem;
		margin-bottom: var(--space-md);
	}

	.version-heading a {
		color: var(--color-heading);
		text-decoration: none;
		transition: opacity 0.2s ease;
	}

	.version-heading a:hover {
		opacity: 0.8;
	}

	.changes {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.changes li {
		padding: var(--space-sm) var(--space-md);
		border-left: 3px solid var(--color-border);
		line-height: 1.7;
		color: var(--color-text);
		font-size: 1.05rem;
	}

	.changes li:hover {
		border-left-color: var(--color-accent);
	}

	.changes li :global(strong) {
		color: var(--color-text);
	}

	.changes li :global(code) {
		color: var(--color-accent);
		font-size: 0.92em;
		background: rgba(218, 165, 32, 0.08);
		padding: 0.1em 0.35em;
		border-radius: 3px;
	}

	.breaking {
		border-left-color: var(--color-error) !important;
		background: var(--color-error-bg);
		border-radius: 0 6px 6px 0;
	}

	.breaking-inline {
		color: var(--color-error);
		font-weight: 700;
	}
</style>
