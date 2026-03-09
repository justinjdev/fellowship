<script lang="ts">
	import CopyButton from '$lib/components/CopyButton.svelte';

	const fullExample = `{
  "dataDir": ".fellowship",
  "branch": {
    "pattern": null,
    "author": null,
    "ticketPattern": "[A-Z]+-\\\\d+"
  },
  "worktree": {
    "enabled": true,
    "directory": null
  },
  "gates": {
    "autoApprove": []
  },
  "pr": {
    "draft": false,
    "template": null
  },
  "palantir": {
    "enabled": true,
    "minQuests": 2
  }
}`;

	const example1 = `{
  "branch": {
    "pattern": "{author}/{ticket}/{slug}",
    "author": "justin"
  },
  "gates": {
    "autoApprove": ["Research"]
  }
}`;

	const example2 = `{
  "pr": {
    "draft": true,
    "template": "## Summary\\n{summary}\\n\\n## Changes\\n{changes}\\n\\n## Task\\n{task}"
  },
  "gates": {
    "autoApprove": ["Research", "Plan"]
  }
}`;

	const example3 = `{
  "worktree": {
    "enabled": false
  },
  "gates": {
    "autoApprove": ["Research", "Plan", "Implement", "Review"]
  },
  "palantir": {
    "enabled": false
  }
}`;

	const projectExample = `{
  "branch": {
    "pattern": "feat/{ticket}-{slug}"
  },
  "gates": {
    "autoApprove": []
  },
  "pr": {
    "draft": true
  }
}`;

	const settings = [
		{
			key: 'dataDir',
			default_val: '".fellowship"',
			desc: 'Directory name for fellowship working files (state, checkpoints, errands, tome). Created inside each worktree and the main repo root.'
		},
		{
			key: 'branch.pattern',
			default_val: 'null',
			desc: 'Branch name template with placeholders: {slug}, {ticket}, {author}. When null, defaults to "fellowship/{slug}".'
		},
		{
			key: 'branch.author',
			default_val: 'null',
			desc: 'Static value for the {author} placeholder. If not set and pattern uses {author}, you\'ll be prompted.'
		},
		{
			key: 'branch.ticketPattern',
			default_val: '"[A-Z]+-\\\\d+"',
			desc: 'Regex to extract ticket IDs from quest descriptions. Default matches Jira-style IDs (e.g., PROJ-123).'
		},
		{
			key: 'worktree.enabled',
			default_val: 'true',
			desc: 'Whether quests create isolated worktrees. Set to false to work on the current branch.'
		},
		{
			key: 'worktree.directory',
			default_val: 'null',
			desc: 'Parent directory for worktrees. null uses Claude Code\'s default (.claude/worktrees/).'
		},
		{
			key: 'gates.autoApprove',
			default_val: '[]',
			desc: 'Gate names to auto-approve: "Research", "Plan", "Implement", "Review", "Complete". Gates not listed still surface to you.'
		},
		{
			key: 'pr.draft',
			default_val: 'false',
			desc: 'Create PRs as drafts.'
		},
		{
			key: 'pr.template',
			default_val: 'null',
			desc: 'PR body template string. Supports {task}, {summary}, and {changes} placeholders.'
		},
		{
			key: 'palantir.enabled',
			default_val: 'true',
			desc: 'Whether to spawn a palantir monitoring agent during fellowships.'
		},
		{
			key: 'palantir.minQuests',
			default_val: '2',
			desc: 'Minimum active quests before palantir is spawned.'
		}
	];

	const examples = [
		{
			title: 'Auto-approve research, custom branch pattern',
			code: example1,
			explanation: 'Auto-approves the Research gate so quests flow from Onboard straight through Research into Plan without pausing. Branch names include your name and Jira ticket.'
		},
		{
			title: 'Team workflow with draft PRs',
			code: example2,
			explanation: 'Creates draft PRs with a custom template. Auto-approves Research and Plan gates for faster iteration \u2014 you still review at Implement, Review, and Complete.'
		},
		{
			title: 'No worktrees, minimal oversight',
			code: example3,
			explanation: 'Works on the current branch without worktree isolation. Auto-approves everything except Complete (PR creation). No palantir monitoring. Use for trusted, low-risk tasks.'
		}
	];
</script>

<svelte:head>
	<title>Configuration | Fellowship</title>
	<meta name="description" content="Configure Fellowship with ~/.claude/fellowship.json. All settings are optional with sensible defaults." />
</svelte:head>

