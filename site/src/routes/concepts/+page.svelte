<script>
	import { base } from '$app/paths';
</script>

<svelte:head>
	<title>Agentic Workflows & Orchestration - Fellowship</title>
	<meta name="description" content="Understanding how AI agents work together to tackle complex tasks, and how Fellowship puts these concepts into practice." />
</svelte:head>

<article class="concepts container">

	<header class="page-header">
		<h1>Agentic Workflows & Orchestration</h1>
		<p class="subtitle">Understanding how AI agents work together to tackle complex tasks</p>
	</header>

	<!-- What Are Agentic Workflows? -->
	<section class="section" id="agentic-workflows">
		<h2>What Are Agentic Workflows?</h2>
		<p class="prose">
			When you use AI for coding today, you're typically the orchestrator. You decide what to do, ask the AI to do it, review the output, then figure out the next step yourself. Whether that's one prompt or ten chained together, <strong>you're driving</strong> &mdash; deciding what to do and when, managing each step manually.
		</p>
		<p class="prose">
			In an <strong>agentic workflow</strong>, the AI drives. Given a goal like "add a caching layer," the agent formulates its own plan, uses tools to read files and run commands, checks its work, adjusts course, and loops until the job is done. The shift isn't just automation &mdash; it's a change in who's orchestrating. You describe <em>what</em> you want; the agent figures out <em>how</em>.
		</p>

		<div class="fellowship-callout">
			<span class="callout-label">In Fellowship</span>
			<p>Every task runs as a <strong>quest</strong> &mdash; a multi-step workflow where Claude researches the codebase, creates a plan, implements with tests, and reviews its own work. Each stage has a clear purpose and a checkpoint before the next one begins.</p>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Why Structure Matters -->
	<section class="section" id="structure">
		<h2>Why Structure Matters</h2>
		<p class="prose">
			Giving an AI agent free rein sounds appealing, but in practice it leads to problems. Without structure, agents tend to drift off scope, skip verification steps, and lose track of what they've already done as the conversation grows. The longer an unstructured session runs, the more likely it is to produce sloppy or incomplete work.
		</p>
		<p class="prose">
			The core issue is <strong>context window degradation</strong>. As a conversation fills up, the AI's ability to reason about earlier content declines. Important instructions from the beginning get buried under thousands of lines of tool output. Structure &mdash; phases, checkpoints, compression &mdash; counteracts this by keeping the agent focused and the context clean.
		</p>

		<div class="fellowship-callout">
			<span class="callout-label">In Fellowship</span>
			<p>Structure is enforced via <strong>gates</strong> &mdash; hard checkpoints between phases that require approval before the agent can proceed. This isn't just a prompt telling the agent to pause; tools are actually blocked at the plugin level until the gate is approved.</p>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Agent Orchestration -->
	<section class="section" id="orchestration">
		<h2>Agent Orchestration</h2>
		<p class="prose">
			When a project has multiple independent tasks, you could run them one at a time. But that's slow. <strong>Orchestration</strong> is the pattern of having a coordinator agent that manages multiple worker agents in parallel. The coordinator doesn't do the hands-on work itself &mdash; it delegates tasks, tracks progress, and routes information between agents.
		</p>
		<p class="prose">
			Think of it like a project manager working with a team of developers. The PM breaks down the project, assigns tasks, checks in on progress, and handles blockers. The developers focus on their individual tasks without needing to coordinate with each other directly.
		</p>

		<div class="fellowship-callout">
			<span class="callout-label">In Fellowship</span>
			<p>The coordinator is called <strong>Gandalf</strong>. It analyzes your task list, spawns quest runners for code tasks and scouts for research questions, routes gate approvals, and tracks progress across all agents &mdash; but never writes code itself.</p>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Isolation & Parallel Execution -->
	<section class="section" id="isolation">
		<h2>Isolation & Parallel Execution</h2>
		<p class="prose">
			Running multiple agents in parallel only works if they can't step on each other's work. If two agents edit the same file at the same time, you get conflicts and corruption. <strong>Isolation</strong> means giving each agent its own workspace so they can operate independently.
		</p>
		<p class="prose">
			Common approaches include separate git branches (but those still share a working directory), containers or VMs (heavyweight), or separate directory copies. The key property is that one agent's file writes never interfere with another's.
		</p>

		<div class="fellowship-callout">
			<span class="callout-label">In Fellowship</span>
			<p>Each quest runs in its own <strong>git worktree</strong> &mdash; a full working copy of the repo on its own branch. Multiple quests can modify code simultaneously without merge conflicts, and each produces a clean PR when it's done.</p>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Context Engineering -->
	<section class="section" id="context-engineering">
		<h2>Context Engineering</h2>
		<p class="prose">
			AI models have a fixed context window &mdash; a limit on how much text they can consider at once. As that window fills with conversation history, tool outputs, and file contents, the model's reasoning quality degrades. It starts missing instructions, repeating itself, and making mistakes it wouldn't make with a fresh context.
		</p>
		<p class="prose">
			<strong>Context engineering</strong> is the practice of managing what goes into the context window and when. Smart workflows compress intermediate results, discard noise (like verbose command output), and carry forward only the essential information between stages. This keeps the AI operating in its "smart zone" throughout a long task.
		</p>

		<div class="fellowship-callout">
			<span class="callout-label">In Fellowship</span>
			<p>Between every phase, Fellowship compresses the conversation into a structured <strong>checkpoint</strong>. This keeps context utilization low and also provides crash recovery &mdash; if a session dies mid-task, the checkpoint survives on disk and the next session can resume from where it left off.</p>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Human in the Loop -->
	<section class="section" id="human-in-the-loop">
		<h2>Human in the Loop</h2>
		<p class="prose">
			There's a spectrum of autonomy for AI agents. On one end, fully supervised: the human approves every action. On the other, fully autonomous: the agent runs to completion without any input. Both extremes have problems &mdash; full supervision is exhausting, and full autonomy is risky.
		</p>
		<p class="prose">
			The sweet spot is <strong>human-on-the-loop</strong>. The agent works autonomously through routine steps but surfaces key decisions for human review. You stay in control of the important choices without micromanaging every file edit. The human sets the direction and reviews the plan; the agent handles the execution.
		</p>

		<div class="fellowship-callout">
			<span class="callout-label">In Fellowship</span>
			<p>All gates surface to you for approval by default. You can <strong>auto-approve</strong> specific gates (like Research and Plan) via configuration, so you only review the high-stakes transitions &mdash; like the jump from planning to writing code, or from implementation to creating a PR.</p>
		</div>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- Getting Started CTA -->
	<section class="section cta-section" id="get-started">
		<h2>Put It Into Practice</h2>
		<p class="prose">
			These aren't abstract ideas &mdash; they're the mechanics behind every Fellowship quest. If you want to see them in action, get started with your first task.
		</p>
		<div class="cta-links">
			<a href="{base}/getting-started" class="cta-link primary">
				<span class="cta-text">Getting Started</span>
				<span class="cta-desc">Install and configure Fellowship</span>
			</a>
			<a href="{base}/how-it-works" class="cta-link">
				<span class="cta-text">How It Works</span>
				<span class="cta-desc">See the full quest lifecycle in detail</span>
			</a>
		</div>
	</section>

