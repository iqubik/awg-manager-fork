/**
 * Настройки пухососа на странице «Настройки».
 *
 * Слои скорости:
 * 1. Патруль (PukhososPatrol) — движение по горизонтали, scale.
 * 2. Спрайт (PukhososSprite) — кадры в спрайтмапе (ходьба, разворот).
 */

/** Ширина робота на экране, px (высота из пропорции ячейки 280×200). */
export const PUKHOSOS_SPRITE_SIZE = 104;

/** Длительность проезда от края до края, мс. CSS-transition `left`. */
export const PUKHOSOS_WALK_MS = 9400;

/** Появление / исчезновение (scale 0↔1), мс. CSS-transition `transform`. */
export const PUKHOSOS_SCALE_MS = 2200;

/**
 * После финального разворота — короткий проезд на полном размере (доля ширины трека, 0…1),
 * затем оставшийся путь с уменьшением.
 */
export const PUKHOSOS_EXIT_COAST_RATIO = 0.14;

/** Режим кадров ходьбы. */
export type PukhososWalkFrameMode = 'cycle' | 'sequence';

/** cycle — 7 кадров по кругу (CSS). sequence — псевдослучайные секвенсы (JS). */
export const PUKHOSOS_WALK_FRAME_MODE: PukhososWalkFrameMode = 'sequence';

/** Длительность цикла ходьбы, мс. Только mode === 'cycle'. */
export const PUKHOSOS_SPRITE_WALK_CYCLE_MS = 880;

/** Интервал смены кадра в mode === 'sequence', мс. */
export const PUKHOSOS_SPRITE_WALK_FRAME_MS = 120;

/** Приоритетные кадры езды (номера 1…7 в спрайтмапе). */
export const PUKHOSOS_WALK_PRIORITY_FRAMES = [1, 5, 7] as const;

/** Полный подпрыг: строго 2 → 3 → 6 → 4. */
export const PUKHOSOS_WALK_JUMP_SEQUENCE = [2, 3, 6, 4] as const;

/** Мини-подпрыг: 2→4 и 3→6, чередуются между собой. */
export const PUKHOSOS_WALK_MINI_JUMP_A = [2, 4] as const;
export const PUKHOSOS_WALK_MINI_JUMP_B = [3, 6] as const;

/**
 * Сколько кадров езды (1/5/7) подряд перед прыжком — случайно в этом диапазоне.
 * Среднее ~12.5 при 9…16 → примерно в 10+ раз больше прыжков по длительности.
 */
export const PUKHOSOS_DRIVE_MIN_BEFORE_JUMP = 9;
export const PUKHOSOS_DRIVE_MAX_BEFORE_JUMP = 16;

function rollDrivesUntilJump() {
	const span = PUKHOSOS_DRIVE_MAX_BEFORE_JUMP - PUKHOSOS_DRIVE_MIN_BEFORE_JUMP + 1;
	return PUKHOSOS_DRIVE_MIN_BEFORE_JUMP + Math.floor(Math.random() * span);
}

type WalkBlockKind = 'priority' | 'mini' | 'jump';

function frameToCol(frame1: number) {
	return frame1 - 1;
}

export type WalkFrameSequencer = {
	next: () => number;
	reset: () => void;
};

/** Псевдосеквенс: N кадров езды (1/5/7), затем мини- или полный подпрыг. */
export function createWalkFrameSequencer(): WalkFrameSequencer {
	let priorityIdx = 0;
	let drivesUntilJump = rollDrivesUntilJump();
	let miniJumpToggle = false;
	let seqStep = 0;
	let activeBlock: WalkBlockKind | null = null;

	const scheduleNextBlock = () => {
		if (drivesUntilJump > 0) {
			activeBlock = 'priority';
			drivesUntilJump--;
		} else {
			activeBlock = Math.random() < 0.5 ? 'mini' : 'jump';
			drivesUntilJump = rollDrivesUntilJump();
		}
		seqStep = 0;
	};

	const reset = () => {
		priorityIdx = 0;
		drivesUntilJump = rollDrivesUntilJump();
		miniJumpToggle = false;
		seqStep = 0;
		activeBlock = null;
	};

	const next = (): number => {
		if (activeBlock === null) {
			scheduleNextBlock();
		}

		switch (activeBlock) {
			case 'priority': {
				const frame = PUKHOSOS_WALK_PRIORITY_FRAMES[priorityIdx]!;
				priorityIdx = (priorityIdx + 1) % PUKHOSOS_WALK_PRIORITY_FRAMES.length;
				activeBlock = null;
				return frameToCol(frame);
			}
			case 'mini': {
				const seq = miniJumpToggle ? PUKHOSOS_WALK_MINI_JUMP_B : PUKHOSOS_WALK_MINI_JUMP_A;
				const frame = seq[seqStep]!;
				seqStep++;
				if (seqStep >= seq.length) {
					miniJumpToggle = !miniJumpToggle;
					activeBlock = null;
				}
				return frameToCol(frame);
			}
			case 'jump': {
				const frame = PUKHOSOS_WALK_JUMP_SEQUENCE[seqStep]!;
				seqStep++;
				if (seqStep >= PUKHOSOS_WALK_JUMP_SEQUENCE.length) {
					activeBlock = null;
				}
				return frameToCol(frame);
			}
			default:
				return 0;
		}
	};

	return { next, reset };
}

/** Сколько ждать разворот в патруле, мс (≥ SPRITE_TURN_CYCLE_MS). */
export const PUKHOSOS_TURN_MS = 820;

/** Длительность разворота в спрайтмапе, мс (8 кадров). */
export const PUKHOSOS_SPRITE_TURN_CYCLE_MS = 800;

/** Пауза между сегментами патруля, мс. */
export const PUKHOSOS_PAUSE_MS = 180;

/** Сколько раз проезжает вправо за вызов. */
export const PUKHOSOS_PASS_COUNT = 2;

/**
 * Блокировка кнопки «Едет…», мс.
 * Учитывает доп. короткий проезд после финального разворота.
 */
export const PUKHOSOS_PATROL_MS =
	PUKHOSOS_WALK_MS * (1 + 2 * (PUKHOSOS_PASS_COUNT - 1)) +
	PUKHOSOS_TURN_MS * 2 * (PUKHOSOS_PASS_COUNT - 1) +
	PUKHOSOS_PAUSE_MS * (5 + 2 * (PUKHOSOS_PASS_COUNT - 1)) +
	600;
