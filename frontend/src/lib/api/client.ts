// Фасад API-клиента. Доменные методы разнесены по слоям client*.ts
// (цепочка наследования от CoreClient); публичная поверхность не менялась:
// `api` и сопутствующие экспорты доступны по прежнему пути $lib/api/client.
import type {
	IntegrationRestoreResponse,
	MonitoringSample,
	SingboxWatchdogConfig,
	SingboxWatchdogLogEntry,
	SingboxWatchdogStatus,
} from '$lib/types';
import { SubscriptionsClient } from './clientSubscriptions';

export { ApiGatewayError } from './clientCore';
export type { TrafficPeriod } from './clientCore';

class ApiClient extends SubscriptionsClient {
	private async downloadBinary(endpoint: string, fallbackFilename: string): Promise<void> {
		const response = await fetch(`${this.baseUrl}${endpoint}`, {
			credentials: 'same-origin',
			signal: this.abortController.signal,
		});
		if (!response.ok) {
			const contentType = response.headers.get('content-type') || '';
			if (response.status === 401) {
				this.onUnauthorized?.();
				throw new Error('Сессия истекла');
			}
			if (contentType.includes('application/json')) {
				const payload = (await response.json().catch(() => null)) as { message?: string } | null;
				throw new Error(payload?.message || `Ошибка запроса (${response.status})`);
			}
			throw new Error(`Ошибка сервера (${response.status})`);
		}
		const blob = await response.blob();
		const filename = response.headers.get('Content-Disposition')
			?.match(/filename="(.+)"/)?.[1] || fallbackFilename;
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = filename;
		link.click();
		URL.revokeObjectURL(url);
	}

	private async restoreIntegration(
		component: 'singbox' | 'hydraroute',
		file: File,
		dryRun: boolean,
	): Promise<IntegrationRestoreResponse> {
		let response: Response;
		try {
			response = await fetch(`${this.baseUrl}/${component}/restore?dryRun=${dryRun ? 'true' : 'false'}`, {
				method: 'POST',
				credentials: 'same-origin',
				signal: this.abortController.signal,
				headers: {
					'Content-Type': file.type || 'application/zip',
				},
				body: file,
			});
		} catch (e) {
			if (e instanceof DOMException && e.name === 'AbortError') {
				throw e;
			}
			this.onConnectionLost?.();
			throw new Error('Ошибка сети: не удалось подключиться к серверу');
		}
		if (response.status === 401) {
			this.onUnauthorized?.();
			throw new Error('Сессия истекла');
		}
		const contentType = response.headers.get('content-type') || '';
		if (!contentType.includes('application/json')) {
			if (!response.ok) {
				throw new Error(`Ошибка сервера (${response.status})`);
			}
			throw new Error('Некорректный ответ сервера');
		}
		const payload = (await response.json()) as {
			data?: IntegrationRestoreResponse;
			error?: boolean;
			message?: string;
		};
		if (!response.ok || payload.error || !payload.data) {
			throw new Error(payload.message || `Ошибка запроса (${response.status})`);
		}
		return payload.data;
	}

	async downloadHydraRouteBackup(): Promise<void> {
		return this.downloadBinary('/hydraroute/backup', 'awgm-hydraroute-backup.zip');
	}

	async previewHydraRouteRestore(file: File): Promise<IntegrationRestoreResponse> {
		return this.restoreIntegration('hydraroute', file, true);
	}

	async restoreHydraRouteBackup(file: File): Promise<IntegrationRestoreResponse> {
		return this.restoreIntegration('hydraroute', file, false);
	}

	async downloadSingboxBackup(): Promise<void> {
		return this.downloadBinary('/singbox/backup', 'awgm-singbox-backup.zip');
	}

	async previewSingboxRestore(file: File): Promise<IntegrationRestoreResponse> {
		return this.restoreIntegration('singbox', file, true);
	}

	async restoreSingboxBackup(file: File): Promise<IntegrationRestoreResponse> {
		return this.restoreIntegration('singbox', file, false);
	}

	async selectDeviceProxyInstanceRuntime(id: string, tag: string): Promise<{ active: string }> {
		return this.request<{ active: string }>(`/proxy/instance/runtime/select?id=${encodeURIComponent(id)}`, {
			method: 'POST',
			body: JSON.stringify({ tag }),
		});
	}

	async getMonitoringHistory(opts: {
		target: string;
		tunnelId: string;
		limit?: number;
	}): Promise<MonitoringSample[]> {
		const params = new URLSearchParams({
			target: opts.target,
			tunnelId: opts.tunnelId,
		});
		if (typeof opts.limit === 'number' && opts.limit > 0) {
			params.set('limit', String(opts.limit));
		}
		return this.request<MonitoringSample[]>(`/monitoring/history?${params.toString()}`);
	}

	async singboxWatchdogStatus(): Promise<SingboxWatchdogStatus[]> {
		return this.request('/singbox/watchdog/status');
	}

	async singboxWatchdogLogs(targetId?: string): Promise<SingboxWatchdogLogEntry[]> {
		const qs = targetId ? `?targetId=${encodeURIComponent(targetId)}` : '';
		return this.request(`/singbox/watchdog/logs${qs}`);
	}

	async singboxWatchdogConfigure(config: SingboxWatchdogConfig): Promise<void> {
		await this.request('/singbox/watchdog/configure', {
			method: 'POST',
			body: JSON.stringify(config),
		});
	}

	async singboxWatchdogEnable(id: string): Promise<void> {
		await this.request('/singbox/watchdog/enable', {
			method: 'POST',
			body: JSON.stringify({ id }),
		});
	}

	async singboxWatchdogDisable(id: string): Promise<void> {
		await this.request('/singbox/watchdog/disable', {
			method: 'POST',
			body: JSON.stringify({ id }),
		});
	}

	async singboxWatchdogCheckNow(id?: string): Promise<void> {
		await this.request('/singbox/watchdog/check-now', {
			method: 'POST',
			body: JSON.stringify(id ? { id } : {}),
		});
	}
}

export const api = new ApiClient();