<div class="container page">
	<h1>Configuration</h1>

	<section class="section">
		<h2>Overview</h2>
		<p class="intro">
			Fellowship reads configuration from two files and merges them at startup.
			<code>~/.claude/fellowship.json</code> is your personal cross-project config.
			<code>.fellowship/config.json</code> at the repo root is a per-project config checked into source control.
			Settings are merged in order: built-in defaults &rarr; project config &rarr; user config.
			User config always wins. All settings are optional; missing keys use sensible defaults.
		</p>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<section class="section">
		<h2>Project Config</h2>
		<p class="intro">
			Place a <code>.fellowship/config.json</code> file at the root of your repository to share team-wide
			defaults. Useful for enforcing a consistent branch naming convention or gate policy across everyone
			who works on the project. Individual team members can still override any setting in their personal
			<code>~/.claude/fellowship.json</code>.
		</p>
		<div class="code-block" style="margin-top: var(--space-md);">
			<div class="code-block-header">
				<span class="code-block-label">.fellowship/config.json</span>
				<CopyButton text={projectExample} />
			</div>
			<pre><code>{projectExample}</code></pre>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<section class="section">
		<h2>Complete Configuration</h2>
		<div class="code-block">
			<div class="code-block-header">
				<span class="code-block-label">~/.claude/fellowship.json</span>
				<CopyButton text={fullExample} />
			</div>
			<pre><code>{fullExample}</code></pre>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<section class="section">
		<h2>Settings Reference</h2>
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th scope="col">Setting</th>
						<th scope="col">Default</th>
						<th scope="col">Description</th>
					</tr>
				</thead>
				<tbody>
					{#each settings as setting, i (setting.key)}
						<tr class:alt-row={i % 2 === 1}>
							<td><code>{setting.key}</code></td>
							<td><code>{setting.default_val}</code></td>
							<td>{setting.desc}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<section class="section">
		<h2>Common Setups</h2>
		<div class="examples">
			{#each examples as example, i (example.title)}
				<div class="example-card">
					<h3>{i + 1}. {example.title}</h3>
					<div class="code-block">
						<div class="code-block-header">
							<span class="code-block-label">fellowship.json</span>
							<CopyButton text={example.code} />
						</div>
						<pre><code>{example.code}</code></pre>
					</div>
					<p class="example-explanation">{example.explanation}</p>
				</div>
			{/each}
		</div>
	</section>
</div>

<style>
	.page {
		padding-top: var(--space-2xl);
		padding-bottom: var(--space-2xl);
	}

	h1 {
		margin-bottom: var(--space-md);
	}

	h2 {
		margin-bottom: var(--space-md);
	}

	.section {
		margin-bottom: var(--space-lg);
	}

	.intro {
		font-size: 1.15rem;
		color: var(--color-text-secondary);
		max-width: 52em;
		line-height: 1.7;
	}

	.intro code {
		background: var(--color-code-bg);
		padding: 0.15em 0.4em;
		border-radius: 4px;
		font-family: var(--font-code);
		font-size: 0.9em;
	}

	/* Code blocks */
	.code-block {
		background: var(--color-code-bg);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		overflow: hidden;
	}

	.code-block-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-xs) var(--space-sm);
		border-bottom: 1px solid var(--color-border);
		background: var(--color-bg-card);
	}

	.code-block-label {
		font-family: var(--font-code);
		font-size: 0.8rem;
		color: var(--color-text-secondary);
	}

	.code-block pre {
		margin: 0;
		padding: var(--space-md);
		overflow-x: auto;
	}

	.code-block code {
		font-family: var(--font-code);
		font-size: 0.88rem;
		line-height: 1.6;
		color: var(--color-text);
	}

	/* Reference table */
	.table-wrap {
		overflow-x: auto;
		border: 1px solid var(--color-border);
		border-radius: 8px;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.95rem;
	}

	thead {
		background: var(--color-bg-card);
	}

	th {
		text-align: left;
		padding: var(--space-sm) var(--space-md);
		font-family: var(--font-heading);
		font-weight: 600;
		color: var(--color-heading);
		border-bottom: 2px solid var(--color-border);
		white-space: nowrap;
	}

	td {
		padding: var(--space-sm) var(--space-md);
		border-bottom: 1px solid var(--color-border);
		vertical-align: top;
		line-height: 1.6;
	}

	tr:last-child td {
		border-bottom: none;
	}

	.alt-row {
		background: var(--color-bg-card);
	}

	td code {
		background: var(--color-code-bg);
		padding: 0.1em 0.35em;
		border-radius: 3px;
		font-family: var(--font-code);
		font-size: 0.85em;
		white-space: nowrap;
	}

	/* Example cards */
	.examples {
		display: flex;
		flex-direction: column;
		gap: var(--space-lg);
	}

	.example-card {
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: var(--space-lg);
		transition: border-color 0.2s ease;
	}

	.example-card:hover {
		border-color: var(--color-accent);
	}

	.example-card h3 {
		font-family: var(--font-heading);
		color: var(--color-heading);
		margin-bottom: var(--space-md);
		font-size: 1.1rem;
	}

	.example-card .code-block {
		margin-bottom: var(--space-md);
	}

	.example-explanation {
		color: var(--color-text-secondary);
		line-height: 1.7;
		max-width: 60em;
	}
</style>
