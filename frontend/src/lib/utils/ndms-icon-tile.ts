/** NDMS DNS / policy icon tile — same geometry as IconTile / DnsRouteCard. */
export const NDMS_ICON_TILE_SIZE = 36;
export const NDMS_ICON_TILE_RADIUS = 6;

/** CSS clip-path for NDMS tile corners (matches border-radius). */
export const NDMS_ICON_TILE_CLIP_PATH = `inset(0 round ${NDMS_ICON_TILE_RADIUS}px)`;

/** Stroke / Lucide glyph scale inside the tile (PresetIcon brand art ≈ 0.56). */
export const NDMS_ICON_INNER_SCALE = 0.56;

export function ndmsIconTileInnerSize(
	tileSize: number = NDMS_ICON_TILE_SIZE,
	innerScale: number = NDMS_ICON_INNER_SCALE,
): number {
	return Math.round(tileSize * innerScale);
}
