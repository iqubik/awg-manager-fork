<script lang="ts" generics="T extends LayoutViewMode">
	import SegmentedControl from './SegmentedControl.svelte';
	import type { SegmentedOption } from './segmentedControl';
	import type { LayoutViewDense, LayoutViewMode } from './layoutViewToggle';

	interface Props {
		value: T;
		onchange: (next: T) => void;
		ariaLabel?: string;
		/** Значение сегмента «мелкая сетка»: AWG — `cards`, sing-box — `dense`. */
		denseValue?: LayoutViewDense;
		/** Скрыть «список» (базовый уровень, вкладка участников подписки). */
		showListOption?: boolean;
		/** Скрыть «мелкую сетку» (вкладка участников подписки). */
		showDenseOption?: boolean;
	}

	let {
		value,
		onchange,
		ariaLabel = 'Вид списка',
		denseValue = 'dense',
		showListOption = true,
		showDenseOption = true,
	}: Props = $props();

	const options = $derived.by((): SegmentedOption<T>[] => {
		const items: SegmentedOption<T>[] = [];
		if (showDenseOption) {
			items.push({ value: denseValue as T, label: 'Мелкая сетка', icon: 'dense' });
		}
		items.push({ value: 'compact' as T, label: 'Сетка', icon: 'compact' });
		if (showListOption) {
			items.push({ value: 'list' as T, label: 'Список', icon: 'list' });
		}
		return items;
	});
</script>

<SegmentedControl variant="icon" {value} {options} {ariaLabel} onchange={onchange} />
