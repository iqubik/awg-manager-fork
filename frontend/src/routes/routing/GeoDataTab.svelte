<script lang="ts">
	import { api } from '$lib/api/client';
	import type { GeoFileEntry, GeoFileSettings, Settings } from '$lib/types';
	import { HrNeoGeoDataView } from '$lib/components/hrneo';
	import { notifications } from '$lib/stores/notifications';

	let geoFiles = $state<GeoFileEntry[]>([]);
	let settings = $state<Settings | null>(null);
	let saving = $state(false);
	let updatingAll = $state(false);

	async function loadGeoFiles() {
		try { await api.rescanGeoFiles(); } catch { /* HR не установлен — не ломаем вкладку */ }
		try { geoFiles = (await api.getGeoFiles()) ?? []; } catch { geoFiles = []; }
	}

	async function loadSettings() {
		try { settings = await api.getSettings(); } catch { settings = null; }
	}

	$effect(() => {
		void loadGeoFiles();
		void loadSettings();
	});

	async function saveGeo(next: GeoFileSettings) {
		if (!settings) return;
		saving = true;
		try {
			settings = await api.updateSettings({ ...settings, geoFile: next });
			notifications.success('Настройки автообновления сохранены');
		} catch {
			notifications.error('Ошибка сохранения настроек');
		} finally {
			saving = false;
		}
	}

	async function updateAllNow() {
		updatingAll = true;
		try {
			const upd = await api.updateGeoFile('');
			await loadGeoFiles();
			if (upd.partial && upd.error) {
				notifications.error(
					upd.updated > 0 ? `Обновлено ${upd.updated}, ошибки: ${upd.error}` : upd.error,
				);
				return;
			}
			notifications.success(`Обновлено ${upd.updated}`);
		} catch (e) {
			await loadGeoFiles();
			notifications.error(e instanceof Error ? e.message : 'Ошибка обновления');
		} finally {
			updatingAll = false;
		}
	}
</script>

<HrNeoGeoDataView
	files={geoFiles}
	settings={settings}
	saving={saving}
	updatingAll={updatingAll}
	onrefresh={loadGeoFiles}
	onSaveGeoSettings={saveGeo}
	onUpdateAllNow={updateAllNow}
/>
