import { describe, expect, it } from 'vitest';
import {
	pluralForm,
	pluralize,
	formatRunningSub,
	ERROR_WORDS,
	RULE_WORDS,
	SUBSCRIPTION_WORDS,
	TEMPLATE_WORDS,
} from './pluralize';

describe('pluralForm', () => {
	it('rules: 1, 2–4, 5+, teens', () => {
		expect(pluralForm(1, RULE_WORDS)).toBe('правило');
		expect(pluralForm(2, RULE_WORDS)).toBe('правила');
		expect(pluralForm(4, RULE_WORDS)).toBe('правила');
		expect(pluralForm(5, RULE_WORDS)).toBe('правил');
		expect(pluralForm(11, RULE_WORDS)).toBe('правил');
		expect(pluralForm(21, RULE_WORDS)).toBe('правило');
		expect(pluralForm(22, RULE_WORDS)).toBe('правила');
	});

	it('templates', () => {
		expect(pluralize(1, TEMPLATE_WORDS)).toBe('1 шаблон');
		expect(pluralize(3, TEMPLATE_WORDS)).toBe('3 шаблона');
		expect(pluralize(12, TEMPLATE_WORDS)).toBe('12 шаблонов');
	});

	it('errors', () => {
		expect(pluralize(1, ERROR_WORDS)).toBe('1 ошибка');
		expect(pluralize(2, ERROR_WORDS)).toBe('2 ошибки');
		expect(pluralize(5, ERROR_WORDS)).toBe('5 ошибок');
	});

	it('running sub line', () => {
		expect(formatRunningSub(4, 5)).toBe('в работе 4 · остановлено 1');
	});

	it('subscriptions after number', () => {
		expect(pluralForm(1, SUBSCRIPTION_WORDS)).toBe('подписка');
		expect(pluralForm(3, SUBSCRIPTION_WORDS)).toBe('подписки');
		expect(pluralForm(5, SUBSCRIPTION_WORDS)).toBe('подписок');
		expect(pluralForm(12, SUBSCRIPTION_WORDS)).toBe('подписок');
	});
});
