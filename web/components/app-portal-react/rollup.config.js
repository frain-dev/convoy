import babel from '@rollup/plugin-babel';
import external from 'rollup-plugin-peer-deps-external';
import image from '@rollup/plugin-image';
import del from 'rollup-plugin-delete';
import pkg from './package.json';
import scss from 'rollup-plugin-scss';

export default {
	input: pkg.source,
	output: [
		{ file: pkg.main, format: 'cjs' },
		{ file: pkg.module, format: 'esm' }
	],
	plugins: [
		external(),
		babel({
			exclude: 'node_modules/**',
			babelHelpers: 'bundled'
		}),
		del({ targets: ['dist/*'] }),
		image(),
		scss()
	],
	external: Object.keys(pkg.peerDependencies || {})
};