</article>

<style>
	.concepts {
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
		font-size: 1.2rem;
		color: var(--color-text-secondary);
		max-width: 36rem;
		line-height: 1.6;
	}

	.section {
		margin-bottom: var(--space-lg);
	}

	.section h2 {
		margin-bottom: var(--space-md);
	}

	.prose {
		font-size: 1.05rem;
		color: var(--color-text);
		line-height: 1.8;
		max-width: 42rem;
		margin-bottom: var(--space-md);
	}

	.prose strong {
		color: var(--color-heading);
	}

	.prose em {
		font-style: italic;
	}

	/* Fellowship callout cards */
	.fellowship-callout {
		border-left: 3px solid var(--color-accent);
		background: var(--color-bg-card);
		padding: var(--space-md) var(--space-lg);
		border-radius: 0 8px 8px 0;
		margin: var(--space-lg) 0;
		max-width: 42rem;
	}

	.callout-label {
		display: block;
		font-family: var(--font-heading);
		font-size: 0.85rem;
		letter-spacing: 0.06em;
		color: var(--color-accent);
		margin-bottom: var(--space-sm);
	}

	.fellowship-callout p {
		color: var(--color-text-secondary);
		line-height: 1.7;
		font-size: 1rem;
		margin: 0;
	}

	.fellowship-callout p strong {
		color: var(--color-text);
	}

	/* CTA section */
	.cta-links {
		display: flex;
		gap: var(--space-md);
		margin-top: var(--space-lg);
	}

	.cta-link {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
		padding: var(--space-md) var(--space-lg);
		background: var(--color-bg-card);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		text-decoration: none;
		transition: border-color 0.2s ease;
		min-width: 14rem;
	}

	.cta-link:hover {
		border-color: var(--color-accent);
	}

	.cta-link.primary {
		border-color: var(--color-accent);
		background: var(--color-bg-elevated);
	}

	.cta-text {
		font-family: var(--font-heading);
		font-size: 1.1rem;
		color: var(--color-heading);
	}

	.cta-desc {
		font-size: 0.9rem;
		color: var(--color-text-secondary);
	}

	/* Responsive */
	@media (max-width: 768px) {
		.cta-links {
			flex-direction: column;
		}

		.cta-link {
			min-width: unset;
		}

		.fellowship-callout {
			padding: var(--space-sm) var(--space-md);
		}
	}
</style>
