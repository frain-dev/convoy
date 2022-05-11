<template>
	<div class="page">
		<aside>
			<div class="logo">
				<nuxt-link to="/"><img src="~/assets/images/logo.svg" alt="logo" /></nuxt-link>
				<span>Docs</span>
			</div>

			<div class="input">
				<select name="" id="" required>
					<option v-for="version in versions" :key="version" :value="version">{{ version }}</option>
				</select>
				<label for="version">Version</label>
			</div>
			<nuxt-link to="/docs"><h3>HOME</h3></nuxt-link>

			<ul>
				<li>
					<nuxt-link to="/docs/guide">Quick Start Guide</nuxt-link>
				</li>
				<li v-for="(page, index) in pages" :key="index">
					<div v-show="page.id !== 'guide' && page.id !== 'index'">
						<nuxt-link :to="'/docs/' + page.id">
							<img src="~/assets/images/angle-down-icon.svg" alt="angle right" />
							{{ page.title }}
						</nuxt-link>

						<!-- <ul v-show="page.toc.length > 0" class="" :class="{ show: currentPage == page.id }">
							<li v-for="(subpage, index) in page.toc" :key="index">
								<nuxt-link :to="{ path: '/docs/' + page.id, hash: '#' + subpage.id }">
									{{ subpage.text }}
								</nuxt-link>
							</li>
						</ul> -->
					</div>
				</li>
			</ul>
		</aside>

		<div class="main">
			<header>
				<DocsSearch />

				<div>
					<a href="https://github.com/frain-dev/convoy/" target="_blank" rel="noreferrer">
						<img src="~/assets/images/github-icon-dark.svg" alt="github icon" />
					</a>
				</div>
			</header>

			<main class="page--container" :class="{ padding: currentRoute !== '/docs' }">
				<Nuxt />
				<div class="sidemenu">
					<h4 v-show="!stringContains(currentRoute, 'sdk') && !stringContains(currentRoute, 'release') && !stringContains(currentRoute, 'api')">ON THIS PAGE</h4>
					<ul>
						<li v-for="(page, index) in pages" :key="index">
							<ul v-show="page.toc.length > 0 && stringContains(currentRoute, page.id)">
								<li class="sub-menu" v-for="(subpage, index) in page.toc" :key="index" @click="currentSubPage = subpage.id">
									<img src="~/assets/images/arrow-right.svg" alt="angle right" :class="{ show: currentSubPage === subpage.id }" />
									<nuxt-link :to="{ path: '/docs/' + page.id, hash: '#' + subpage.id }">
										{{ subpage.text }}
									</nuxt-link>
								</li>
							</ul>
						</li>
					</ul>
				</div>
			</main>
		</div>
	</div>
</template>

<script>
export default {
	data() {
		return {
			pages: [],
			currentPage: '',
			currentSubPage: '',
			versions: ['Latest v0.5.x']
		};
	},
	computed: {
		currentRoute() {
			return this.$route.path;
		}
	},
	async mounted() {
		let pages = await this.$content('docs').only(['title', 'id', 'toc', 'order']).sortBy('order', 'asc').fetch();
		pages = pages.sort((a, b) => a.order - b.order);
		this.pages = pages;
	},
	methods: {
		stringContains(text, word) {
			return text.includes(word);
		}
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;
body,
html {
	padding: 0;
}
.page {
	display: flex;
	height: 100vh;
}

aside {
	max-width: 270px;
	width: 100%;
	background: #16192c;
	color: #ffffff;
	height: 100vh;
	overflow-y: scroll;
	.logo {
		display: flex;
		align-items: center;
		padding: 20px 24px;
		border-bottom: 1px solid rgba(236, 233, 241, 0.1);
		img {
			height: 22px;
			width: 85px;
			margin-right: 4px;
		}
		span {
			font-weight: 500;
			font-size: 16px;
			line-height: 20px;
			color: #47b38d;
		}
	}
	a {
		color: #fff;
	}

	a.nuxt-link-exact-active {
		color: #47b38d;
		h3 {
			color: inherit;
		}
	}

	h3 {
		font-weight: bold;
		font-size: 14px;
		line-height: 17px;
		font-variant: small-caps;
		color: rgba(255, 255, 255, 0.5);
		padding: 24px 0 0 24px;
		margin: 0 0 0 0;
	}

	& > ul {
		padding: 24px 0 24px 24px;

		h3 {
			padding: 0;
			margin: 0 0 16px;
		}

		li {
			font-size: 14px;
			line-height: 16px;
			margin-bottom: 30px;

			li {
				margin-bottom: 20px;
			}

			a,
			button {
				display: flex;
				align-items: center;
			}

			img {
				width: 16px;
				margin-right: 10px;
			}
		}

		ul {
			margin: 16px 0 16px 40px;
		}
	}
}
a.api-reference {
	img {
		transform: rotate(270deg);
		margin-left: 5px;
		filter: brightness(0) invert(1);
	}
}
.main {
	width: 100%;
	overflow-y: auto;
	padding-bottom: 100px;
	header {
		position: fixed;
		width: 100%;
		@media (min-width: $desktopBreakPoint) {
			width: calc(100% - 270px);
		}
	}
}

header {
	padding: 13px 24px;
	background: #ffffff;
	display: flex;
	align-items: center;
	justify-content: space-between;
}

.page--container {
	padding: 36px 20px;
	max-width: 100%;
	width: 100%;
	margin: auto;

	@media (min-width: $desktopBreakPoint) {
		display: flex;
		padding: 36px 48px;
	}
	.nuxt-content {
		width: 100%;
		margin-top: 70px;
		@media (min-width: $desktopBreakPoint) {
			width: calc(100% - 300px);
		}
	}
}
.sidemenu {
	min-width: 250px;
	margin-top: 30px;
	@media (min-width: $desktopBreakPoint) {
		margin-top: unset;
		position: fixed;
		right: 60px;
		top: 110px;
	}
	h4 {
		font-weight: 600;
		font-size: 14px;
		line-height: 22px;
		color: #000624;
		padding-left: 10px;
		margin-bottom: 8px;
	}
	li {
		&.sub-menu {
			font-weight: 500;
			font-size: 14px;
			line-height: 22px;
			color: #737a91;
			padding: 8px 0;
			border-bottom: 1px solid #edeff5;

			img {
				visibility: hidden;
				height: 8px;
				width: 8px;
				&.show {
					visibility: unset;
				}
			}
		}
	}
}
</style>
