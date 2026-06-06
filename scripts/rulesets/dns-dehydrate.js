/**
 * Drop redundant subdomains within a single preset domain list.
 * If `google.com` is present, `mail.google.com` is removed.
 *
 * Skips geoip:/CIDR entries; bare zone labels (`ru`, `com`) are never parents.
 */

const CIDR_IN_DOMAINS = /^[\d.:a-fA-F]+\/\d+$/;

function isProtectedDomainEntry(entry) {
	const s = String(entry).trim();
	if (!s) return true;
	if (s.startsWith('geoip:')) return true;
	if (CIDR_IN_DOMAINS.test(s)) return true;
	return false;
}

function normalizeHost(entry) {
	return String(entry).trim().toLowerCase().replace(/^\./, '');
}

function isDehydrationParent(parent) {
	return parent.includes('.');
}

function isStrictSubdomainOf(child, parent) {
	if (child === parent) return false;
	return child.endsWith(`.${parent}`);
}

/** @returns {{ domains: string[], removed: Array<{ domain: string, coveredBy: string }> }} */
export function dehydrateDomainList(domains) {
	if (!domains?.length) {
		return { domains: domains ?? [], removed: [] };
	}

	const protectedEntries = [];
	const hostnames = [];

	for (const raw of domains) {
		if (isProtectedDomainEntry(raw)) protectedEntries.push(raw);
		else hostnames.push({ raw, norm: normalizeHost(raw) });
	}

	const normSet = new Set(hostnames.map((h) => h.norm));
	const parents = [...normSet].sort((a, b) => b.length - a.length);

	const removedNorm = new Set();
	const removedDetail = [];

	for (const { raw, norm } of hostnames) {
		for (const parent of parents) {
			if (parent === norm) continue;
			if (!isDehydrationParent(parent)) continue;
			if (isStrictSubdomainOf(norm, parent) && normSet.has(parent)) {
				removedNorm.add(norm);
				removedDetail.push({ domain: raw, coveredBy: parent });
				break;
			}
		}
	}

	const keptHostnames = hostnames.filter((h) => !removedNorm.has(h.norm)).map((h) => h.raw);

	return {
		domains: [...protectedEntries, ...keptHostnames],
		removed: removedDetail,
	};
}
