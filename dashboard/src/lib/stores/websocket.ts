import { writable } from 'svelte/store';

export type WSEventType =
	| 'quest-changed'
	| 'gate-submitted'
	| 'gate-resolved'
	| 'herald-event'
	| 'alert'
	| 'command-queued'
	| 'command-completed';

export interface WSEvent {
	type: WSEventType;
	quest_id?: string;
	phase?: string;
	action?: string;
	alert_type?: string;
	quests?: string[];
	command_id?: string;
	timestamp: number;
}

export const connected = writable(false);
export const lastEvent = writable<WSEvent | null>(null);

let ws: WebSocket | null = null;
let reconnectDelay = 1000;
const MAX_DELAY = 30000;
let shouldReconnect = true;

function getWSUrl(): string {
	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	return `${protocol}//${window.location.host}/ws`;
}

function connect() {
	if (ws?.readyState === WebSocket.OPEN) return;

	let socket: WebSocket;
	try {
		socket = new WebSocket(getWSUrl());
	} catch {
		scheduleReconnect();
		return;
	}
	ws = socket;

	socket.onopen = () => {
		if (ws !== socket) return;
		connected.set(true);
		reconnectDelay = 1000;
	};

	socket.onclose = () => {
		if (ws !== socket) return;
		connected.set(false);
		ws = null;
		if (shouldReconnect) scheduleReconnect();
	};

	socket.onerror = () => {
		socket.close();
	};

	socket.onmessage = (event) => {
		try {
			const data: WSEvent = JSON.parse(event.data);
			lastEvent.set(data);
		} catch {
			// ignore malformed messages
		}
	};
}

function scheduleReconnect() {
	setTimeout(() => {
		if (shouldReconnect) connect();
	}, reconnectDelay);
	reconnectDelay = Math.min(reconnectDelay * 2, MAX_DELAY);
}

export function startWebSocket() {
	shouldReconnect = true;
	connect();
}

export function stopWebSocket() {
	shouldReconnect = false;
	ws?.close();
}
