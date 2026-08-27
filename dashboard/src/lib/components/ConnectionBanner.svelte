<script lang="ts">
	import { connected } from '$lib/stores/websocket';
</script>

{#if !$connected}
	<div class="banner">
		Disconnected — reconnecting...
	</div>
{/if}

<style>
	.banner {
		background: var(--accent-red-dim);
		color: var(--accent-red);
		text-align: center;
		padding: 6px;
		font-size: 12px;
		font-weight: 500;
	}
</style>
