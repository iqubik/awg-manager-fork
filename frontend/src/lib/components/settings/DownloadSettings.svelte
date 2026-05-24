<script lang="ts">
	import { Button, Dropdown, type DropdownOption } from '$lib/components/ui';
	import type { DownloadOutbound, Settings } from '$lib/types';

	interface Props {
		settings: Settings;
		saving: boolean;
		outbounds: DownloadOutbound[];
		loading: boolean;
		error: string;
		onRefresh: () => void;
		onSelectRoute: (routeTag: string) => void;
	}

	let {
		settings = $bindable(),
		saving,
		outbounds,
		loading,
		error,
		onRefresh,
		onSelectRoute,
	}: Props = $props();

	function maskSensitiveToken(token: string): string {
		if (!token) return token;
		if (token.length <= 6) return '*'.repeat(token.length);
		return `${token.slice(0, 3)}${'*'.repeat(token.length - 6)}${token.slice(-3)}`;
	}

	function maskSensitiveInText(text: string): string {
		if (!text) return text;
		const hostLike = /\b([a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+|\d{1,3}(?:\.\d{1,3}){3})\b/g;
		return text.replace(hostLike, (m) => maskSensitiveToken(m));
	}

	function kindLabel(kind?: string): string {
		const k = (kind ?? '').trim();
		if (!k) return '';
		if (k.toLowerCase() === 'subscription') return 'SUB';
		return k.toUpperCase();
	}

	function inferKindFromLabel(label: string): string {
		const s = label.toUpperCase();
		if (s.includes('VLESS')) return 'VLESS';
		if (s.includes('AWG') || s.includes('AMNEZIA')) return 'AWG';
		if (s.includes('SUB')) return 'SUB';
		if (s.includes('WIREGUARD') || s.includes('WG')) return 'WG';
		return '';
	}

	function displayRouteName(ob: DownloadOutbound): string {
		const base = maskSensitiveInText(ob.label);
		const k = kindLabel(ob.kind) || inferKindFromLabel(base);
		if (!k || k.toLowerCase() === 'direct') return base;
		if (base.toUpperCase().includes(`(${k})`) || base.toUpperCase().endsWith(`- ${k}`)) {
			return base;
		}
		return `${base} (${k})`;
	}

	function optionLabel(ob: DownloadOutbound): string {
		return `${displayRouteName(ob)}${ob.available ? '' : ' (unavailable)'}`;
	}

	const selectedTag = $derived(settings.download?.routeTag || 'direct');
	const hasSelected = $derived(outbounds.some((ob) => ob.tag === selectedTag));
	const options = $derived.by(() => {
		const built: DropdownOption<string>[] = outbounds.map((ob) => ({
			value: ob.tag,
			label: optionLabel(ob),
			disabled: !ob.available,
		}));
		if (!hasSelected && selectedTag) {
			built.unshift({
				value: selectedTag,
				label: `Недоступный маршрут: ${maskSensitiveInText(selectedTag)}`,
				disabled: true,
			});
		}
		return built;
	});

	function handleChange(v: string) {
		onSelectRoute(v);
	}
</script>

<div id="downloads" class="setting-row download-row">
	<div class="flex flex-col gap-1">
		<span class="font-medium">Служебные загрузки AWGM</span>
		<span class="setting-description">
			Используется для загрузки geo.dat и обновлений AWGM. Позже этот маршрут будет применяться к обновлениям sing-box, HydraRoute и спискам.
		</span>
		{#if error}
			<span class="download-error">{error}</span>
		{/if}
	</div>
	<div class="download-controls">
		<div class="route-select">
			<Dropdown
				value={selectedTag}
				options={options}
				onchange={handleChange}
				disabled={saving || loading || options.length === 0}
				fullWidth
			/>
		</div>
		<Button
			variant="secondary"
			size="sm"
			onclick={onRefresh}
			disabled={saving || loading}
		>
			Обновить список
		</Button>
	</div>
</div>

<style>
	#downloads {
		scroll-margin-top: 5.5rem;
	}
	.download-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		justify-content: stretch;
		align-items: stretch;
		gap: 0.75rem;
	}
	.download-row > :first-child {
		flex: initial;
	}
	.download-controls {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
	}
	.route-select {
		width: 100%;
		min-width: 0;
		max-width: 100%;
	}
	.download-error {
		color: var(--color-danger);
		font-size: 0.75rem;
	}
	@media (max-width: 900px) {
		.download-controls {
			grid-template-columns: 1fr;
			align-items: stretch;
		}
		.route-select {
			width: 100%;
			min-width: 0;
			max-width: none;
		}
	}
</style>
