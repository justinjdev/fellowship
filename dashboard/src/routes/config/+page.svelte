<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchConfig, saveConfig } from '$lib/api';

	let globalConfig = $state<Record<string, unknown>>({});
	let projectConfig = $state<Record<string, unknown>>({});
	let loading = $state(true);
	let saving = $state<string | null>(null);
	let saveStatus = $state<{ key: string; ok: boolean } | null>(null);

	const knownKeys: {
		key: string;
		label: string;
		type: 'boolean' | 'text' | 'select';
		options?: string[];
	}[] = [
		{ key: 'gate_auto_approve', label: 'Gate Auto-Approve', type: 'boolean' },
		{ key: 'branch_prefix', label: 'Branch Prefix', type: 'text' },
		{ key: 'pr_draft', label: 'PR Draft Mode', type: 'boolean' },
		{ key: 'palantir_enabled', label: 'Palantir Enabled', type: 'boolean' },
		{ key: 'worktree_strategy', label: 'Worktree Strategy', type: 'select', options: ['auto', 'manual'] },
	];

	function getEffectiveValue(key: string): unknown {
		if (key in projectConfig) return projectConfig[key];
		if (key in globalConfig) return globalConfig[key];
		return undefined;
	}

	function getScope(key: string): 'project' | 'global' {
		return key in projectConfig ? 'project' : 'global';
	}

	function unknownGlobalKeys(): string[] {
		const known = new Set(knownKeys.map((k) => k.key));
		return Object.keys(globalConfig).filter((k) => !known.has(k));
	}

	function unknownProjectKeys(): string[] {
		const known = new Set(knownKeys.map((k) => k.key));
		return Object.keys(projectConfig).filter((k) => !known.has(k));
	}

	async function handleSave(key: string, value: unknown, scope: 'global' | 'project') {
		saving = key;
		try {
			await saveConfig(key, value, scope);
			if (scope === 'project') {
				projectConfig[key] = value;
			} else {
				globalConfig[key] = value;
			}
			saveStatus = { key, ok: true };
		} catch {
			saveStatus = { key, ok: false };
		}
		saving = null;
		setTimeout(() => {
			if (saveStatus?.key === key) saveStatus = null;
		}, 2000);
	}

	async function handleToggle(key: string) {
		const current = getEffectiveValue(key);
		const scope = getScope(key);
		await handleSave(key, !current, scope);
	}

	async function handleText(key: string, e: Event) {
		const target = e.target as HTMLInputElement;
		const scope = getScope(key);
		await handleSave(key, target.value, scope);
	}

	async function handleSelect(key: string, e: Event) {
		const target = e.target as HTMLSelectElement;
		const scope = getScope(key);
		await handleSave(key, target.value, scope);
	}

	onMount(async () => {
		try {
			const config = await fetchConfig();
			globalConfig = (config.global as Record<string, unknown>) ?? {};
			projectConfig = (config.project as Record<string, unknown>) ?? {};
		} catch {
			// Config load failed — show empty state
		} finally {
			loading = false;
		}
	});
</script>

