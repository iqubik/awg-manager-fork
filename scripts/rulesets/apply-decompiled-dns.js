#!/usr/bin/env node
/**
 * Merge short domain/CIDR lists from internal/presets/decompiled/*.json
 * into defaults.json DNS engines (cap: 500 domains per preset).
 *
 * Presets with `covers` keep only domains/subnets not already listed in
 * covered child presets — composite parents must not duplicate children.
 * Domain lists are dehydrated after each update (drop subdomains when apex exists).
 *
 * Usage:
 *   node scripts/rulesets/apply-decompiled-dns.js [--dry-run]
 *   node scripts/rulesets/apply-decompiled-dns.js --trim-only [--dry-run]
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { dehydrateDomainList } from './dns-dehydrate.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(__dirname, '../..');
const DECOMPILED = path.join(ROOT, 'internal/presets/decompiled');
const DEFAULTS_PATH = path.join(ROOT, 'internal/presets/defaults.json');
const MAX_DOMAINS = 500;
const DRY = process.argv.includes('--dry-run');
const TRIM_ONLY = process.argv.includes('--trim-only');

/** Presets that keep subscriptionUrl only (lists too long inline). */
const SKIP_IDS = new Set(['unavailable-in-russia', 'rkn', 'all-blocked', 'russian-services']);

function extractFromRules(rules) {
	const domains = new Set();
	const subnets = new Set();
	for (const r of rules) {
		const add = (d) => {
			if (!d) return;
			let x = String(d).trim().toLowerCase();
			if (x.startsWith('.')) x = x.slice(1);
			if (x && !x.includes('*') && !x.startsWith('^')) domains.add(x);
		};
		const addList = (v) => {
			if (!v) return;
			if (Array.isArray(v)) v.forEach(add);
			else add(v);
		};
		addList(r.domain);
		addList(r.domain_suffix);
		const cidrs = r.ip_cidr;
		if (Array.isArray(cidrs)) cidrs.forEach((c) => subnets.add(c));
		else if (cidrs) subnets.add(cidrs);
	}
	return {
		domains: [...domains].sort(),
		subnets: [...subnets].sort(),
	};
}

function loadExtract(relPath) {
	const p = path.join(DECOMPILED, relPath);
	if (!fs.existsSync(p)) return null;
	const data = JSON.parse(fs.readFileSync(p, 'utf8'));
	return extractFromRules(data.rules || []);
}

function indexPresets(presets) {
	const byId = new Map();
	for (const p of presets) byId.set(p.id, p);
	return byId;
}

/** Union of inline DNS entries from all presets listed in `covers`. */
function coveredDnsUnion(preset, byId) {
	const domains = new Set();
	const subnets = new Set();
	for (const id of preset.covers ?? []) {
		const child = byId.get(id);
		if (!child) {
			console.error(`warn: preset ${preset.id} covers unknown id ${id}`);
			continue;
		}
		for (const d of child.engines?.dns?.domains ?? []) domains.add(d.toLowerCase());
		for (const s of child.engines?.dns?.subnets ?? []) subnets.add(s);
	}
	return { domains, subnets };
}

function stripCoveredLists(domains, subnets, covered) {
	const nextDomains = (domains ?? [])
		.filter((d) => !covered.domains.has(String(d).toLowerCase()))
		.sort();
	const nextSubnets = (subnets ?? []).filter((s) => !covered.subnets.has(s)).sort();
	return { domains: nextDomains, subnets: nextSubnets };
}

function findDecompiledForPreset(preset) {
	const tries = new Set();
	const sb = preset.engines?.singbox;
	if (!sb?.ruleSets) return null;

	for (const rs of sb.ruleSets) {
		if (rs.tag) tries.add(path.join('sagernet', `${rs.tag}.json`));
		if (rs.url?.includes('vernette')) {
			const base = path.basename(rs.url, '.srs');
			tries.add(path.join('vernette', `${base}.json`));
			tries.add(`${base}.json`);
		}
	}
	tries.add(path.join('vernette', `${preset.id}.json`));
	tries.add(`${preset.id}.json`);

	for (const rel of tries) {
		const e = loadExtract(rel);
		if (e && (e.domains.length || e.subnets.length)) return e;
	}
	return null;
}

function buildDns(existing, extracted) {
	if (!extracted) return existing;
	const domains = new Set((existing?.domains ?? []).map((d) => d.toLowerCase()));
	const subnets = new Set(existing?.subnets ?? []);
	for (const d of extracted.domains) domains.add(d);
	for (const s of extracted.subnets) subnets.add(s);

	const out = {
		domains: [...domains].sort(),
		subnets: [...subnets].sort(),
	};
	if (existing?.subscriptionUrl) out.subscriptionUrl = existing.subscriptionUrl;
	return out;
}

