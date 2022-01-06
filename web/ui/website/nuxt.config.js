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
			{ hid: 'description', name: 'description', content: 'A Cloud native Webhook Service with out-of-the-box security, reliability and scalability for your webhooks infrastructure.' }
		],
		link: [{ rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' }]
	},

	// Global CSS: https://go.nuxtjs.dev/config-css
	css: ['@/scss/main.scss'],

	// Plugins to run before rendering page: https://go.nuxtjs.dev/config-plugins
	plugins: [],

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
