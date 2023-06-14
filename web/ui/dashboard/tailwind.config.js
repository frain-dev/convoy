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
	safelist: [
		'bg-success-500',
		'text-success-100',
		'text-success-400',
		'text-primary-400',
		'bg-success-50',
		'bg-primary-50',
		'bg-warning-500',
		'text-warning-100',
		'bg-danger-500',
		'text-danger-100',
		'text-grey-40',
		'bg-grey-10',
		'bg-primary-400',
		'bg-danger-100',

		'text-new.primary-400',
		'bg-new.primary-25',
		'bg-new.gray-50',
		'text-new.error-400',
		'bg-new.success-25',
		'bg-new.success-50',
		'text-new.success-400',
		'bg-new.primary-50',
		'bg-new.error-50',
		'text-new.gray-400'
	],
	theme: {
		extend: {
			spacing: customSpacing(),
			boxShadow: {
				sm: '0px 2px 8px rgba(12, 26, 75, 0.08), 0px 3px 8px -1px rgba(50, 50, 71, 0.05)',
				default: '0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08)',
				lg: '0px 4px 8px rgba(12, 26, 75, 0.1), 0px 10px 16px rgba(20, 37, 63, 0.06)',
				xl: '0px 8px 16px rgba(12, 26, 75, 0.1), 0px 20px 24px rgba(20, 37, 63, 0.06)',
				'2xl': '0px 16px 16px rgba(12, 26, 75, 0.05), 0px 30px 40px rgba(20, 37, 63, 0.08)',
				'focus--primary': '0px 0px 0px 4px #FFFBFA',
				'focus--success': '0px 0px 0px 4px #F6FEF9',
				'focus--warning': '0px 0px 0px 4px #FFFCF5',
				'focus--error': '0px 0px 0px 4px #FFFBFA'
			},
			fontFamily: {
				sans: ['Inter', ...defaultTheme.fontFamily.sans],
				menlo: ['Menlo Regular', ...defaultTheme.fontFamily.sans]
			},
			backgroundImage: {
				'gradient-radial': 'radial-gradient(white 10%, #fafafe78)'
			}
		},
		fontWeight: {
			thin: '100',
			extralight: '100',
			light: '200',
			normal: '300',
			medium: '400',
			semibold: '500',
			bold: '600',
			extrabold: '700'
		},
		screens: {
			desktop: { max: '1050px' }
		},
		borderRadius: {
			'4px': '4px',
			'8px': '8px',
			'12px': '12px',
			'16px': '16px',
			'22px': '22px',
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
			'new.primary': {
				25: '#EDF2F7',
				50: '#DAE5F0',
				100: '#B5CBE1',
				200: '#91B1D1',
				300: '#6699CC',
				400: '#477DB3',
				500: '#2E6399',
				600: '#194D80',
				700: '#0A3866',
				800: '#00264D',
				900: '#001A33,'
			},
			'new.success': {
				25: '#F6FEF9',
				50: '#ECFDF3',
				100: '#D1FADF',
				200: '#A6F4C5',
				300: '#6CE9A6',
				400: '#32D583',
				500: '#12B76A',
				600: '#039855',
				700: '#027A48',
				800: '#05603A,',
				900: '#054F31,'
			},
			'new.error': {
				25: '#FFFBFA',
				50: '#FEF3F2',
				100: '#FEE4E2',
				200: '#FECDCA',
				300: '#FDA29B',
				400: '#F97066',
				500: '#F04438',
				600: '#D92D20',
				700: '#B42318',
				800: '#912018,',
				900: '#7A271A,'
			},
			'new.warning': {
				25: '#FFFCF5',
				50: '#FFFAEB',
				100: '#FEF0C7',
				200: '#FEDF89',
				300: '#FEC84B',
				400: '#FDB022',
				500: '#F79009',
				600: '#DC6803',
				700: '#B54708',
				800: '#93370D,',
				900: '#7A2E0E,'
			},
			'new.gray': {
				25: '#FCFCFD',
				50: '#F9FAFB',
				100: '#F2F4F7',
				200: '#E4E7EC',
				300: '#D0D5DD',
				400: '#98A2B3',
				500: '#667085',
				600: '#475467',
				700: '#344054',
				800: '#1D2939',
				900: '#101828'
			},
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
	plugins: [require('@tailwindcss/container-queries')]
};
