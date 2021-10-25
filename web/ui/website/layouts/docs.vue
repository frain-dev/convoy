<template>
	<div class="page">
		<aside>
			<div class="logo">
				Convoy.
				<span>Docs</span>
			</div>

			<nuxt-link to="/docs"><h3>QUICK START GUIDE</h3></nuxt-link>

			<ul>
				<h3>Docs</h3>

				<li v-for="(page, index) in pages" :key="index" v-if="page.id !== 'welcome'">
					<nuxt-link :to="'/docs/' + page.id">
						<img src="~/assets/images/angle-down-icon.svg" alt="angle right" />
						{{ page.title }}
					</nuxt-link>

					<ul v-if="page.toc.length > 0">
						<li v-for="(subpage, index) in page.toc" :key="index">
							<nuxt-link :to="{ path: '/docs/' + page.id, hash: '#' + subpage.id }">
								{{ subpage.text }}
							</nuxt-link>
						</li>
					</ul>
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

			<main class="page--container">
				<Nuxt />
			</main>

			<footer>
				<div>
					<img src="" alt="" />
				</div>
			</footer>
		</div>
	</div>
</template>

<script>
export default {
	data: () => {
		return {
			pages: []
		};
	},
	async mounted() {
		const pages = await this.$content('docs').only(['title', 'id', 'toc']).sortBy('order', 'asc').fetch();
		this.pages = pages;
	}
};
</script>

<style lang="scss" scoped>
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

	.logo {
		font-weight: bold;
		font-size: 21px;
		line-height: 26px;
		color: #ffffff;
		padding: 20px 24px;
		border-bottom: 1px solid rgba(236, 233, 241, 0.1);

		span {
			font-weight: 500;
			font-size: 16px;
			line-height: 20px;
			color: #47b38d;
		}
	}

	a.nuxt-link-exact-active {
		color: #47b38d;
		font-weight: bold;

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

.main {
	width: 100%;
	overflow-y: auto;
	padding-bottom: 100px;
}

header {
	padding: 13px 24px;
	background: #ffffff;
	display: flex;
	align-items: center;
	justify-content: space-between;
}

.page--container {
	padding: 36px 32px;
	max-width: 900px;
	width: 100%;
	margin: auto;
}
</style>
