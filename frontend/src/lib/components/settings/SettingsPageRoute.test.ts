import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const settingsPagePath = resolve(process.cwd(), 'src/routes/settings/+page.svelte');

describe('settings page source', () => {
	it('moves access controls out of a dedicated access card and wires feedback through appearance', () => {
		const source = readFileSync(settingsPagePath, 'utf8');

		expect(source).toMatch(/from\s+["']lucide-svelte["']/);
		expect(source).not.toMatch(/\bLock\b/);
		expect(source).not.toMatch(/<SettingsSectionLabel\s+label="Доступ"/);
		expect(source).toMatch(/<ThemeSchemeCard[\s\S]*showFeedbackToggle=\{settings\.updates\.channel === 'develop'\}/);
		expect(source).toMatch(/<div class="setting-row session-ttl-row">/);
		expect(source).toMatch(/<span class="font-medium">Вход по учётным данным Entware<\/span>/);
	});
});
