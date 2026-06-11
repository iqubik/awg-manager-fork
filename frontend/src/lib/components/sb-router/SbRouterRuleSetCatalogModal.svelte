<script lang="ts">
  import { ServiceCatalogModal } from '$lib/components/dnsroutes';
  import { presetCatalog } from '$lib/stores/presets';
  import { singboxRouterCatalogPresetFilter } from '$lib/utils/catalog-preset';
  import type { CatalogPreset } from '$lib/types';
  import { fullyAddedPresetNames } from './rulesetCatalogActions';

  interface Props {
    open: boolean;
    existingRuleSetTags: string[];
    submitting?: boolean;
    onclose: () => void;
    onconfirm: (presets: CatalogPreset[]) => void;
  }

  let {
    open = false,
    existingRuleSetTags,
    submitting = false,
    onclose,
    onconfirm,
  }: Props = $props();

  const existingNames = $derived(
    fullyAddedPresetNames($presetCatalog, new Set(existingRuleSetTags)),
  );
</script>

<ServiceCatalogModal
  {open}
  title="Каталог наборов"
  presetFilter={singboxRouterCatalogPresetFilter}
  footer="none"
  multiple
  warnLargeDnsLists={false}
  markExisting
  {existingNames}
  confirmLabel="Добавить наборы"
  {submitting}
  {onclose}
  {onconfirm}
/>
