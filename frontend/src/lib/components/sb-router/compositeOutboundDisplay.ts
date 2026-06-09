import type { SingboxRouterOutbound, Subscription } from '$lib/types';
import type { OutboundGroup } from '$lib/components/routing/singboxRouter/outboundOptions';
import { resolveMemberLabel } from '$lib/utils/memberLabel';
import { outboundDisplay as compositeTitle } from './outboundLabel';

export const COMPOSITE_OUTBOUND_TYPES = new Set(['selector', 'urltest', 'loadbalance']);

export type CompositeOutboundType = 'selector' | 'urltest' | 'loadbalance';

export interface CompositeMemberDisplay {
	compositeType: CompositeOutboundType;
	groupTitle: string;
	memberLabels: string[];
	memberTitles: string[];
}

function memberTagsFor(
	ob: SingboxRouterOutbound | undefined,
	subscriptions: Subscription[] | null | undefined,
	tag: string,
): string[] {
	if (ob?.outbounds?.length) return ob.outbounds;
	const sub = subscriptions?.find((s) => s.selectorTag === tag);
	if (sub?.memberTags?.length) return sub.memberTags;
	return sub?.members?.map((m) => m.tag).filter(Boolean) ?? [];
}

function labelMembers(
	members: string[],
	subscriptions: Subscription[] | null | undefined,
	outboundOptions: OutboundGroup[],
): { labels: string[]; titles: string[] } {
	const labels = members.map((tag) => resolveMemberLabel(tag, subscriptions, outboundOptions));
	return { labels, titles: members };
}

function subscriptionCompositeType(sub: Subscription): CompositeOutboundType {
	return sub.mode === 'urltest' ? 'urltest' : 'selector';
}

/**
 * Раскрывает composite outbound в список человекочитаемых меток участников
 * для простого режима (RuleCard). Возвращает null, если тег не composite.
 */
export function resolveCompositeMemberDisplay(
	tag: string,
	outbounds: SingboxRouterOutbound[],
	outboundOptions: OutboundGroup[],
	subscriptions: Subscription[] | null | undefined,
): CompositeMemberDisplay | null {
	const ob = outbounds.find((o) => o.tag === tag);

	if (ob && !COMPOSITE_OUTBOUND_TYPES.has(ob.type)) return null;

	if (ob && COMPOSITE_OUTBOUND_TYPES.has(ob.type)) {
		const members = memberTagsFor(ob, subscriptions, tag);
		if (members.length === 0) return null;
		const { labels, titles } = labelMembers(members, subscriptions, outboundOptions);
		return {
			compositeType: ob.type as CompositeOutboundType,
			groupTitle: compositeTitle(ob, subscriptions).title,
			memberLabels: labels,
			memberTitles: titles,
		};
	}

	const sub = subscriptions?.find((s) => s.selectorTag === tag);
	if (!sub) return null;

	const members = memberTagsFor(undefined, subscriptions, tag);
	if (members.length === 0) return null;

	const { labels, titles } = labelMembers(members, subscriptions, outboundOptions);
	return {
		compositeType: subscriptionCompositeType(sub),
		groupTitle: sub.label || tag,
		memberLabels: labels,
		memberTitles: titles,
	};
}
