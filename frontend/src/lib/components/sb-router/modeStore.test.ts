import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';

const STORAGE_KEY = 'awg.sb-router.mode';

function resetEnv(url: string) {
  // jsdom URL mock
  window.history.replaceState({}, '', url);
  localStorage.clear();
  vi.resetModules();
}

describe('modeStore', () => {
  beforeEach(() => {
    resetEnv('/');
  });

  it('defaults to beginner when no URL no storage', async () => {
    const { mode } = await import('./modeStore');
    expect(get(mode)).toBe('beginner');
  });

  it('reads expert from URL query', async () => {
    resetEnv('/?mode=expert');
    const { mode } = await import('./modeStore');
    expect(get(mode)).toBe('expert');
  });

  it('falls back to beginner on invalid URL value', async () => {
    resetEnv('/?mode=ninja');
    const { mode } = await import('./modeStore');
    expect(get(mode)).toBe('beginner');
  });

  it('reads from localStorage when URL has no mode', async () => {
    localStorage.setItem(STORAGE_KEY, 'expert');
    const { mode } = await import('./modeStore');
    expect(get(mode)).toBe('expert');
  });

  it('URL takes precedence over localStorage', async () => {
    localStorage.setItem(STORAGE_KEY, 'expert');
    resetEnv('/?mode=beginner');
    localStorage.setItem(STORAGE_KEY, 'expert');
    const { mode } = await import('./modeStore');
    expect(get(mode)).toBe('beginner');
  });

  it('setMode updates store, URL, and localStorage', async () => {
    const { mode, setMode } = await import('./modeStore');
    setMode('expert');
    expect(get(mode)).toBe('expert');
    expect(new URL(window.location.href).searchParams.get('mode')).toBe('expert');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('expert');
  });

  it('setMode preserves other URL params', async () => {
    resetEnv('/?tab=singbox&sub=engine');
    const { setMode } = await import('./modeStore');
    setMode('expert');
    const sp = new URL(window.location.href).searchParams;
    expect(sp.get('tab')).toBe('singbox');
    expect(sp.get('sub')).toBe('engine');
    expect(sp.get('mode')).toBe('expert');
  });
});
