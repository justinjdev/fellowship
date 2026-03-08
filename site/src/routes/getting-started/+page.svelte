<script>
	import { base } from '$app/paths';
	import CopyButton from '$lib/components/CopyButton.svelte';

	const installCmd1 = '/plugin marketplace add justinjdev/claude-plugins';
	const installCmd2 = '/plugin install fellowship@justinjdev';
	const superpowersCmd1 = '/plugin marketplace add obra/superpowers-marketplace';
	const superpowersCmd2 = '/plugin install superpowers@superpowers-marketplace';
	const prReviewCmd = '/plugin install pr-review-toolkit@claude-plugins-official';
	const questCmd = '/quest "your task description"';
	const hookConfig = `{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "if [ -f .fellowship/checkpoint.md ]; then echo '--- CHECKPOINT DETECTED ---'; cat .fellowship/checkpoint.md; echo '--- END CHECKPOINT ---'; echo 'A checkpoint from a previous session was found. Use /council to resume or start fresh.'; fi"
          }
        ]
      }
    ]
  }
}`;
</script>

<svelte:head>
	<title>Getting Started - Fellowship</title>
	<meta name="description" content="Install and configure Fellowship for Claude Code." />
</svelte:head>

<article class="getting-started container">

	<header class="page-header">
		<h1>Getting Started</h1>
		<p class="subtitle">Install Fellowship and run your first quest in minutes.</p>
	</header>

	<!-- Install -->
	<section class="section" id="install">
		<h2>Install</h2>
		<p>Add the plugin marketplace, then install Fellowship:</p>

		<div class="code-block">
			<pre><code>{installCmd1}</code></pre>
			<CopyButton text={installCmd1} />
		</div>

		<div class="code-block">
			<pre><code>{installCmd2}</code></pre>
			<CopyButton text={installCmd2} />
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Dependencies -->
	<section class="section" id="dependencies">
		<h2>Dependencies</h2>
		<p>These plugins are optional but recommended. Fellowship skill prompts reference them by name.</p>

		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Plugin</th>
						<th>Skills Used</th>
						<th>Install Commands</th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td><strong>superpowers</strong></td>
						<td>
							<code>using-git-worktrees</code>,
							<code>test-driven-development</code>,
							<code>verification-before-completion</code>,
							<code>finishing-a-development-branch</code>
						</td>
						<td class="cmd-cell">
							<div class="code-block compact">
								<pre><code>{superpowersCmd1}</code></pre>
								<CopyButton text={superpowersCmd1} />
							</div>
							<div class="code-block compact">
								<pre><code>{superpowersCmd2}</code></pre>
								<CopyButton text={superpowersCmd2} />
							</div>
						</td>
					</tr>
					<tr>
						<td><strong>pr-review-toolkit</strong></td>
						<td><code>review-pr</code></td>
						<td class="cmd-cell">
							<div class="code-block compact">
								<pre><code>{prReviewCmd}</code></pre>
								<CopyButton text={prReviewCmd} />
							</div>
						</td>
					</tr>
				</tbody>
			</table>
		</div>

		<p class="note">
			These are referenced by name in skill prompts. If a dependency isn't installed,
			Claude will skip that step rather than fail -- but you lose the discipline that step provides.
		</p>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- System Dependencies -->
	<section class="section" id="system-dependencies">
		<h2>System Dependencies</h2>
		<dl>
			<dt>Go CLI binary</dt>
			<dd>
				Gate enforcement hooks use a Go binary automatically downloaded from GitHub releases
				on first use. No manual installation needed.
			</dd>
		</dl>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Project Setup -->
	<section class="section" id="project-setup">
		<h2>Project Setup</h2>
		<p>
			Optionally add a <code>SessionStart</code> hook to detect checkpoints from previous sessions.
			Add this to <code>.claude/settings.local.json</code> in your project:
		</p>

		<div class="code-block">
			<pre><code>{hookConfig}</code></pre>
			<CopyButton text={hookConfig} />
		</div>

		<p>Also add <code>.fellowship/</code> to your <code>.gitignore</code> so checkpoint files are not committed. If you have configured a custom <code>dataDir</code> in <code>~/.claude/fellowship.json</code>, use that directory name instead.</p>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Your First Quest -->
	<section class="section" id="your-first-quest">
		<h2>Your First Quest</h2>
		<p>Here is what a typical quest looks like end to end:</p>

		<ol class="quest-steps">
			<li>
				<strong>Open Claude Code</strong> in your project directory.
			</li>
			<li>
				<strong>Run your quest:</strong>
				<div class="code-block compact">
					<pre><code>{questCmd}</code></pre>
					<CopyButton text={questCmd} />
				</div>
			</li>
			<li>
				<span class="phase-tag">Phase 0 &middot; Onboard</span>
				Creates a git worktree and loads project context via <code>/council</code>.
			</li>
			<li>
				<span class="phase-tag">Phase 1 &middot; Research</span>
				Explores the codebase, gathers conventions and relevant prior art.
			</li>
			<li>
				<span class="phase-tag">Phase 2 &middot; Plan</span>
				Presents a plan for your approval -- review it carefully.
			</li>
			<li>
				<span class="phase-tag">Phase 3 &middot; Implement</span>
				TDD implementation: red-green-refactor.
			</li>
			<li>
				<span class="phase-tag">Phase 4 &middot; Review</span>
				Convention check and verification pass.
			</li>
			<li>
				<span class="phase-tag">Phase 5 &middot; Complete</span>
				Creates a PR and cleans up the worktree.
			</li>
		</ol>

		<p>
			See <a href="{base}/how-it-works">How It Works</a> for a deeper explanation of each phase,
			or <a href="{base}/configuration">Configuration</a> to customize behavior.
		</p>
	</section>

