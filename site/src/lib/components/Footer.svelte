<script lang="ts">
	import { base } from '$app/paths';
</script>

<footer class="footer">
	<div class="container">
		<div class="divider">
			<div class="divider-ring"></div>
		</div>
		<div class="footer-content">
			<div class="footer-links">
				<a href="{base}/getting-started">Getting Started</a>
				<a href="{base}/skills">Skills</a>
				<a href="{base}/configuration">Configuration</a>
				<a href="https://github.com/justinjdev/fellowship">GitHub</a>
			</div>
			<p class="footer-note">A Claude Code plugin. MIT License.</p>
		</div>
	</div>
</footer>

<style>
	.footer {
		padding: var(--space-lg) 0 var(--space-xl);
		margin-top: var(--space-2xl);
	}
	.footer-content { text-align: center; }
	.footer-links {
		display: flex;
		justify-content: center;
		gap: var(--space-lg);
		flex-wrap: wrap;
		margin-bottom: var(--space-md);
	}
	.footer-links a {
		font-size: 0.95rem;
		color: var(--color-text-secondary);
	}
	.footer-links a:hover { color: var(--color-accent); }
	.footer-note {
		font-size: 0.85rem;
		color: var(--color-text-secondary);
	}
</style>
