export async function approveGate(dir: string): Promise<void> {
	await fetch('/api/gate/approve', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ dir }),
	});
}

export async function rejectGate(dir: string): Promise<void> {
	await fetch('/api/gate/reject', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ dir }),
	});
}

export async function spawnQuest(task: string, branch?: string, company?: string): Promise<string> {
	const res = await fetch('/api/quest/spawn', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ task, branch, company }),
	});
	const data = await res.json();
	return data.command_id;
}

export async function spawnScout(question: string): Promise<string> {
	const res = await fetch('/api/scout/spawn', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ question }),
	});
	const data = await res.json();
	return data.command_id;
}

export async function killQuest(questId: string): Promise<string> {
	const res = await fetch('/api/quest/kill', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ quest_id: questId }),
	});
	const data = await res.json();
	return data.command_id;
}

export async function restartQuest(questId: string): Promise<string> {
	const res = await fetch('/api/quest/restart', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ quest_id: questId }),
	});
	const data = await res.json();
	return data.command_id;
}

export async function fetchErrands(worktree: string): Promise<unknown> {
	const encoded = btoa(worktree).replace(/\+/g, '-').replace(/\//g, '_');
	const res = await fetch(`/api/errand/${encoded}`);
	if (res.ok) return res.json();
	return null;
}

export async function fetchTome(questName: string): Promise<unknown> {
	const res = await fetch(`/api/tome/${encodeURIComponent(questName)}`);
	if (res.ok) return res.json();
	return null;
}

export async function fetchAutopsies(): Promise<unknown[]> {
	const res = await fetch('/api/autopsies');
	if (res.ok) return res.json();
	return [];
}

export async function fetchConfig(): Promise<{ global: unknown; project: unknown }> {
	const res = await fetch('/api/config');
	if (res.ok) return res.json();
	return { global: null, project: null };
}

export async function saveConfig(key: string, value: unknown, scope: 'global' | 'project'): Promise<void> {
	await fetch('/api/config', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ key, value, scope }),
	});
}

export async function fetchCommands(): Promise<unknown[]> {
	const res = await fetch('/api/commands');
	if (res.ok) return res.json();
	return [];
}
