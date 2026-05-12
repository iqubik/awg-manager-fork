// Launches the backend-only mock stack: Prism (8080) + mock-proxy (8081).
// Use together with `yarn dev:mock`, which points Vite to the proxy.

import { spawn } from 'node:child_process';

const children = [];
const useDynamic = process.argv.includes('--dynamic');

function start(name, cmd, args, env = {}) {
	const child = spawn(cmd, args, {
		stdio: ['ignore', 'inherit', 'inherit'],
		env: { ...process.env, ...env },
	});
	child.on('exit', (code, signal) => {
		console.log(`[${name}] exited (code=${code} signal=${signal})`);
		shutdown();
	});
	children.push({ name, child });
	return child;
}

function shutdown() {
	for (const { child } of children) {
		if (!child.killed) child.kill('SIGTERM');
	}
	setTimeout(() => process.exit(0), 200);
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

const prismArgs = [
	'-y', '@stoplight/prism-cli', 'mock',
	'../internal/openapi/swagger.yaml',
	'-p', '8080', '--host', '127.0.0.1',
];

if (useDynamic) {
	prismArgs.push('--dynamic');
}

start('prism', 'npx', prismArgs);

setTimeout(() => {
	start('proxy', 'node', ['scripts/mock-proxy.mjs'], {
		PORT: '8081',
		UPSTREAM: 'http://127.0.0.1:8080',
	});
}, 1500);
