<!--
  Источник дизайна: singbox-router/project/screens/AddRuleFlow.jsx (Step 1 «Описать вручную»)
  Переиспользует общий InlineRuleListEditor (smart-list + парс + geo-пикеры).
-->
<script lang="ts">
  import { Code } from 'lucide-svelte';
  import InlineRuleListEditor from '$lib/components/routing/singboxRouter/InlineRuleListEditor.svelte';
  import { wizardCustom, updateCustomField } from './addWizardStore';

  // svelte-ignore state_referenced_locally
  let value = $state($wizardCustom.rulesList);
  $effect(() => { updateCustomField('rulesList', value); });
</script>

<details class="form">
  <summary class="summary">
    <Code size={14} color="var(--text-muted)" />
    <span>Описать вручную</span>
    <span class="meta">· домены, IP, CIDR, порты, geosite:/geoip:</span>
  </summary>
  <div class="body">
    <InlineRuleListEditor bind:value />
  </div>
</details>

<style>
  .form {
    padding: 12px;
    border-radius: var(--radius-sm);
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
  }
  .summary {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 12.5px;
    font-weight: 500;
    list-style: none;
  }
  .summary::-webkit-details-marker { display: none; }
  .meta { color: var(--text-muted); font-weight: 400; }
  .body { margin-top: 12px; }
</style>
