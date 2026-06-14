<script lang="ts">
  import Modal from '$lib/components/ui/Modal.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { copyToClipboard } from '$lib/utils/clipboard';
  import { engineFatalHint } from './engineFatalHints';

  interface Props {
    open: boolean;
    lastError: string;
    onclose: () => void;
  }
  let { open, lastError, onclose }: Props = $props();

  let copied = $state(false);
  const hint = $derived(engineFatalHint(lastError));

  async function copy(): Promise<void> {
    copied = await copyToClipboard(lastError);
    if (copied) setTimeout(() => (copied = false), 1500);
  }
</script>

<Modal {open} {onclose} title="Движок sing-box: ошибка запуска" size="lg">
  {#if hint}
    <p class="hint">{hint}</p>
  {/if}
  <pre class="fatal">{lastError}</pre>
  <a class="logs-link" href="/diagnostics?tab=logs">Открыть полный лог sing-box</a>

  {#snippet actions()}
    <Button variant="ghost" onclick={copy}>{copied ? 'Скопировано' : 'Копировать'}</Button>
    <Button variant="ghost" onclick={onclose}>Закрыть</Button>
  {/snippet}
</Modal>

<style>
  .hint {
    margin: 0 0 0.75rem;
    padding: 0.6rem 0.75rem;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-error, #dc2626) 12%, transparent);
    color: var(--text-primary);
    font-size: 0.85rem;
    line-height: 1.4;
  }
  .fatal {
    margin: 0;
    max-height: 40vh;
    overflow: auto;
    padding: 0.75rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-tertiary);
    color: var(--text-primary);
    font-family: var(--font-mono);
    font-size: 0.78rem;
    line-height: 1.45;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .logs-link {
    display: inline-block;
    margin-top: 0.6rem;
    font-size: 0.82rem;
    color: var(--accent);
  }
</style>