function finalizeDns(dns) {
	if (!dns) return { dns: undefined, dehydrated: 0 };

	let dehydrated = 0;
	if (dns.domains?.length) {
		const { domains, removed } = dehydrateDomainList(dns.domains);
		dehydrated = removed.length;
		dns = { ...dns, domains };
	}

	const hasDomains = (dns.domains?.length ?? 0) > 0;
	const hasSubnets = (dns.subnets?.length ?? 0) > 0;
	const hasSub = !!dns.subscriptionUrl;
	if (!hasDomains && !hasSubnets && !hasSub) return { dns: undefined, dehydrated };
	if (!hasDomains) delete dns.domains;
	if (!hasSubnets) delete dns.subnets;
	return { dns, dehydrated };
}

function applyDehydrate(preset) {
	if (!preset.engines?.dns?.domains?.length) return null;
	const before = preset.engines.dns.domains.length;
	const { dns, dehydrated } = finalizeDns({ ...preset.engines.dns });
	if (dehydrated === 0) return null;
	if (dns) preset.engines.dns = dns;
	else delete preset.engines.dns;
	return {
		id: preset.id,
		action: 'dehydrate',
		domains: `${before} -> ${dns?.domains?.length ?? 0}`,
		removedDomains: dehydrated,
	};
}

function applyStrip(preset, byId) {
	if (!preset.covers?.length || !preset.engines?.dns) return null;
	const covered = coveredDnsUnion(preset, byId);
	const beforeD = preset.engines.dns.domains?.length ?? 0;
	const beforeS = preset.engines.dns.subnets?.length ?? 0;
	const stripped = stripCoveredLists(
		preset.engines.dns.domains,
		preset.engines.dns.subnets,
		covered,
	);
	const { dns: next, dehydrated } = finalizeDns({
		...preset.engines.dns,
		domains: stripped.domains,
		subnets: stripped.subnets,
	});
	const afterD = next?.domains?.length ?? 0;
	const afterS = next?.subnets?.length ?? 0;
	if (beforeD === afterD && beforeS === afterS && dehydrated === 0) return null;
	if (next) preset.engines.dns = next;
	else delete preset.engines.dns;
	return {
		id: preset.id,
		action: 'strip-covered',
		domains: `${beforeD} -> ${afterD}`,
		subnets: `${beforeS} -> ${afterS}`,
		removedDomains: beforeD - afterD,
		dehydrated,
	};
}

const presets = JSON.parse(fs.readFileSync(DEFAULTS_PATH, 'utf8'));
const byId = indexPresets(presets);
const changes = [];

// Pass 1: strip covered duplicates from every composite preset (existing inline DNS).
for (const preset of presets) {
	const ch = applyStrip(preset, byId);
	if (ch) changes.push(ch);
}

if (!TRIM_ONLY) {
	// Pass 2: merge decompiled parent rulesets, then strip covered children again.
	for (const preset of presets) {
		if (!preset.engines?.singbox) continue;
		if (SKIP_IDS.has(preset.id)) continue;

		const existing = preset.engines.dns;
		if (existing?.subscriptionUrl) continue;

		const extracted = findDecompiledForPreset(preset);
		if (!extracted || (!extracted.domains.length && !extracted.subnets.length)) continue;

		let next = buildDns(existing, extracted);
		if (preset.covers?.length) {
			const covered = coveredDnsUnion(preset, byId);
			const stripped = stripCoveredLists(next.domains, next.subnets, covered);
			next = { ...next, ...stripped };
		}

		const beforeD = existing?.domains?.length ?? 0;
		const beforeS = existing?.subnets?.length ?? 0;
		const { dns: polished, dehydrated } = finalizeDns(next);
		next = polished;

		if ((next?.domains?.length ?? 0) > MAX_DOMAINS) {
			changes.push({ id: preset.id, skip: `domains ${next.domains.length} > ${MAX_DOMAINS}` });
			continue;
		}

		const afterD = next?.domains?.length ?? 0;
		const afterS = next?.subnets?.length ?? 0;
		if (next && beforeD === afterD && beforeS === afterS && dehydrated === 0 && existing) continue;

		if (next) preset.engines.dns = next;
		else delete preset.engines.dns;

		changes.push({
			id: preset.id,
			action: 'decompile',
			domains: `${beforeD} -> ${afterD}`,
			subnets: `${beforeS} -> ${afterS}`,
			dehydrated,
		});
	}
}

// Pass 3: dehydrate any preset lists not touched above (idempotent cleanup).
for (const preset of presets) {
	const ch = applyDehydrate(preset);
	if (ch) changes.push(ch);
}

console.log(JSON.stringify(changes, null, 2));

if (!DRY && changes.some((c) => !c.skip)) {
	fs.writeFileSync(DEFAULTS_PATH, `${JSON.stringify(presets, null, 2)}\n`);
	console.error(`updated ${DEFAULTS_PATH}`);
}
