import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const settingsPagePath = resolve(process.cwd(), 'src/routes/settings/+page.svelte');

describe('settings page source', () => {
	it('keeps the Lock icon imported for the access section header', () => {
		const source = readFileSync(settingsPagePath, 'utf8');

		expect(source).toMatch(/from\s+["']lucide-svelte["']/);
		expect(source).toMatch(/\bLock\b/);
		expect(source).toMatch(/<SettingsSectionLabel\s+label="Доступ"\s+icon=\{Lock\}\s+tone="blue"\s+header\s*\/>/);
	});
});