<div class="config-view">
	<div class="view-header">
		<h1>Config</h1>
	</div>

	{#if loading}
		<div class="empty-state">Loading configuration...</div>
	{:else}
		<div class="config-sections">
			<div class="config-section">
				<div class="section-header">Settings</div>
				<div class="settings-list">
					{#each knownKeys as setting}
						{@const val = getEffectiveValue(setting.key)}
						{@const scope = getScope(setting.key)}
						<div class="setting-row">
							<div class="setting-info">
								<span class="setting-label">{setting.label}</span>
								<span class="setting-scope">{scope}</span>
							</div>
							<div class="setting-control">
								{#if setting.type === 'boolean'}
									<button
										class="toggle-btn"
										class:active={!!val}
										disabled={saving === setting.key}
										onclick={() => handleToggle(setting.key)}
									>
										{val ? 'ON' : 'OFF'}
									</button>
								{:else if setting.type === 'text'}
									<input
										type="text"
										class="setting-input"
										value={val ?? ''}
										disabled={saving === setting.key}
										onblur={(e) => handleText(setting.key, e)}
									/>
								{:else if setting.type === 'select'}
									<select
										class="setting-select"
										value={val ?? setting.options?.[0]}
										disabled={saving === setting.key}
										onchange={(e) => handleSelect(setting.key, e)}
									>
										{#each setting.options ?? [] as opt}
											<option value={opt}>{opt}</option>
										{/each}
									</select>
								{/if}
								{#if saveStatus?.key === setting.key}
									<span class="save-indicator" class:error={!saveStatus.ok}>
										{saveStatus.ok ? 'Saved' : 'Error'}
									</span>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>

			{#if unknownGlobalKeys().length > 0}
				<div class="config-section">
					<div class="section-header">Global Config (raw)</div>
					<pre class="raw-json">{JSON.stringify(
						Object.fromEntries(unknownGlobalKeys().map((k) => [k, globalConfig[k]])),
						null,
						2
					)}</pre>
				</div>
			{/if}

			{#if unknownProjectKeys().length > 0}
				<div class="config-section">
					<div class="section-header">Project Config (raw)</div>
					<pre class="raw-json">{JSON.stringify(
						Object.fromEntries(unknownProjectKeys().map((k) => [k, projectConfig[k]])),
						null,
						2
					)}</pre>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.config-view {
		padding: var(--space-lg);
		display: flex;
		flex-direction: column;
		gap: var(--space-lg);
		height: 100%;
	}

	.view-header h1 {
		font-family: var(--font-heading);
		font-size: 18px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.config-sections {
		display: flex;
		flex-direction: column;
		gap: var(--space-lg);
		overflow-y: auto;
		flex: 1;
	}

	.config-section {
		background: var(--bg-surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-md);
	}

	.section-header {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--text-faint);
		margin-bottom: var(--space-md);
	}

	.settings-list {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.setting-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px var(--space-sm);
		border-radius: var(--radius-md);
		transition: background var(--transition-fast);
	}

	.setting-row:hover {
		background: var(--bg-raised);
	}

	.setting-info {
		display: flex;
		align-items: baseline;
		gap: var(--space-sm);
	}

	.setting-label {
		font-size: 13px;
		font-weight: 500;
		color: var(--text-primary);
	}

	.setting-scope {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--text-faint);
		background: var(--bg-raised);
		padding: 1px 6px;
		border-radius: var(--radius-sm);
	}

	.setting-control {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.toggle-btn {
		font-family: var(--font-mono);
		font-size: 11px;
		font-weight: 600;
		padding: 4px 14px;
		border-radius: var(--radius-md);
		background: var(--bg-raised);
		color: var(--text-muted);
		border: 1px solid var(--border);
		cursor: pointer;
		transition: all var(--transition-fast);
		min-width: 52px;
	}

	.toggle-btn.active {
		background: var(--accent-green-dim);
		color: var(--accent-green-text);
		border-color: var(--accent-green-dim);
	}

	.toggle-btn:hover:not(:disabled) {
		border-color: var(--border-hover);
	}

	.toggle-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.setting-input {
		background: var(--bg-raised);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 5px 10px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-primary);
		outline: none;
		width: 180px;
		transition: border-color var(--transition-fast);
	}

	.setting-input:focus {
		border-color: var(--border-active);
	}

	.setting-select {
		background: var(--bg-raised);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 5px 10px;
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-primary);
		outline: none;
		cursor: pointer;
	}

	.setting-select option {
		background: var(--bg-raised);
		color: var(--text-primary);
	}

	.save-indicator {
		font-size: 11px;
		color: var(--accent-green-text);
		animation: fadeIn 150ms ease;
	}

	.save-indicator.error {
		color: var(--accent-red);
	}

	@keyframes fadeIn {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	.raw-json {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--text-secondary);
		background: var(--bg-raised);
		padding: var(--space-md);
		border-radius: var(--radius-md);
		overflow-x: auto;
		line-height: 1.5;
		margin: 0;
	}

	.empty-state {
		text-align: center;
		padding: var(--space-2xl);
		color: var(--text-muted);
		font-size: 13px;
	}
</style>
