export async function approveGate(dir: string): Promise<void> {
	const res = await fetch('/api/gate/approve', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ dir }),
	});
	if (!res.ok) throw new Error(`Gate approve failed: ${res.status}`);
}

export async function rejectGate(dir: string): Promise<void> {
	const res = await fetch('/api/gate/reject', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ dir }),
	});
	if (!res.ok) throw new Error(`Gate reject failed: ${res.status}`);
}

export async function spawnQuest(task: string, branch?: string, company?: string): Promise<string> {
	const res = await fetch('/api/quest/spawn', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ task, branch, company }),
	});
	if (!res.ok) throw new Error(`Spawn quest failed: ${res.status}`);
	const data = await res.json();
	return data.command_id;
}

export async function spawnScout(question: string): Promise<string> {
	const res = await fetch('/api/scout/spawn', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ question }),
	});
	if (!res.ok) throw new Error(`Spawn scout failed: ${res.status}`);
	const data = await res.json();
	return data.command_id;
}

export async function killQuest(questId: string): Promise<string> {
	const res = await fetch('/api/quest/kill', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ quest_id: questId }),
	});
	if (!res.ok) throw new Error(`Kill quest failed: ${res.status}`);
	const data = await res.json();
	return data.command_id;
}

export async function restartQuest(questId: string): Promise<string> {
	const res = await fetch('/api/quest/restart', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ quest_id: questId }),
	});
	if (!res.ok) throw new Error(`Restart quest failed: ${res.status}`);
	const data = await res.json();
	return data.command_id;
}

export async function fetchErrands(worktree: string): Promise<unknown> {
	try {
		const encoded = btoa(String.fromCharCode(...new TextEncoder().encode(worktree)))
			.replace(/\+/g, '-')
			.replace(/\//g, '_');
		const res = await fetch(`/api/errand/${encoded}`);
		if (res.ok) return res.json();
		return null;
	} catch {
		return null;
	}
}

export async function fetchTome(questName: string): Promise<unknown> {
	try {
		const res = await fetch(`/api/tome/${encodeURIComponent(questName)}`);
		if (res.ok) return res.json();
		return null;
	} catch {
		return null;
	}
}

export async function fetchAutopsies(): Promise<unknown[]> {
	try {
		const res = await fetch('/api/autopsies');
		if (res.ok) return res.json();
		return [];
	} catch {
		return [];
	}
}

export async function fetchConfig(): Promise<{ global: unknown; project: unknown }> {
	try {
		const res = await fetch('/api/config');
		if (res.ok) return res.json();
		return { global: null, project: null };
	} catch {
		return { global: null, project: null };
	}
}

export async function saveConfig(key: string, value: unknown, scope: 'global' | 'project'): Promise<void> {
	const res = await fetch('/api/config', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ key, value, scope }),
	});
	if (!res.ok) throw new Error(`Save config failed: ${res.status}`);
}

export async function fetchCommands(): Promise<unknown[]> {
	try {
		const res = await fetch('/api/commands');
		if (res.ok) return res.json();
		return [];
	} catch {
		return [];
	}
}
