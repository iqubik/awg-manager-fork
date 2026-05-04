<script lang="ts">
	import type { SubscriptionMember } from '$lib/types';

	interface Props {
		member: SubscriptionMember;
		active: boolean;
		switching: boolean;
		disabled: boolean;
		onclick: () => void;
	}
	let { member, active, switching, disabled, onclick }: Props = $props();

	const protocolLabel = $derived.by(() => {
		switch (member.protocol) {
			case 'vless': return 'VLESS';
			case 'trojan': return 'Trojan';
			case 'shadowsocks': return 'Shadowsocks';
			case 'hysteria2': return 'Hysteria2';
			case 'naive': return 'Naive';
			default: return member.protocol;
		}
	});
</script>

<button
	type="button"
	class="card"
	class:active
	class:switching
	{disabled}
	onclick={onclick}
	aria-pressed={active}
>
	<div class="header">
		<span class="led" class:on={active} aria-hidden="true"></span>
		<span class="title" title={member.tag}>{member.server}</span>
		<span class="port mono">:{member.port}</span>
	</div>
	<div class="badges">
		<span class="badge proto">{protocolLabel}</span>
		{#if member.transport && member.transport !== 'tcp'}
			<span class="badge transport">{member.transport.toUpperCase()}</span>
		{/if}
		{#if member.security === 'reality'}
			<span class="badge reality">Reality</span>
		{:else if member.security === 'tls'}
			<span class="badge tls">TLS</span>
		{/if}
	</div>
	<div class="footer">
		<span class="tag mono" title={member.tag}>{member.tag}</span>
		{#if active}
			<span class="state-badge active-badge">активен</span>
		{:else if switching}
			<span class="state-badge switching-badge">переключаем...</span>
		{/if}
	</div>
</button>

<style>
	.card {
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
		padding: 14px 16px;
		border: 1px solid var(--color-border);
		border-radius: 10px;
		background: var(--color-bg-secondary);
		color: var(--color-text-primary);
		font: inherit;
		text-align: left;
		cursor: pointer;
		transition: border-color 0.15s ease, background 0.15s ease;
	}
	.card:hover:not(.active):not(:disabled) { border-color: var(--color-accent); }
	.card.active { border-color: #3fb950; background: rgba(63, 185, 80, 0.06); }
	.card.switching { opacity: 0.7; cursor: wait; }
	.card:disabled { cursor: wait; opacity: 0.6; }
	.header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.led {
		width: 10px; height: 10px;
		border-radius: 999px;
		background: var(--color-bg-tertiary);
		flex-shrink: 0;
	}
	.led.on {
		background: #3fb950;
		box-shadow: 0 0 0 3px rgba(63, 185, 80, 0.22);
	}
	.title {
		font-size: 0.92rem;
		font-weight: 600;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.port { font-size: 0.78rem; color: var(--color-text-muted); }
	.badges { display: flex; gap: 0.4rem; flex-wrap: wrap; }
	.badge {
		font-size: 0.68rem;
		padding: 0.15rem 0.5rem;
		border-radius: 4px;
		font-weight: 600;
		letter-spacing: 0.3px;
	}
	.badge.proto { background: rgba(88,166,255,0.15); color: var(--color-accent); }
	.badge.transport { background: var(--color-bg-tertiary); color: var(--color-text-muted); }
	.badge.tls { background: rgba(63,185,80,0.15); color: #3fb950; }
	.badge.reality { background: rgba(210,153,34,0.15); color: #d29922; }
	.footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding-top: 0.4rem;
		border-top: 1px solid var(--color-border);
	}
	.tag {
		font-size: 0.68rem;
		color: var(--color-text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 150px;
	}
	.state-badge {
		font-size: 0.7rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
	}
	.active-badge { background: rgba(63,185,80,0.15); color: #3fb950; }
	.switching-badge { background: rgba(88,166,255,0.15); color: var(--color-accent); }
	.mono { font-family: var(--font-mono, ui-monospace, monospace); }
</style>
