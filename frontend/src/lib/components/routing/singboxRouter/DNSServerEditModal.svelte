<script lang="ts">
	import { Button, Dropdown, SideDrawer, type DropdownOption } from '$lib/components/ui';
	import type {
		SingboxRouterDNSServer,
		SingboxRouterDNSType,
		SingboxRouterDNSStrategy,
	} from '$lib/types';
	import type { OutboundGroup } from './outboundOptions';

	interface Props {
		server?: SingboxRouterDNSServer;
		servers: SingboxRouterDNSServer[];
		outboundOptions: OutboundGroup[];
		onClose: () => void;
		onSave: (server: SingboxRouterDNSServer) => Promise<void> | void;
	}
	let { server, servers, outboundOptions, onClose, onSave }: Props = $props();

	const TYPE_OPTIONS: DropdownOption<SingboxRouterDNSType>[] = [
		{ value: 'udp', label: 'UDP (обычный DNS)' },
		{ value: 'tls', label: 'DoT (DNS over TLS)' },
		{ value: 'https', label: 'DoH (DNS over HTTPS)' },
		{ value: 'quic', label: 'DoQ (DNS over QUIC)' },
		{ value: 'h3', label: 'DoH3' },
		{ value: 'local', label: 'Local (системный resolver роутера)' },
	];

	const STRATEGY_OPTIONS: DropdownOption<SingboxRouterDNSStrategy>[] = [
		{ value: '', label: '— default —' },
		{ value: 'ipv4_only', label: 'ipv4_only' },
		{ value: 'ipv6_only', label: 'ipv6_only' },
		{ value: 'prefer_ipv4', label: 'prefer_ipv4' },
		{ value: 'prefer_ipv6', label: 'prefer_ipv6' },
	];

	const detourOptions = $derived<DropdownOption[]>([
		{ value: '', label: '— через route (по умолчанию) —' },
		...outboundOptions.flatMap((g) =>
			g.items
				.filter((i) => server != null || i.value !== 'direct')
				.map((i) => ({ value: i.value, label: i.label, group: g.group })),
		),
	]);

	// svelte-ignore state_referenced_locally
	let tag = $state(server?.tag ?? '');
	// svelte-ignore state_referenced_locally
	let type = $state<SingboxRouterDNSType>(server?.type ?? 'udp');
	// svelte-ignore state_referenced_locally
	let serverAddr = $state(server?.server ?? '');
	// svelte-ignore state_referenced_locally
	let serverPort = $state<number | ''>(server?.server_port ?? '');
	// svelte-ignore state_referenced_locally
	let path = $state(server?.path ?? '');
	// svelte-ignore state_referenced_locally
	let detour = $state(server?.detour ?? '');
	// svelte-ignore state_referenced_locally
	let strategy = $state<SingboxRouterDNSStrategy>(server?.domain_strategy ?? '');
	// svelte-ignore state_referenced_locally
	let resolverEnabled = $state(server?.domain_resolver != null);
	// svelte-ignore state_referenced_locally
	let resolverServer = $state(server?.domain_resolver?.server ?? '');
	// svelte-ignore state_referenced_locally
	let resolverStrategy = $state<SingboxRouterDNSStrategy>(server?.domain_resolver?.strategy ?? '');

	let busy = $state(false);
	let error = $state('');

	// Snapshot initial state for isDirty detection
	let initialTag = $state('');
	let initialType = $state<SingboxRouterDNSType>('udp');
	let initialServerAddr = $state('');
	let initialServerPort = $state<number | ''>('');
	let initialPath = $state('');
	let initialDetour = $state('');
	let initialStrategy = $state<SingboxRouterDNSStrategy>('');
	let initialResolverEnabled = $state(false);
	let initialResolverServer = $state('');
	let initialResolverStrategy = $state<SingboxRouterDNSStrategy>('');

	// Initialize snapshot when modal opens
	$effect(() => {
		if (server) {
			initialTag = server.tag;
			initialType = server.type;
			initialServerAddr = server.server;
			initialServerPort = server.server_port ?? '';
			initialPath = server.path ?? '';
			initialDetour = server.detour ?? '';
			initialStrategy = server.domain_strategy ?? '';
			initialResolverEnabled = server.domain_resolver != null;
			initialResolverServer = server.domain_resolver?.server ?? '';
			initialResolverStrategy = server.domain_resolver?.strategy ?? '';
		} else {
			initialTag = '';
			initialType = 'udp';
			initialServerAddr = '';
			initialServerPort = '';
			initialPath = '';
			initialDetour = '';
			initialStrategy = '';
			initialResolverEnabled = false;
			initialResolverServer = '';
			initialResolverStrategy = '';
		}
	});

	const isDirty = $derived.by(() => {
		return (
			tag !== initialTag ||
			type !== initialType ||
			serverAddr !== initialServerAddr ||
			serverPort !== initialServerPort ||
			path !== initialPath ||
			detour !== initialDetour ||
			strategy !== initialStrategy ||
			resolverEnabled !== initialResolverEnabled ||
			resolverServer !== initialResolverServer ||
			resolverStrategy !== initialResolverStrategy
		);
	});

	const needsResolver = $derived(type !== 'udp' && type !== 'local' && !isIPLiteral(serverAddr));
	const availableResolvers = $derived(servers.filter((s) => s.tag !== tag).map((s) => s.tag));
	const resolverServerOptions = $derived<DropdownOption[]>([
		{ value: '', label: '— выберите —' },
		...availableResolvers.map((t) => ({ value: t, label: t })),
	]);

	function isIPLiteral(s: string): boolean {
		return /^(\d{1,3}\.){3}\d{1,3}$/.test(s) || s.includes(':');
	}

	async function save(): Promise<void> {
		busy = true;
		error = '';
		try {
			if (!tag.trim()) { error = 'Tag обязателен'; busy = false; return; }
			if (type !== 'local' && !serverAddr.trim()) { error = 'Server обязателен'; busy = false; return; }
			if (resolverEnabled && !resolverServer) { error = 'Укажите domain_resolver'; busy = false; return; }

			const built: SingboxRouterDNSServer = {
				tag: tag.trim(),
				type,
				server: type === 'local' ? '' : serverAddr.trim(),
			};
			if (type !== 'local') {
				if (serverPort !== '' && Number(serverPort) > 0) built.server_port = Number(serverPort);
				if (path.trim()) built.path = path.trim();
				if (resolverEnabled && resolverServer) {
					built.domain_resolver = { server: resolverServer };
					if (resolverStrategy) built.domain_resolver.strategy = resolverStrategy;
				}
			}
			if (type !== 'local') {
				if (detour) built.detour = detour;
				if (strategy) built.domain_strategy = strategy;
			}

			await onSave(built);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}
</script>

<SideDrawer
	open
	onClose={onClose}
	title={server ? 'Редактировать DNS сервер' : 'Новый DNS сервер'}
	width={620}
>
	<div class="drawer-card">
		<div class="drawer-card-body">
			<div class="form">
		<div class="fields-grid">
			<label class="field">
				<div class="lbl">Tag <span class="req">*</span></div>
				<input bind:value={tag} placeholder="bootstrap, cloudflare, vpn-dns" />
			</label>

			<label class="field">
				<div class="lbl">Type <span class="req">*</span></div>
				<Dropdown bind:value={type} options={TYPE_OPTIONS} fullWidth />
			</label>

			{#if type !== 'local'}
				<label class="field span-full">
					<div class="lbl">Server <span class="req">*</span></div>
					<input bind:value={serverAddr} placeholder={type === 'udp' ? '1.1.1.1' : 'cloudflare-dns.com'} />
				</label>

				<label class="field" class:span-full={type !== 'https'}>
					<div class="lbl">Server port</div>
					<input type="number" bind:value={serverPort} placeholder={type === 'udp' ? '53' : type === 'https' ? '443' : '853'} />
				</label>

				{#if type === 'https'}
					<label class="field">
						<div class="lbl">Path</div>
						<input bind:value={path} placeholder="/dns-query" />
					</label>
				{/if}
			{:else}
				<div class="field span-full hint">
					Local-сервер резолвит через системный resolver роутера (NDMS/AdGuard/Pi-hole). Адрес и порт не требуются.
				</div>
			{/if}
		</div>

		{#if type !== 'local'}
			<section class="form-section">
				<div class="section-label">Маршрутизация</div>

				<label class="field">
					<div class="lbl">Detour (outbound)</div>
					<Dropdown bind:value={detour} options={detourOptions} fullWidth />
					<div class="hint">
						{#if server}
							Через какой outbound сам сервер отправляет запросы. <code>direct</code> — через провайдера,
							выбранный туннель — через VPN (шифрованный DNS без утечек).
						{:else}
							Через какой outbound сам сервер отправляет запросы.
							Выбранный туннель — через VPN (шифрованный DNS без утечек).
						{/if}
					</div>
				</label>

				<label class="field">
					<div class="lbl">Стратегия (IPv4/IPv6)</div>
					<Dropdown bind:value={strategy} options={STRATEGY_OPTIONS} fullWidth />
				</label>
			</section>
		{/if}

		{#if type !== 'udp' && type !== 'local'}
			<section class="form-section">
				<div class="section-label">Bootstrap resolver (для домена сервера)</div>

				<label class="toggle">
					<input type="checkbox" bind:checked={resolverEnabled} />
					<span>Использовать другой DNS для резолва домена этого сервера</span>
				</label>

				{#if needsResolver && !resolverEnabled}
					<div class="warn">
						У <code>{type}</code> сервера адрес — доменное имя. Без bootstrap resolver sing-box не сможет его резолвить.
					</div>
				{/if}

				{#if resolverEnabled}
					<div class="resolver-fields">
						<label class="field">
							<div class="lbl">Resolver server (tag)</div>
							<Dropdown bind:value={resolverServer} options={resolverServerOptions} fullWidth />
						</label>
						<label class="field">
							<div class="lbl">Resolver strategy</div>
							<Dropdown bind:value={resolverStrategy} options={STRATEGY_OPTIONS} fullWidth />
						</label>
					</div>
				{/if}
			</section>
		{/if}

		{#if error}<div class="error">{error}</div>{/if}
			</div>
		</div>
		<footer class="drawer-card-footer">
			<Button variant="ghost" size="md" onclick={onClose} type="button">Отмена</Button>
			<Button variant="primary" size="md" onclick={save} disabled={busy} loading={busy} type="button">
				Сохранить
			</Button>
		</footer>
	</div>
</SideDrawer>

<style>
	.drawer-card {
		min-width: 0;
		border: 1px solid var(--border);
		border-radius: 12px;
		background:
			linear-gradient(180deg, rgba(255, 255, 255, 0.025), rgba(255, 255, 255, 0)),
			var(--bg-secondary, var(--color-bg-secondary));
		overflow: hidden;
	}
	.drawer-card-body {
		padding: 1rem;
		min-width: 0;
	}
	.drawer-card-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		padding: 0.875rem 1rem;
		border-top: 1px solid var(--border);
		background: var(--bg-secondary, var(--color-bg-secondary));
	}
	.form {
		display: grid;
		gap: 0.875rem;
		min-width: 0;
	}
	.fields-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem 0.6rem;
	}
	.span-full {
		grid-column: 1 / -1;
	}
	.form-section {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		padding-top: 0.5rem;
		border-top: 1px solid var(--border);
		min-width: 0;
	}
	.section-label {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		color: var(--muted-text);
		margin: 0;
	}
	.resolver-fields {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
	}
	@media (max-width: 520px) {
		.fields-grid,
		.resolver-fields {
			grid-template-columns: 1fr;
		}
		.span-full {
			grid-column: auto;
		}
	}
	@media (max-width: 640px) {
		.drawer-card {
			border-radius: 12px;
		}
		.drawer-card-body {
			padding: 0.875rem;
		}
		.drawer-card-footer {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 0.5rem;
			padding: 0.75rem 0.875rem;
			align-items: stretch;
		}
		.drawer-card-footer :global(.btn) {
			width: 100%;
			min-width: 0;
		}
	}
	.field {
		display: grid;
		gap: 0.25rem;
	}
	.lbl {
		font-size: 0.75rem;
		color: var(--muted-text);
	}
	.req {
		color: var(--danger, #dc2626);
	}
	.field input {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.4rem 0.6rem;
		border-radius: 4px;
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
		box-sizing: border-box;
		width: 100%;
	}
	.hint {
		font-size: 0.75rem;
		color: var(--muted-text);
		line-height: 1.4;
	}
	.hint code {
		background: var(--bg);
		padding: 0.05rem 0.25rem;
		border-radius: 2px;
		font-family: ui-monospace, monospace;
	}
	.toggle {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		font-size: 0.85rem;
		color: var(--text);
		cursor: pointer;
		line-height: 1.4;
	}
	.toggle input {
		margin-top: 0.15rem;
		flex-shrink: 0;
	}
	.warn {
		padding: 0.5rem 0.7rem;
		background: rgba(224, 175, 104, 0.12);
		border-left: 3px solid var(--warning, #e0af68);
		border-radius: 3px;
		font-size: 0.8rem;
		color: var(--muted-text);
		line-height: 1.4;
	}
	.warn code {
		background: var(--bg);
		padding: 0.05rem 0.25rem;
		border-radius: 2px;
		font-family: ui-monospace, monospace;
	}
	.error {
		color: var(--danger, #dc2626);
		font-size: 0.85rem;
	}
</style>
