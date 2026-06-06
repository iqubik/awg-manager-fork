export type ViewLayoutIconName = 'dense' | 'compact' | 'list';

export type SegmentedOption<V extends string = string> = {
	value: V;
	label: string;
	icon?: ViewLayoutIconName;
};
