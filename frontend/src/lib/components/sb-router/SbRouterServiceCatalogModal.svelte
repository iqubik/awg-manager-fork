<script lang="ts">
  import { ServiceCatalogModal } from '$lib/components/dnsroutes';
  import type { CatalogPreset } from '$lib/types';
  import { singboxRouterCatalogPresetFilter } from '$lib/utils/catalog-preset';
  import {
    templatesOpen,
    templatesSelection,
    dismissTemplatesModal,
    catalogIdsFromTemplatesSelection,
    setServiceTemplateSelection,
  } from './templatesStore';

  const initialSelectedIds = $derived(catalogIdsFromTemplatesSelection($templatesSelection));

  function handleConfirm(presets: CatalogPreset[]) {
    setServiceTemplateSelection(presets.map((p) => p.id));
    dismissTemplatesModal();
  }

  function handleClose() {
    dismissTemplatesModal();
  }
</script>

<ServiceCatalogModal
  open={$templatesOpen}
  title="Каталог сервисов"
  presetFilter={singboxRouterCatalogPresetFilter}
  footer="none"
  multiple
  warnLargeDnsLists={false}
  markExisting={false}
  {initialSelectedIds}
  onclose={handleClose}
  onconfirm={handleConfirm}
/>
