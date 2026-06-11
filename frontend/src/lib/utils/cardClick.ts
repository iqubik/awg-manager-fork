/** Zones where a click should not trigger the parent card/row navigation. */
const CARD_NESTED_ZONE_SELECTOR =
	'.actions,.chart-section,.lc-traffic,.lc-ping,.lc-actions';

/**
 * Returns true when the event originated from an interactive child of a clickable card/row.
 * Compares [role="button"] against currentTarget so the card itself is not treated as nested.
 */
export function isCardNestedInteraction(e: Event): boolean {
	const target = e.target;
	const currentTarget = e.currentTarget;
	// SVG icons inside action buttons are SVGElement, not HTMLElement.
	if (!(target instanceof Element) || !(currentTarget instanceof Element)) return false;

	if (target.closest('button,a,input,select,textarea,label')) return true;
	if (target.closest(CARD_NESTED_ZONE_SELECTOR)) return true;

	const roleButton = target.closest('[role="button"]');
	if (roleButton && roleButton !== currentTarget) return true;

	return false;
}
