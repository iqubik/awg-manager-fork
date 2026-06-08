<script lang="ts">
	import { tick } from 'svelte';
	import { PukhososSprite } from '$lib/components/ui';
	import type { PukhososAnimation } from '$lib/components/ui';
	import { pukhososSummonTick } from '$lib/stores/pukhososSummon';
	import {
		PUKHOSOS_EXIT_COAST_RATIO,
		PUKHOSOS_PASS_COUNT,
		PUKHOSOS_PAUSE_MS,
		PUKHOSOS_SCALE_MS,
		PUKHOSOS_SPRITE_SIZE,
		PUKHOSOS_TURN_MS,
		PUKHOSOS_WALK_MS,
	} from '$lib/utils/pukhososPatrol';

	const SPRITE_SIZE = PUKHOSOS_SPRITE_SIZE;
	const WALK_MS = PUKHOSOS_WALK_MS;
	const SCALE_SEC = PUKHOSOS_SCALE_MS / 1000;
	const TURN_MS = PUKHOSOS_TURN_MS;
	const PAUSE_MS = PUKHOSOS_PAUSE_MS;
	const PASS_COUNT = PUKHOSOS_PASS_COUNT;

	interface Props {
		/** Ширина блока футера (из bind:clientWidth на хосте). */
		trackWidth?: number;
	}

	let { trackWidth = 0 }: Props = $props();
	let visible = $state(false);
	let x = $state(0);
	let scale = $state(0);
	let moving = $state(false);
	let scaling = $state(false);
	let facing = $state<'left' | 'right'>('right');
	let animation = $state<PukhososAnimation>('idle');
	let turnNonce = $state(0);
	let moveDurationSec = $state(WALK_MS / 1000);
	let scaleDurationSec = $state(SCALE_SEC);
	let spriteEl: HTMLDivElement | undefined = $state();

	let runId = 0;
	let lastSummonTick = 0;

	$effect(() => {
		const tick = $pukhososSummonTick;
		if (tick <= lastSummonTick) return;
		lastSummonTick = tick;
		void startPatrol();
	});

	function sleep(ms: number) {
		return new Promise<void>((resolve) => setTimeout(resolve, ms));
	}

	/** Дождаться отрисовки кадра — иначе transition не сработает с начальной позиции. */
	function waitPaint() {
		return new Promise<void>((resolve) => {
			requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
		});
	}

	async function waitSprite() {
		for (let i = 0; i < 30; i++) {
			await tick();
			if (spriteEl) return;
		}
	}

	function forceReflow() {
		void spriteEl?.offsetWidth;
	}

	function waitMove(durationMs: number) {
		if (!spriteEl || !moving) return sleep(durationMs);
		return new Promise<void>((resolve) => {
			const fallback = window.setTimeout(resolve, durationMs + 120);
			const onEnd = (event: TransitionEvent) => {
				if (event.propertyName !== 'left') return;
				window.clearTimeout(fallback);
				spriteEl?.removeEventListener('transitionend', onEnd);
				resolve();
			};
			spriteEl.addEventListener('transitionend', onEnd);
		});
	}

	function waitTurn() {
		return sleep(TURN_MS);
	}

	type WalkOpts = { skipPause?: boolean; skipPaint?: boolean; scaleDurationMs?: number };

	async function walkTo(
		targetX: number,
		scaleTo?: number,
		durationMs = WALK_MS,
		opts?: WalkOpts,
	) {
		moveDurationSec = durationMs / 1000;
		const willScale = scaleTo !== undefined;
		if (willScale) {
			scaleDurationSec =
				(opts?.scaleDurationMs ?? Math.min(PUKHOSOS_SCALE_MS, durationMs)) / 1000;
		}

		if (!opts?.skipPaint) {
			moving = false;
			scaling = false;
			await tick();
			await waitPaint();
		} else {
			await tick();
		}

		// Сначала включаем transition на текущей позиции, потом меняем left/scale —
		// иначе браузер «телепортирует» в конечную точку (старт справа).
		moving = true;
		if (willScale) {
			scaling = true;
		}
		forceReflow();
		await waitPaint();

		if (willScale) {
			scale = scaleTo;
		}
		x = targetX;

		await waitMove(durationMs);
		moving = false;
		scaling = false;
		if (!opts?.skipPause) {
			await sleep(PAUSE_MS);
		}
	}

	async function ensureTrackReady(): Promise<boolean> {
		for (let i = 0; i < 40; i++) {
			if (trackWidth > SPRITE_SIZE) return true;
			await tick();
			await waitPaint();
		}
		return trackWidth > SPRITE_SIZE;
	}

	async function turnTo(nextFacing: 'left' | 'right') {
		animation = 'turn';
		facing = nextFacing;
		turnNonce += 1;
		await waitTurn();
		animation = 'walk';
		facing = nextFacing;
		await sleep(PAUSE_MS);
	}

	async function startPatrol() {
		const id = ++runId;
		if (!(await ensureTrackReady())) return;

		visible = true;
		facing = 'right';
		animation = 'walk';
		moving = false;
		x = 0;
		scale = 0;

		const maxX = Math.max(0, trackWidth - SPRITE_SIZE);

		await waitSprite();
		await waitPaint();
		forceReflow();

		await walkTo(maxX, 1);
		if (id !== runId) return;

		for (let pass = 1; pass < PASS_COUNT; pass++) {
			await turnTo('left');
			if (id !== runId) return;

			await walkTo(0);
			if (id !== runId) return;

			await turnTo('right');
			if (id !== runId) return;

			if (pass === PASS_COUNT - 1) {
				const coastX = Math.round(maxX * PUKHOSOS_EXIT_COAST_RATIO);
				if (coastX > 0 && coastX < maxX) {
					const coastMs = Math.round(WALK_MS * (coastX / maxX));
					const exitMs = WALK_MS - coastMs;
					await walkTo(coastX, undefined, coastMs, { skipPause: true });
					if (id !== runId) return;
					await walkTo(maxX, 0, exitMs, {
						skipPause: true,
						skipPaint: true,
						scaleDurationMs: exitMs,
					});
				} else {
					await walkTo(maxX, 0);
				}
				if (id !== runId) return;
			} else {
				await walkTo(maxX);
				if (id !== runId) return;
			}
		}

		visible = false;
		scale = 0;
	}
</script>

<div class="patrol-track" aria-hidden="true">
	{#if visible}
		<div
			class="patrol-sprite"
			class:moving-left={moving}
			class:moving-scale={scaling}
			bind:this={spriteEl}
			style:left="{x}px"
			style:transform="scale({scale})"
			style:--walk-dur="{moveDurationSec}s"
			style:--scale-dur="{scaleDurationSec}s"
		>
			{#key `${animation}-${turnNonce}`}
				<PukhososSprite {animation} {facing} size={SPRITE_SIZE} />
			{/key}
		</div>
	{/if}
</div>

<style>
	.patrol-track {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 0;
		pointer-events: none;
		z-index: 3;
		overflow: visible;
	}

	.patrol-sprite {
		position: absolute;
		bottom: 0;
		left: 0;
		transform-origin: bottom center;
		filter: drop-shadow(0 1px 2px rgb(0 0 0 / 0.22));
	}

	.patrol-sprite.moving-left {
		transition: left var(--walk-dur) linear;
	}

	.patrol-sprite.moving-scale {
		transition: transform var(--scale-dur) linear;
	}

	/* Оба класса вместе — иначе moving-scale затирает transition для left. */
	.patrol-sprite.moving-left.moving-scale {
		transition:
			left var(--walk-dur) linear,
			transform var(--scale-dur) linear;
	}
</style>