</article>

<style>
	.getting-started {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-2xl);
	}

	.page-header {
		margin-bottom: var(--space-xl);
	}

	.page-header h1 {
		margin-bottom: var(--space-sm);
	}

	.subtitle {
		font-size: 1.25rem;
		color: var(--color-text-secondary);
	}

	.section {
		margin-bottom: var(--space-lg);
	}

	.section h2 {
		margin-bottom: var(--space-md);
	}

	.section p {
		margin-bottom: var(--space-md);
	}

	/* Code blocks with copy button */
	.code-block {
		position: relative;
		display: flex;
		align-items: flex-start;
		gap: var(--space-sm);
		margin-bottom: var(--space-md);
	}

	.code-block pre {
		flex: 1;
		margin: 0;
	}

	.code-block.compact {
		margin-bottom: var(--space-sm);
	}

	.code-block.compact pre {
		font-size: 0.875rem;
		padding: var(--space-sm);
	}

	/* Table */
	.table-wrap {
		overflow-x: auto;
		margin-bottom: var(--space-md);
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.95rem;
	}

	th, td {
		padding: var(--space-sm) var(--space-md);
		border: 1px solid var(--color-border);
		text-align: left;
		vertical-align: top;
	}

	th {
		background: var(--color-bg-card);
		font-family: var(--font-heading);
		font-size: 0.9rem;
		letter-spacing: 0.03em;
		color: var(--color-heading);
	}

	td code {
		font-size: 0.8em;
		white-space: nowrap;
	}

	.cmd-cell {
		min-width: 340px;
	}

	.cmd-cell .code-block:last-child {
		margin-bottom: 0;
	}

	/* Note callout */
	.note {
		font-size: 0.95rem;
		color: var(--color-text-secondary);
		border-left: 3px solid var(--color-accent);
		padding-left: var(--space-md);
		font-style: italic;
	}

	/* Definition list for system deps */
	dl {
		margin: 0;
	}

	dt {
		font-weight: 700;
		font-family: var(--font-heading);
		color: var(--color-heading);
		margin-bottom: var(--space-xs);
	}

	dd {
		margin-left: 0;
		color: var(--color-text-secondary);
	}

	/* Quest steps */
	.quest-steps {
		list-style: none;
		counter-reset: step;
		padding: 0;
		margin-bottom: var(--space-lg);
	}

	.quest-steps li {
		counter-increment: step;
		position: relative;
		padding-left: 3rem;
		margin-bottom: var(--space-md);
		line-height: 1.6;
	}

	.quest-steps li::before {
		content: counter(step);
		position: absolute;
		left: 0;
		top: 0;
		width: 2rem;
		height: 2rem;
		border: 2px solid var(--color-accent);
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-family: var(--font-heading);
		font-size: 0.85rem;
		font-weight: 700;
		color: var(--color-accent);
	}

	.phase-tag {
		display: inline-block;
		font-family: var(--font-heading);
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-accent);
		letter-spacing: 0.03em;
		margin-right: var(--space-xs);
	}

	@media (max-width: 768px) {
		.cmd-cell {
			min-width: auto;
		}

		.code-block.compact pre {
			font-size: 0.875rem;
		}
	}
</style>
