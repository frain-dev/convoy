const defaultTheme = require('tailwindcss/defaultTheme');

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
	content: ['./src/**/*.{html,ts}'],
	safelist: ['bg-success-500', 'text-success-100', 'bg-warning-500', 'text-warning-100', 'bg-danger-500', 'text-danger-100', 'text-grey-40', 'bg-grey-10', 'bg-primary-400', 'bg-danger-100'],
	theme: {
		extend: {
			spacing: customSpacing(),
			boxShadow: {
				sm: '0px 2px 8px rgba(12, 26, 75, 0.08), 0px 3px 8px -1px rgba(50, 50, 71, 0.05)',
				default: '0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08)',
				lg: '0px 4px 8px rgba(12, 26, 75, 0.1), 0px 10px 16px rgba(20, 37, 63, 0.06)',
				xl: '0px 8px 16px rgba(12, 26, 75, 0.1), 0px 20px 24px rgba(20, 37, 63, 0.06)',
				'2xl': '0px 16px 16px rgba(12, 26, 75, 0.05), 0px 30px 40px rgba(20, 37, 63, 0.08)'
			},
			fontFamily: {
				sans: ['Quicksand', ...defaultTheme.fontFamily.sans],
				menlo: ['Menlo Regular', ...defaultTheme.fontFamily.sans]
			},
			backgroundImage: {
				'gradient-radial': 'radial-gradient(white 10%, #fafafe78)'
			}
		},
		screens: {
			desktop: { max: '1050px' }
		},
		borderRadius: {
			'4px': '4px',
			'8px': '8px',
			'12px': '12px',
			'16px': '16px',
			'100px': '100px'
		},
		fontSize: {
			10: ['10px', '150%'],
			12: ['12px', '20px'],
			14: ['14px', '22px'],
			16: ['16px', '24px'],
			18: ['18px', '30px'],
			20: ['20px', '30px'],
			24: ['24px', '35px'],
			h1: ['20px', '140%'],
			h2: ['18px', '140%'],
			h3: ['16px', '140%'],
			h4: ['14px', '140%']
		},
		colors: {
			grey: {
				100: '#000624',
				80: '#31323D',
				60: '#5F5F68',
				40: '#737A91',
				20: '#E8E8E9',
				10: '#EDEDF5'
			},
			white: {
				100: 'rgba(var(--color-white), 1)',
				64: 'rgba(var(--color-white), 0.64)',
				40: 'rgba(var(--color-white), 0.40)',
				24: 'rgba(var(--color-white), 0.24)',
				16: 'rgba(var(--color-white), 0.16)',
				8: 'rgba(var(--color-white), 0.08)',
				4: 'rgba(var(--color-white), 0.04)'
			},
			primary: {
				100: '#477DB3',
				200: '#7EA4CA',
				300: '#A3BED9',
				400: '#C8D8E8',
				500: '#EDF2F7'
			},
			success: {
				100: '#25C26E',
				200: '#66D49A',
				300: '#92E1B7',
				400: '#BEEDD4',
				500: '#E9F9F1'
			},
			danger: {
				100: '#FF554A',
				200: '#FF8880',
				300: '#FF9992',
				400: '#FFCCC9',
				500: '#FFEEED'
			},
			warning: {
				100: '#F0AD4E',
				200: '#F3BD71',
				300: '#F6CE95',
				400: '#FBE6CA',
				500: '#FEF7ED'
			},
			secondary: '#32587D',
			purple: '#5A53B3',
			'dark-green': '#327D63',
			'light-green': '#47B38D',
			black: '#16192C',
			'dark-grey': '#B2B2B2',
			transparent: 'transparent'
		},
		animation: {
			'spin-slow': 'spin 3s linear infinite'
		}
	},
	plugins: []
};
