<script lang="ts">
	import { base } from '$app/paths';
</script>

<svelte:head>
	<title>Changelog | Fellowship</title>
	<meta name="description" content="Fellowship release history and version changelog." />
</svelte:head>

<div class="container page">
	<h1>Changelog</h1>

	<!-- Unreleased -->
	<section class="version" id="unreleased">
		<h2 class="version-heading"><a href="{base}/changelog#unreleased">Unreleased</a></h2>
		<ul class="changes">
			<li>
				<strong>Teammates run the full lifecycle</strong> &mdash; a fellowship teammate could take <code>/quest</code>'s escape hatch for a small task, skip <code>fellowship init</code>, and finish with no gate ever submitted; with no state row the hooks had nothing to enforce. The escape hatch is now standalone-only, the spawn prompt says so, and until <code>fellowship init</code> has run in a registered worktree <code>gate-guard</code> allows only what bootstrap needs: Edit/Write into the data directory (a plan-driven quest copies its plan there), the fellowship CLI minus the lead's <code>state</code> commands, read-only git, and read-only shell builtins with no redirection &mdash; a source write, a heredoc, a <code>git commit</code> all wait for <code>init</code>. A <code>[COMPLETE]</code> with no gate history is a violation, not a finished quest.
			</li>
			<li>
				<strong>Hooks allow outside a git repository</strong> &mdash; a session started in a directory that is not inside any repo had every Bash, Edit and Write call blocked by <code>gate-guard</code> &ldquo;for safety&rdquo;: the store lookup surfaced git's &ldquo;not a git repository&rdquo; as an unreadable store. Outside a repo there is nothing to enforce, so it now reads as &ldquo;no fellowship here&rdquo; and hooks allow; <code>fellowship</code> commands report &ldquo;no fellowship state&rdquo; instead of a git error.
			</li>
			<li>
				<strong>Ported to the implicit-team API</strong> &mdash; Claude Code removed <code>TaskCreate</code>, <code>TaskUpdate</code>, <code>TaskGet</code>, <code>TaskList</code>, <code>TeamCreate</code>, <code>TeamDelete</code>, and the <code>shutdown_request</code> protocol. The plugin now spawns teammates as named background agents (<code>Agent(name: ...)</code>), addresses them with <code>SendMessage(to: &lt;name&gt;)</code> (which resumes an idle teammate with its context intact), and stops them with <code>TaskStop</code>. Quest identity is the quest name, and the store (<code>state add-quest</code>/<code>update-quest</code>/<code>show --json</code>) is the only coordination state &mdash; <code>--task-id</code> flags are gone (the <code>task_id</code>/<code>team_name</code> columns stay in the schema, unused, so no migration is needed). The metadata prerequisite is now <code>fellowship phase confirm --dir &lt;worktree&gt; --phase &lt;phase&gt;</code> (same validation: a valid phase equal to the quest's current phase), and completion is <code>fellowship complete --dir &lt;worktree&gt;</code> (allowed only in Review with no gate pending, enforced by the command and by gate-guard's refusal of the Bash form) &mdash; each replacing a TaskUpdate hook (<code>metadata-track</code>, <code>completion-guard</code>) that no longer had anything to fire on. Teammates end their turn after a <code>[GATE]</code> message and are resumed by the lead's next <code>SendMessage</code>; a <code>[COMPLETE]</code> envelope carries the PR URL; scouts report with <code>[REPORT]</code>; palantir dropped <code>TaskGet</code>/<code>TaskList</code>. Identity under the implicit team: a hook payload is the lead only when its session id matches the recorded lead's <em>and</em> it carries no <code>agent_id</code> (a background agent shares the lead's session id, so an <code>agent_id</code> in the payload is what marks it as a subagent, never the lead); <code>fellowship init</code> now records a session id against a quest only when it differs from the recorded lead's, and <code>init --phase</code>/<code>--plan-skip</code> on an existing quest additionally requires the process to be standing in the main working tree. Also: unknown <code>gates.autoApprove</code> names are now ignored with a warning instead of failing <code>fellowship init</code>, and the out-of-date-store block message now says the upgrade command must be run alone. In-process teammates keep the lead's working directory for their whole life &mdash; a bare <code>cd</code> does not persist between their Bash calls &mdash; so hooks resolve a subagent's quest from the <code>--dir</code> or file path its tool call names, backed by an <code>agent_quests</code> mapping (schema migration 6) that a new PostToolUse <code>Bash</code> hook, <code>agent-track</code>, records after the teammate's first <code>fellowship init --dir &lt;worktree&gt;</code>. Teammates now address everything by <code>--dir &lt;worktree&gt;</code> and absolute paths rather than relying on their cwd; <code>fellowship complete --dir &lt;X&gt;</code> is judged against the quest that <code>--dir</code> names, and <code>gate-submit</code> returns the whole <code>tool_input</code> with only <code>message</code> rewritten.
			</li>
			<li>
				<strong>CLI subcommand nouns renamed to plain words</strong> &mdash; the CLI's Tolkien-flavored subcommand nouns are now plain English, matching their Go packages: <code>herald</code> &rarr; <code>events</code>, <code>tome</code> &rarr; <code>history</code>, <code>errand</code> &rarr; <code>todo</code>, <code>eagles</code> &rarr; <code>health</code>, <code>bulletin</code> &rarr; <code>notes</code>, <code>autopsy</code> &rarr; <code>failures</code>, <code>company</code> &rarr; <code>group</code> (and <code>state add-company</code> &rarr; <code>state add-group</code>). Skill and agent names are unaffected &mdash; this only touches the seven reporting/side-channel subcommand nouns above. Each old name still works for one release: running it prints one deprecation line to stderr, then runs the renamed command. The <code>failures.expiryDays</code> config key replaces <code>autopsy.expiryDays</code> (no alias &mdash; update <code>~/.claude/fellowship.json</code> and any project <code>.fellowship/config.json</code>). SQLite table and column names are unchanged (no schema migration).
			</li>
			<li>
				<strong>One health classifier, reachable everywhere</strong> &mdash; <code>fellowship health</code> and <code>fellowship events --problems</code> were two separate Go implementations of the same stalled/zombie classification, and palantir carried a third copy in prose driven by unbounded <code>git diff</code>/<code>git status</code> over each worktree. <code>events.DetectProblems</code> now delegates to health's sweep (which gained a <code>struggling</code> classification &mdash; repeated gate rejections in a quest's current phase, independent of its <code>health</code>), and <code>health.WriteReport</code> and the <code>.fellowship/health-report.json</code> file it wrote (nothing read it) are gone. palantir is now a thin reporter over that one sweep: it runs <code>fellowship health --json</code> and <code>fellowship state show --json</code> instead of reconstructing stuck/stalled from task metadata, and reads scope-drift/file-conflict signals from each quest's history (<code>history show --json</code>'s <code>files_touched</code>) instead of diffing worktrees itself. Below <code>palantir.minQuests</code> or with <code>palantir.enabled: false</code>, Gandalf runs the same sweep itself after every gate transition and spawn, so health monitoring never depends on an extra agent. <code>/rekindle</code> and <code>/retro</code> read quest state and phase/health the same way instead of shelling <code>gate status --dir &lt;worktree&gt;</code> per quest. <code>state show</code> (always JSON, but had no flag to name that) and <code>group show &lt;name&gt;</code> (table-only before) both accept <code>--json</code> now.
			</li>
			<li>
				<strong>Four phases, three gates</strong> &mdash; The quest lifecycle is now <strong>Research &rarr; Plan &rarr; Implement &rarr; Review</strong>. Onboard's work (worktree provisioning, context loading, the checkpoint resume check) is the first step of Research; the adversarial balrog pass is the first step of Review and PR creation the last. A gate leaves Research, Plan, and Implement; nothing leaves Review, so the quest ends inside it when the PR is open and the task is marked complete &mdash; which <code>completion-guard</code> now allows only in Review with no gate pending. Valid <code>gates.autoApprove</code> entries are the three gate-bearing phases. A schema migration rewrites stored phase names (live state, phase and gate history, and each quest's <code>autoApprove</code> list) in existing stores, and the pre-2.0 JSON importer runs through the same table, so an in-flight quest keeps advancing across the upgrade.
			</li>
			<li>
				<strong><code>/quest</code> and <code>/fellowship</code> are half the size</strong> &mdash; <code>quest/SKILL.md</code> went from ~25 KB to ~15 KB and <code>fellowship/SKILL.md</code> from ~21 KB to ~15.5 KB. Quest inlines the orientation <code>/council</code> did and the pattern extraction <code>/gather-lore</code> did rather than invoking them, so a quest no longer hands its phase vocabulary to two satellite skills that then have to track it; both remain as skills you invoke yourself. Fellowship's isolation pre-flight and provisioning protocol moved to <code>resources/isolation.md</code>, and Gandalf's voice to <code>resources/lead-behavior.md</code>.
			</li>
			<li>
				<strong><code>/lembas</code> stops asking for <code>/compact</code></strong> &mdash; It ended by telling the user to run a command Claude cannot run, so the step was either ignored or handed over as a chore. It now writes the checkpoint and continues from that summary, which is what the checkpoint was always for.
			</li>
			<li>
				<strong>One checkpoint reader per context</strong> &mdash; Four things looked for a <code>/lembas</code> checkpoint and disagreed about who resumes. Now quest's Research step 0 is the only checkpoint check inside a quest, <code>/rekindle</code> is the recovery path outside one, and the README's <code>SessionStart</code> hook only prints a hint. <code>/council</code> no longer looks for one at all.
			</li>
			<li>
				<strong><code>/rekindle</code> shares the spawn template</strong> &mdash; It carried a hand-copied quest spawn prompt with an undefined <code>{'{gate_config_override}'}</code> placeholder. <code>spawn-prompts.md</code> gained a RESUME variant and rekindle references it, so gate, hold, isolation, and boundary language has one home.
			</li>
			<li>
				<strong>Quest templates ship with one</strong> &mdash; Templates were a feature with nothing in it. <code>/lorebook</code> now resolves a built-in directory after project and user, and fellowship ships <code>example</code> &mdash; a worked template at the specificity the docs ask for, with no keywords so it never auto-suggests. <code>/lorebook</code> and <code>/scribe</code> both cover all four phases, and Review's section is the last guidance a quest loads.
			</li>
			<li>
				<strong><code>/dashboard</code> command</strong> — Starts the fellowship web dashboard in the background and prints its URL. The dashboard's group gate approval now shares <code>group.BatchApprove</code> with the CLI's <code>fellowship group approve</code> instead of a second, drifted copy that skipped history recording. The core fellowship state model (<code>FellowshipState</code>, <code>QuestEntry</code>, <code>GroupEntry</code>, and their SQLite CRUD) moved out of the <code>dashboard</code> package into a new <code>cli/internal/fellowship</code> package, removing the import cycle that forced <code>group</code> to duplicate that batch-approve logic. The dashboard's <code>/api/status</code> response now includes a <code>phases</code> field so the UI's phase list tracks the server instead of a hardcoded array (which was previously missing the Adversarial phase).
			</li>
			<li>
				<strong>The lead is no longer locked out of the main tree</strong> &mdash; <code>worktree-guard</code> blocked every <code>Edit</code>/<code>Write</code> in the main working tree while a fellowship was active, including the lead's own. <code>fellowship state init</code> now records the lead's Claude Code session in a <code>lead</code> marker inside the data directory, and the guard allows that session, blocks a quest worktree that resolves to the main root, blocks a session known not to be the lead, and allows anything it cannot identify.
			</li>
			<li>
				<strong><code>dataDir</code> moves the store too</strong> &mdash; the fellowship database was always created in <code>.fellowship/</code> even when <code>dataDir</code> named a different directory, so the store and everything that reads it lived in different places. The store now follows the configured data directory.
			</li>
			<li>
				<strong><code>hold</code>/<code>unhold</code> report an unregistered <code>--dir</code></strong> &mdash; instead of guessing the quest from the directory's name, which could hold a different quest that happened to share it.
			</li>
			<li>
				<strong>One gate state machine</strong> &mdash; approve, reject, submit and reset are single functions used by <code>gate approve|reject</code>, group batch approval, the auto-approve path and the resets. Auto-approved gates now clear the gate id and record the approval and phase transition in the history and events log exactly as a lead approval does; a held quest can no longer submit a gate; and <code>fellowship init</code> and <code>state clean-worktrees</code> reset the lembas/metadata flags along with the gate flags.
			</li>
			<li>
				<strong>Fail-closed hook dispatch</strong> — Gate hooks (<code>gate-guard</code>, <code>gate-submit</code>, <code>gate-prereq</code>, <code>completion-guard</code>, <code>metadata-track</code>, <code>file-track</code>) now run through <code>plugin/hooks/scripts/fellowship.sh</code> instead of exec'ing the binary directly; if the binary is missing and can't be installed, they block (exit 2) instead of silently allowing the tool call through. <code>worktree-guard</code> keeps its fail-open backstop posture. The <code>file-track</code> hook is now wired into <code>hooks.json</code>, and <code>SessionStart</code> installs the binary on <code>clear</code> and <code>compact</code> in addition to <code>startup</code>/<code>resume</code>.
			</li>
			<li>
				<strong>Verified downloads</strong> — <code>ensure-binary.sh</code> verifies the downloaded tarball against the release's <code>checksums.txt</code> before installing, assembles the binary atomically, and holds a simple lock so concurrent sessions don't race the same install.
			</li>
			<li>
				<strong>CI</strong> — added <code>gofmt</code>, <code>go vet</code>, race-enabled tests, <code>shellcheck</code> on the hook scripts, a plugin manifest path check, and a site build job.
				<strong>Tightened skill triggers</strong> — <code>quest</code>, <code>council</code>, <code>gather-lore</code>, and <code>warden</code> descriptions now name their actual invocation scope instead of "any non-trivial task", reducing over-triggering.
			</li>
			<li>
				<strong>Removed orphaned <code>quest-runner</code> agent</strong> — never spawned (quest teammates use <code>general-purpose</code>); removed from the plugin manifest, README, and the site's Agents and How It Works pages.
			</li>
			<li>
				<strong>Documentation drift fixes</strong> — corrected <code>gates.autoApprove</code> valid values on the site config page, replaced the removed <code>using-git-worktrees</code> dependency with <code>writing-plans</code> (Plan phase), added the missing v1.6.1 changelog entry, fixed the quest phase/gate count, documented <code>failures.expiryDays</code> and added the missing <code>dataDir</code> row to <code>/settings</code>' schema table, corrected the <code>.fellowship/</code> gitignore wording in lembas, corrected palantir's Bash tool description, and fixed several command titles and skill/command wording.
			</li>
			<li>
				<strong>Archived the <code>gate-state-machine</code> OpenSpec change</strong> — superseded by the Go CLI + SQLite gate enforcement design (v1.5.1&ndash;v2.2.0); moved to <code>openspec/changes/archive/</code> with a SUPERSEDED note.
			</li>
			<li>
				<strong>Documented CLI invocations now work</strong> &mdash; <code>--dir &lt;path&gt;</code> is accepted by <code>gate status|approve|reject</code>, <code>state add-quest|add-scout|add-group|update-quest|show</code>, <code>todo init|list|add|update|show</code>, <code>failures create|scan|infer</code>, and <code>history show</code>, resolving the quest exactly as if the process were running in that directory. <code>gate</code> previously had no flag parsing at all, so every documented <code>--dir</code> call failed.
			</li>
			<li>
				<strong><code>fellowship init</code> name resolution</strong> &mdash; Without <code>--quest</code>, init now uses the quest name the lead registered with <code>state add-quest</code> for that worktree, falling back to the directory name only when the worktree is unregistered.
			</li>
			<li>
				<strong><code>fellowship init</code> reads <code>gates.autoApprove</code></strong> &mdash; Auto-approved gates come from the merged config (project <code>.fellowship/config.json</code>, then <code>~/.claude/fellowship.json</code>) instead of always being empty. Unknown phase names are rejected.
			</li>
			<li>
				<strong><code>fellowship status</code> honors the base branch</strong> &mdash; Merged-branch detection compares against the fellowship's stored <code>base_branch</code> instead of a hardcoded <code>main</code>.
			</li>
			<li>
				<strong><code>fellowship events post</code></strong> &mdash; Records a tiding from the CLI, so the palantir logs alerts without <code>jq</code> or a hand-written JSONL file. <code>events</code> gained <code>--quest</code> and <code>--limit</code>; <code>failures scan --all</code> returns every unexpired failure record.
			</li>
			<li>
				<strong>Prompt layer matches the binary</strong> &mdash; Skills, agents, and commands now call the CLI by its full path, use only flags that exist, and read state through the CLI instead of the pre-2.0 JSON files.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v2.2.0 -->
	<section class="version" id="v2-2-0">
		<h2 class="version-heading"><a href="{base}/changelog#v2-2-0">v2.2.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Model routing</strong> — Every subagent spawn point now routes to a cost-appropriate model: palantir defaults to <code>haiku</code>, scout and the validator to <code>sonnet</code>, and Explore scans to <code>haiku</code>, while quest teammates and balrog keep the session model. Override any role via the new <code>models.*</code> block in fellowship config.
			</li>
			<li>
				<strong>Validator agent</strong> — Scout's adversarial validation now runs in a dedicated read-only agent (Read/Glob/Grep only, enforced by tool restrictions) instead of an unrestricted general-purpose subagent.
			</li>
			<li>
				<strong>Mode-aware gate accounting</strong> — The lead verifies quest completion against the gates the quest's mode actually requires: 6 for standard and promoted quests (Adversarial included), 3 for plan-driven. Progress bars and phase enumerations now include the Adversarial phase everywhere.
			</li>
			<li>
				<strong>Spawn prompt consolidation</strong> — The three quest spawn prompt variants (standard, plan-driven, promoted) collapsed into one base template with per-variant deltas, eliminating ~250 lines of drift-prone duplication and unifying hold/shutdown language.
			</li>
			<li>
				<strong>Project config layer</strong> — Fellowship startup and quest onboard now merge <code>.fellowship/config.json</code> (project) with <code>~/.claude/fellowship.json</code> (user) as defaults &rarr; project &rarr; user, matching <code>/settings</code>.
			</li>
			<li>
				<strong>CLI phase fix</strong> — <code>fellowship init --phase Adversarial</code> was rejected and company progress ranked Adversarial-phase quests as zero; phase lists now derive from a single canonical order in the state package.
			</li>
			<li>
				<strong>Messaging protocol fixes</strong> — SendMessage recipients are teammate names (task-ID addressing never delivered); balrog and scout embed the full report envelope inline; balrog gained Write/Edit scoped strictly to test files ("report, don't repair").
			</li>
			<li>
				<strong>Docs refresh</strong> — README and site now document all 10 skills, 6 commands, and 5 agents; the skills page separates auto-invoked skills from user-invoked commands; <code>/validate-docs</code> gained a config-schema cross-check across settings, README, and the site.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v2.1.0 -->
	<section class="version" id="v2-1-0">
		<h2 class="version-heading"><a href="{base}/changelog#v2-1-0">v2.1.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Worktree isolation guard</strong> — A fail-closed hook blocks quest teammates from writing source into the main working tree when isolation is skipped. <code>fellowship state init</code> registers it in the git-ignored <code>.claude/settings.local.json</code> (no commits to your repo), and it arms only while a quest worktree is live, so it never blocks ordinary solo work.
			</li>
			<li>
				<strong>Lead cd-guard hardening</strong> — Gandalf is now blocked from <code>cd</code>-ing into quest worktrees created outside <code>.claude/worktrees/</code> (e.g. lead-provisioned worktrees), preventing the lead from inheriting a quest's gate or hold state.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v2.0.0 -->
	<section class="version" id="v2-0-0">
		<h2 class="version-heading"><a href="{base}/changelog#v2-0-0">v2.0.0</a></h2>
		<ul class="changes">
			<li>
				<strong>SQLite storage</strong> — All state (quests, gates, tome, errands, herald, bulletin, autopsy) migrated from JSON files to SQLite with WAL mode. Eliminates file locking issues and race conditions in parallel quests. Run <code>fellowship migrate</code> to upgrade existing data.
			</li>
			<li>
				<strong>Interactive <code>/guide</code></strong> — Rewrote the guide from a passive concept explainer to a learn-by-doing walkthrough. Walks beginners through a real quest (research &rarr; plan &rarr; implement &rarr; PR) on their own codebase, then introduces <code>/quest</code> and <code>/fellowship</code>.
			</li>
			<li>
				<strong>Concepts page</strong> — New documentation site page explaining agentic workflows, orchestration, isolation, context engineering, and human-in-the-loop &mdash; with "In Fellowship" callouts connecting each concept to the product.
			</li>
			<li>
				<strong>Quest autopsy</strong> — Failure memory that persists across sessions. When a quest fails, records what went wrong and why. Future quests in the same area can learn from past failures.
			</li>
			<li>
				<strong>Bulletin board</strong> — Cross-quest knowledge sharing. Quests post discoveries to a shared bulletin during Research and Implement. Sibling quests scan the bulletin at Research start.
			</li>
			<li>
				<strong>Gate enrichment</strong> — Gate submissions now include structured context (diff stats, test results, phase summary) so the lead can make informed approval decisions.
			</li>
			<li>
				<strong>WorktreeGuard</strong> — Blocks the lead session from accidentally <code>cd</code>-ing into quest worktrees. Runs before state file checks in the hook runner.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.9.2 -->
	<section class="version" id="v1-9-2">
		<h2 class="version-heading"><a href="{base}/changelog#v1-9-2">v1.9.2</a></h2>
		<ul class="changes">
			<li>
				<strong>Stale gate state fix</strong> — Gate guard hook no longer blocks Gandalf when a previous quest's gate state file is present in a fresh worktree. Prevents stale state from causing spurious tool blocks at session start.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.9.1 -->
	<section class="version" id="v1-9-1">
		<h2 class="version-heading"><a href="{base}/changelog#v1-9-1">v1.9.1</a></h2>
		<ul class="changes">
			<li>
				<strong>Fellowship startup fix</strong> — <code>ensure-binary.sh</code> now runs before any fellowship operations, removing the PATH dependency. The full binary path (<code>~/.claude/fellowship/bin/fellowship</code>) is used for all CLI calls.
			</li>
			<li>
				<strong><code>state init</code> overwrite warning</strong> — Instead of erroring when <code>fellowship-state.json</code> already exists, <code>fellowship state init</code> now warns and proceeds. Warning includes the existing fellowship name and quest count.
			</li>
			<li>
				<strong><code>validate-docs</code> marketplace check</strong> — Validates that the skill and agent counts in the marketplace description match the actual plugin.
			</li>
			<li>
				<strong>Deprecated commands removed</strong> — <code>fellowship install</code> and <code>fellowship uninstall</code> CLI subcommands removed (hooks are provided by the plugin).
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.9.0 -->
	<section class="version" id="v1-9-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-9-0">v1.9.0</a></h2>
		<ul class="changes">
			<li>
				<strong><code>/missive</code> skill</strong> — Fetches GitHub issue context for quest spawning. Pulls title, body, labels, and recent comments via <code>gh</code> CLI. Returns a structured package with issue context, a suggested branch name (incorporating the issue number), and PR closing keywords. Gandalf invokes it automatically when issue references (<code>#N</code>) are detected. Also usable standalone: <code>/missive 42</code>.
			</li>
			<li>
				<strong>Balrog agent</strong> — Adversarial validation agent that reviews code for structural quality: factoring, coupling, cohesion, abstraction levels, and information hiding. Challenges every design decision, not just obvious violations. Integrated into the review workflow.
			</li>
			<li>
				<strong>Per-project config</strong> — Committable project-level config at <code>.fellowship/config.json</code>. Three-way merge chain: defaults → project → user (user always wins). Team can share gate policies, branch patterns, and PR templates. <code>/settings</code> shows merged config with <code>[default]</code> / <code>[project]</code> / <code>[user]</code> provenance per field.
			</li>
			<li>
				<strong><code>issues.autoClose</code> config key</strong> — When true (default), <code>/missive</code> includes <code>Closes #N</code> in PR keywords so issues close automatically on merge.
			</li>
			<li>
				<strong>Base branch fixes</strong> — Worktrees now receive the correct base branch. Handles detached HEAD, dirty working tree warnings, and prompts when not on main.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.8.0 -->
	<section class="version" id="v1-8-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-8-0">v1.8.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Scout-to-quest promotion</strong> — Say <code>promote scout-X to a quest</code> during a fellowship. Gandalf reads the scout's findings file, spawns a quest pre-loaded with the research, and the quest enters validation mode (verify and supplement findings) instead of researching from scratch.
			</li>
			<li>
				<strong><code>/retro</code> skill</strong> — Post-fellowship retrospective. Analyzes gate history, palantir alerts, and quest metrics. Recommends configuration changes like auto-approving gates with zero rejection rates. Integrated into the fellowship disband flow.
			</li>
			<li>
				<strong>Plan-driven quests</strong> — Provide a pre-existing plan file and quests skip Research and Plan phases, jumping straight to Implement. Gandalf can fan out large plans into multiple parallel quests.
			</li>
			<li>
				<strong>Structured conflict resolution</strong> — Hold mechanism for quests with file conflicts. Gandalf detects overlapping file sets and holds conflicting quests until dependencies complete.
			</li>
			<li>
				<strong>Herald logging</strong> — Dashboard gate handlers and company batch approve now emit herald events for observability.
			</li>
			<li>
				<strong>Palantir alert persistence</strong> — Alerts persisted to JSONL log for post-fellowship analysis by <code>/retro</code>.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.7.5 -->
	<section class="version" id="v1-7-5">
		<h2 class="version-heading"><a href="{base}/changelog#v1-7-5">v1.7.5</a></h2>
		<ul class="changes">
			<li>
				<strong>Fix</strong> — Hook binary distribution fixes (v1.7.1–v1.7.5). Use binary directly in hooks, bootstrap via <code>SessionStart</code>, remove duplicate hook installation.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.7.0 -->
	<section class="version" id="v1-7-0">
		<h2 class="version-heading"><a href="{base}/changelog#v1-7-0">v1.7.0</a></h2>
		<ul class="changes">
			<li>
				<strong>Dashboard</strong> — Web dashboard with quest status tracking, gate approve/reject endpoints, and embedded static assets. Served via <code>fellowship dashboard</code>.
			</li>
			<li>
				<strong>Fellowship state CLI</strong> — <code>fellowship state</code> commands for managing fellowship state, companies, and quest metadata.
			</li>
			<li>
				<strong>Data directory change</strong> — Working files moved from <code>tmp/</code> to <code>.fellowship/</code> for cleaner project directories.
			</li>
			<li>
				<strong>File locking</strong> — Cross-platform file locking for state mutations (replaced <code>syscall.Flock</code>).
			</li>
			<li>
				<strong>CI</strong> — Added PR workflow to run Go tests.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.6.3 -->
	<section class="version" id="v1-6-3">
		<h2 class="version-heading"><a href="{base}/changelog#v1-6-3">v1.6.3</a></h2>
		<ul class="changes">
			<li>
				<strong>Fix plugin discovery</strong> — moved <code>.claude-plugin/plugin.json</code> to repo root with explicit path fields for skills, agents, commands, and hooks. Fixes skills not showing up after install.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

	<!-- v1.6.1 -->
	<section class="version" id="v1-6-1">
		<h2 class="version-heading"><a href="{base}/changelog#v1-6-1">v1.6.1</a></h2>
		<ul class="changes">
			<li>
				<strong>GitHub Pages site</strong> — SvelteKit static site with LOTR theme, all documentation pages, and CI deployment.
			</li>
			<li>
				<strong><code>/rekindle</code> skill</strong> — Crash recovery. Scans worktrees and state files, presents a recovery dashboard, and re-spawns Gandalf with recovered quest context.
			</li>
			<li>
				<strong><code>/lorebook</code> skill</strong> — Loads phase-specific guidance from quest templates created by <code>/scribe</code>.
			</li>
			<li>
				<strong>Skills to commands migration</strong> — 5 user-only skills moved to <code>commands/</code> for lower base context cost.
			</li>
			<li>
				<strong>LOTR theming</strong> — Internal renames: convoy → company, cv → tome, patrol → eagles, work/hook → errand, events/feed → herald.
			</li>
		</ul>
	</section>

	<div class="divider"><span class="divider-ring"></span></div>

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
