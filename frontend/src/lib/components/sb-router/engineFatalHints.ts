// Карта известных FATAL-строк sing-box → человекочитаемая подсказка.
// Первое совпадение выигрывает. Для неизвестных строк подсказки нет —
// показывается только сырой вывод sing-box.

export interface FatalHintPattern {
	match: RegExp;
	hint: string;
}

export const FATAL_HINT_PATTERNS: FatalHintPattern[] = [
	{
		match: /Address Filter Fields/i,
		hint:
			'DNS-правило ссылается на IP-набор (address-filter). Уберите IP-наборы ' +
			'(например cloudflare_2) из DNS-маршрутов — они работают только в IP-маршрутизации.',
	},
];

export function engineFatalHint(raw: string | null | undefined): string | null {
	if (!raw) return null;
	for (const p of FATAL_HINT_PATTERNS) {
		if (p.match.test(raw)) return p.hint;
	}
	return null;
}
