export interface QuestStatus {
	name: string;
	worktree: string;
	phase: string;
	status: string;
	gate_pending: boolean;
	gate_id: string | null;
	lembas_completed: boolean;
	metadata_updated: boolean;
	errands_done: number;
	errands_total: number;
}

export interface ScoutEntry {
	name: string;
	question: string;
	task_id: string;
}

export interface CompanyEntry {
	name: string;
	quests: string[];
	scouts: string[];
}

export interface DashboardStatus {
	name: string;
	quests: QuestStatus[];
	scouts: ScoutEntry[];
	companies: CompanyEntry[];
	poll_interval: number;
}

export interface QuestHealth {
	name: string;
	worktree: string;
	phase: string;
	health: 'working' | 'stalled' | 'zombie' | 'idle' | 'complete';
	gate_pending_sec: number;
	has_checkpoint: boolean;
	last_activity: string;
	action: string;
}

export interface Tiding {
	timestamp: string;
	quest: string;
	type: string;
	phase: string;
	detail: string;
}

export interface Problem {
	type: string;
	quest: string;
	detail: string;
	severity: string;
}

export interface Autopsy {
	version: number;
	ts: string;
	quest: string;
	task: string;
	phase: string;
	trigger: string;
	files: string[];
	modules: string[];
	what_failed: string;
	resolution: string;
	tags: string[];
}

export interface Errand {
	id: string;
	description: string;
	status: 'pending' | 'active' | 'done' | 'blocked';
	phase: string;
	depends_on: string[];
	created_at: string;
	updated_at: string;
}

export interface QuestErrandList {
	version: number;
	quest_name: string;
	task: string;
	items: Errand[];
}

export interface QuestTome {
	version: number;
	quest_name: string;
	created_at: string;
	updated_at: string;
	task: string;
	phases_completed: { phase: string; timestamp: string }[];
	gate_history: { phase: string; action: string; timestamp: string; reason?: string }[];
	files_touched: string[];
	respawns: number;
	status: string;
}

export interface Command {
	id: string;
	action: string;
	status: 'pending' | 'completed' | 'failed';
	params: Record<string, unknown>;
	timestamp: number;
	result?: string;
}

export interface DashboardError {
	id: number;
	timestamp: string;
	source: string;
	handler: string;
	message: string;
	detail?: string;
}
