<script lang="ts">
	interface Props {
		host: string;
		port: number;
		show?: boolean;
		onToggle?: () => void;
		class?: string;
	}

	let { host, port, show = $bindable(false), onToggle, class: className = '' }: Props = $props();
</script>

<div class="tunnel-list-endpoint-line mono {className}">
	<span class="tunnel-list-endpoint-host" class:tunnel-list-endpoint-host--muted={!show}>
		{show ? host : '••••••••'}
	</span>
	<button
		type="button"
		class="tunnel-list-endpoint-eye"
		onclick={(e) => {
			e.stopPropagation();
			show = !show;
			onToggle?.();
		}}
		aria-label={show ? 'Скрыть' : 'Показать'}
		title={show ? 'Скрыть' : 'Показать'}
	>
		{#if show}
			<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
				<circle cx="12" cy="12" r="3" />
			</svg>
		{:else}
			<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path
					d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
				/>
				<line x1="1" y1="1" x2="23" y2="23" />
			</svg>
		{/if}
	</button>
	<span
		class="tunnel-list-endpoint-port"
		class:tunnel-list-endpoint-port--muted={!show}
	>
		{show ? `:${port}` : ':•••••'}
	</span>
</div>
