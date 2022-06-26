/** @type {import('tailwindcss').Config} */

function customSpacing() {
	const maxSpace = 384;
	// const minSpace = 2;
	const spaces = {};

	for (let i = 2; i <= maxSpace; ) {
		const value = i + 'px';
		spaces[value] = value;
		i = i + 2;
	}

	return spaces;
}

module.exports = {
	mode: 'jit',
	purge: ['./src/**/*.{html,ts}'],
	content: ['./src/**/*.{html,ts}'],
	theme: {
		extend: {
			spacing: customSpacing()
		},
		screens: {
			desktop: '1050px'
		}
	},
	plugins: []
};
