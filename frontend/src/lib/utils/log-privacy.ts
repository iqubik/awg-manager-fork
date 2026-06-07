import type { LogEntry } from '$lib/types';

const ipv4WithOptionalPortRe = /\b(?:\d{1,3}\.){3}\d{1,3}(?::\d{1,5})?\b/g;
const bracketedIpv6WithOptionalPortRe = /\[[0-9A-Fa-f:]+\](?::\d{1,5})?/g;
const plainIpv6Re = /\b(?:[0-9A-Fa-f]{0,4}:){2,}[0-9A-Fa-f]{0,4}(?:%[A-Za-z0-9_.-]+)?\b/g;
const domainWithOptionalPortRe = /\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{1,62}(?::\d{1,5})?\b/gi;

function splitHostPort(value: string): { host: string; port: string } {
  const idx = value.lastIndexOf(':');
  if (idx <= 0 || idx === value.length - 1) return { host: value, port: '' };
  const port = value.slice(idx + 1);
  if (!/^\d+$/.test(port)) return { host: value, port: '' };
  return { host: value.slice(0, idx), port: value.slice(idx) };
}

function maskToken(value: string): string {
  const chars = Array.from(value);
  if (chars.length <= 4) return '*'.repeat(chars.length);
  return `${chars.slice(0, 2).join('')}${'*'.repeat(chars.length - 4)}${chars.slice(-2).join('')}`;
}

function maskHostPort(value: string): string {
  const { host, port } = splitHostPort(value);
  return `${maskToken(host)}${port}`;
}

function maskBracketedIpv6(value: string): string {
  const end = value.indexOf(']');
  if (end <= 1) return value;
  const host = value.slice(1, end);
  const rest = value.slice(end + 1);
  return `[${maskToken(host)}]${rest}`;
}

function maskPlainIpv6(value: string): string {
  if (!value.includes(':')) return value;
  const colonCount = (value.match(/:/g) ?? []).length;
  if (colonCount < 3 && !value.includes('::')) return value;
  return maskToken(value);
}

export function sanitizeLogText(value: string): string {
  if (!value) return value;
  return value
    .replace(bracketedIpv6WithOptionalPortRe, maskBracketedIpv6)
    .replace(plainIpv6Re, maskPlainIpv6)
    .replace(ipv4WithOptionalPortRe, maskHostPort)
    .replace(domainWithOptionalPortRe, maskHostPort);
}

export function sanitizeLogEntry(log: LogEntry): LogEntry {
  const target = sanitizeLogText(log.target);
  const message = sanitizeLogText(log.message);
  if (target === log.target && message === log.message) return log;
  return { ...log, target, message };
}
