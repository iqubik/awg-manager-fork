#!/usr/bin/env node
/**
 * Remove redundant subdomains from preset DNS domain lists in defaults.json.
 *
 * Usage:
 *   node scripts/rulesets/dehydrate-defaults-dns.js          # report only (default)
 *   node scripts/rulesets/dehydrate-defaults-dns.js --write  # apply to defaults.json
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { dehydrateDomainList } from './dns-dehydrate.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(__dirname, '../..');
const DEFAULTS_PATH = path.join(ROOT, 'internal/presets/defaults.json');
const WRITE = process.argv.includes('--write');

const presets = JSON.parse(fs.readFileSync(DEFAULTS_PATH, 'utf8'));
const changes = [];
let totalBefore = 0;
let totalAfter = 0;
let totalRemoved = 0;

for (const preset of presets) {
	const dns = preset.engines?.dns;
	if (!dns?.domains?.length) continue;

	totalBefore += dns.domains.length;
	const { domains, removed } = dehydrateDomainList(dns.domains);
	totalAfter += domains.length;
	totalRemoved += removed.length;

	if (removed.length === 0) continue;

	if (WRITE) preset.engines.dns.domains = domains;

	changes.push({
		id: preset.id,
		before: dns.domains.length,
		after: domains.length,
		removed: removed.length,
		samples: removed.slice(0, 8).map((r) => `${r.domain} ← ${r.coveredBy}`),
	});
}

changes.sort((a, b) => b.removed - a.removed);

const summary = {
	presetsTouched: changes.length,
	domainEntriesBefore: totalBefore,
	domainEntriesAfter: totalAfter,
	domainEntriesRemoved: totalRemoved,
	write: WRITE,
};

console.log(JSON.stringify({ summary, changes }, null, 2));

if (WRITE && changes.length > 0) {
	fs.writeFileSync(DEFAULTS_PATH, `${JSON.stringify(presets, null, 2)}\n`);
	console.error(`updated ${DEFAULTS_PATH}`);
} else if (!WRITE) {
	console.error(
		`dry-run: would remove ${totalRemoved} subdomain entries across ${changes.length} presets (${totalBefore} → ${totalAfter})`,
	);
}
