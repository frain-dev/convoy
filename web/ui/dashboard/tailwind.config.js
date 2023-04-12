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
				xs: '0px 2px 2px 2px rgba(12, 26, 75, 0.04)',
				sm: '0px 2px 8px rgba(12, 26, 75, 0.08), 0px 3px 8px -1px rgba(50, 50, 71, 0.05)',
				md: '0px 4px 4px rgba(12, 26, 75, 0.1), 0px 10px 10px rgba(20, 37, 63, 0.06)',
				lg: '0px 4px 8px rgba(12, 26, 75, 0.1), 0px 10px 16px rgba(20, 37, 63, 0.06)',
				xl: '0px 8px 16px rgba(12, 26, 75, 0.1), 0px 20px 24px rgba(20, 37, 63, 0.06)',
				'2xl': '0px 16px 16px rgba(12, 26, 75, 0.05), 0px 30px 40px rgba(20, 37, 63, 0.08)',
				'3xl': '0px 16px 16px rgba(12, 26, 75, 0.06), 0px 16px 16px rgba(12, 26, 75, 0.05), 0px 30px 40px rgba(20, 37, 63, 0.08)',
				default: '0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08)'
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
			'100px': '100px'
		},
		fontSize: {
			10: ['10px', '150%'],
			12: ['12px', '150%'],
			14: ['14px', '20px'],
			16: ['16px', '150%'],
			18: ['18px', '28px'],
			20: ['20px', '150%'],
			24: ['24px', '150%'],
			30: ['30px', '38px'],
			36: ['36px', '44px'],
			48: ['48px', '60px'],
			60: ['60px', '72px'],
			72: ['72px', '90px'],
			h1: ['20px', '140%'],
			h2: ['18px', '140%'],
			h3: ['16px', '140%'],
			h4: ['14px', '140%']
		},
		colors: {
			gray: {
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
				25: '#EDF2F7',
				50: '#F9F5FF',
				100: '#B5CBE1',
				200: '#91B1D1',
				300: '#6699CC',
				400: '#477DB3',
				500: '#2E6399',
				600: '#194D80',
				700: '#0A3866',
				800: '#00264D',
				900: '#001A33'
			},
			success: {
				25: '#F6FEF9',
				50: '#ECFDF3',
				100: '#D1FADF',
				200: '#A6F4C5',
				300: '#6CE9A6',
				400: '#32D583',
				500: '#12B76A',
				600: '#039855',
				700: '#027A48',
				800: '#05603A',
				900: '#054F31'
			},
			danger: {
				25: '#FFFBFA',
				50: '#FEF3F2',
				100: '#FEE4E2',
				200: '#FECDCA',
				300: '#FDA29B',
				400: '#F97066',
				500: '#F04438',
				600: '#D92D20',
				700: '#B42318',
				800: '#912018',
				900: '#7A271A'
			},
			warning: {
				25: '#FFFCF5',
				50: '#FFFAEB',
				100: '#FEF0C7',
				200: '#FEDF89',
				300: '#FEC84B',
				400: '#FDB022',
				500: '#F79009',
				600: '#DC6803',
				700: '#B54708',
				800: '#93370D',
				900: '#7A2E0E'
			},
			'blue-gray': {
				25: '#FCFCFD',
				50: '#F8F9FC',
				100: '#EAECF5',
				200: '#C8CCE5',
				300: '#9EA5D1',
				400: '#717BBC',
				500: '#4E5BA6',
				600: '#3E4784',
				700: '#363F72',
				800: '#293056',
				900: '#101323'
			},
			lightblue: {
				25: '#F5FBFF',
				50: '#F0F9FF',
				100: '#E0F2FE',
				200: '#B9E6FE',
				300: '#7CD4FD',
				400: '#36BFFA',
				500: '#0BA5EC',
				600: '#0086C9',
				700: '#026AA2',
				800: '#065986',
				900: '#0B4A6F'
			},
			blue: {
				25: '#F5FAFF',
				50: '#EFF8FF',
				100: '#D1E9FF',
				200: '#B2DDFF',
				300: '#84CAFF',
				400: '#53B1FD',
				500: '#2E90FA',
				600: '#1570EF',
				700: '#175CD3',
				800: '#1849A9',
				900: '#194185'
			},
			indigo: {
				25: '#F5F8FF',
				50: '#EEF4FF',
				100: '#E0EAFF',
				200: '#C7D7FE',
				300: '#A4BCFD',
				400: '#8098F9',
				500: '#6172F3',
				600: '#444CE7',
				700: '#3538CD',
				800: '#2D31A6',
				900: '#2D3282'
			},
			purple: {
				25: '#FAFAFF',
				50: '#F4F3FF',
				100: '#EBE9FE',
				200: '#D9D6FE',
				300: '#BDB4FE',
				400: '#9B8AFB',
				500: '#7A5AF8',
				600: '#6938EF',
				700: '#5925DC',
				800: '#4A1FB8',
				900: '#3E1C96'
			},
			pink: {
				25: '#FEF6FB',
				50: '#FDF2FA',
				100: '#FCE7F6',
				200: '#FCCEEE',
				300: '#FAA7E0',
				400: '#F670C7',
				500: '#EE46BC',
				600: '#DD2590',
				700: '#C11574',
				800: '#9E165F',
				900: '#851651'
			},
			rose: {
				25: '#FFF5F6',
				50: '#FFF1F3',
				100: '#FFE4E8',
				200: '#FECDD6',
				300: '#FEA3B4',
				400: '#FD6F8E',
				500: '#F63D68',
				600: '#E31B54',
				700: '#C01048',
				800: '#A11043',
				900: '#89123E'
			},
			orange: {
				25: '#FFFAF5',
				50: '#FFF6ED',
				100: '#FFEAD5',
				200: '#FDDCAB',
				300: '#FEB273',
				400: '#FD853A',
				500: '#FB6514',
				600: '#EC4A0A',
				700: '#C4320A',
				800: '#9C2A10',
				900: '#7E2410'
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
