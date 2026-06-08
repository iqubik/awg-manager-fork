<script lang="ts">
	import SettingsSectionLabel from './SettingsSectionLabel.svelte';
	import { Button } from '$lib/components/ui';
	import { summonPukhosos } from '$lib/stores/pukhososSummon';
	import { PUKHOSOS_PATROL_MS } from '$lib/utils/pukhososPatrol';
	import { FlaskConical } from 'lucide-svelte';

	let summoning = $state(false);

	function handleSummon() {
		if (summoning) return;
		summoning = true;
		summonPukhosos();
		window.setTimeout(() => {
			summoning = false;
		}, PUKHOSOS_PATROL_MS);
	}
</script>

<div class="settings-block" id="experimental-settings">
	<div class="card">
		<SettingsSectionLabel label="Экспериментальное" icon={FlaskConical} tone="info" header cycleInVivid />
		<div class="setting-row">
			<div class="flex flex-col gap-1">
				<span class="font-medium">Пухосос</span>
				<span class="setting-description">
					Вызвать пухосос, он пару раз проедет по блоку ссылок и благодарностей.
				</span>
			</div>
			<Button variant="secondary" size="md" onclick={handleSummon} disabled={summoning}>
				{summoning ? 'Едет…' : 'Вызвать пухосос'}
			</Button>
		</div>
	</div>
</div>