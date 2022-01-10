export default {
	// Target: https://go.nuxtjs.dev/config-target
	target: 'static',

	// Global page headers: https://go.nuxtjs.dev/config-head
	head: {
		title: 'Convoy',
		htmlAttrs: {
			lang: 'en'
		},
		meta: [
			{ charset: 'utf-8' },
			{
				name: 'viewport',
				content: 'width=device-width, initial-scale=1'
			},
			{ hid: 'description', name: 'description', content: 'A Cloud native Webhook Service with out-of-the-box security, reliability and scalability for your webhooks infrastructure.' },
			{
				hid: 'keywords',
				name: 'keywords',
				keywords: ['Convoy', 'Webhook', 'Webhooks', 'open-source', 'open source', 'dev tools', 'dev tool']
			},
			{
				hid: 'og:title',
				property: 'og:title',
				content: 'Convoy'
			},
			{
				hid: 'twitter:title',
				property: 'twitter:title',
				content: 'Convoy'
			},
			{
				hid: 'og:url',
				property: 'og:url',
				content: 'https://getconvoy.io/'
			},
			{
				hid: 'twitter:url',
				property: 'twitter:url',
				content: 'https://getconvoy.io/'
			},
			{
				hid: 'og:image',
				property: 'og:image',
				content: 'https://getconvoy.io/assets/convoy.png'
			},
			{
				hid: 'twitter:image',
				property: 'twitter:image',
				content: 'https://getconvoy.io/assets/convoy.png'
			},
			{
				hid: 'og:description',
				property: 'og:description',
				content: 'A Cloud native Webhook Service with out-of-the-box security, reliability and scalability for your webhooks infrastructure.'
			},
			{
				hid: 'twitter:description',
				property: 'twitter:description',
				content: 'A Cloud native Webhook Service with out-of-the-box security, reliability and scalability for your webhooks infrastructure.'
			},
			{
				hid: 'og:image:width',
				property: 'og:image:width',
				content: '437'
			},
			{
				hid: 'og:image:height',
				property: 'og:image:height',
				content: '182'
			},
			{
				hid: 'og:image:type',
				property: 'og:image:type',
				content: 'img/png'
			},
			{
				hid: 'twitter:image:alt',
				name: 'twitter:image:alt',
				content: 'Convoy Logo'
			},
			{
				hid: 'twitter:card',
				name: 'twitter:card',
				content: 'summary_large_image'
			},
			{
				hid: 'og:type',
				name: 'og:type',
				content: 'website'
			}
		],
		link: [
			{ rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
			{ rel: 'canonical', href: 'https://getconvoy.io' }
		]
	},

	// Global CSS: https://go.nuxtjs.dev/config-css
	css: ['@/scss/main.scss'],

	// Plugins to run before rendering page: https://go.nuxtjs.dev/config-plugins
	plugins: ['~/plugins/date.js'],

	// Auto import components: https://go.nuxtjs.dev/config-components
	components: true,

	// Modules for dev and build (recommended): https://go.nuxtjs.dev/config-modules
	buildModules: [],

	// Modules: https://go.nuxtjs.dev/config-modules
	modules: [
		// https://go.nuxtjs.dev/content
		'@nuxt/content'
	],

	// Content module configuration: https://go.nuxtjs.dev/config-content
	content: {
		markdown: {
			prism: {
				theme: false
			}
		},
		liveEdit: false
	},

	generate: {
		async routes() {
			const { $content } = require('@nuxt/content');
			const files = await $content({ deep: true }).only(['path']).fetch();
			return files.map(file => (file.path === '/index' ? '/' : file.path));
		}
	},

	env: {
		url: process.env.NODE_ENV === 'production' ? process.env.URL || 'http://getconvoy.io' : 'http://localhost:3000',
		lang: 'en-US'
	},

	// Build Configuration: https://go.nuxtjs.dev/config-build
	build: {},
	runtimeCompiler: true
};
