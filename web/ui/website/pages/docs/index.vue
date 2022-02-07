<template>
	<div>
		<nuxt-content v-if="currentRoute !== '/docs'" :document="pageData"></nuxt-content>
		<div v-else>
			<h2>Convoy as Open Source</h2>
			<p class="subtitle">Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore.</p>
			<section class="banner">
				<div>
					<h4 class="title">Title goes here</h4>
					<p>
						Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore.Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam
						nonumy eirmod tempor invidunt ut labore et dolore.
					</p>
					<a class="get-started">
						Get started
						<img src="~/assets/images/arrow-circle-right.svg" alt="arrow right" />
					</a>
				</div>
				<div class="banner-img">
					<img src="~/assets/images/doc-banner.svg" alt="banner" />
				</div>
			</section>

			<section class="cards">
				<div class="cards_item green">
					<h3>Title goes here</h3>
					<p>Add team members to your Convoy organization so everyone has access to manage webhooks across all environments</p>
					<button>
						Get started
						<img src="~/assets/images/dark-arrow-circle-right.svg" alt="arrow right" />
					</button>
				</div>
				<div class="cards_item yellow">
					<h3>Title goes here</h3>
					<p>Add team members to your Convoy organization so everyone has access to manage webhooks across all environments</p>
					<button>
						Get started
						<img src="~/assets/images/dark-arrow-circle-right.svg" alt="arrow right" />
					</button>
				</div>
				<div class="cards_item purple">
					<h3>Title goes here</h3>
					<p>Add team members to your Convoy organization so everyone has access to manage webhooks across all environments</p>
					<button>
						Get started
						<img src="~/assets/images/dark-arrow-circle-right.svg" alt="arrow right" />
					</button>
				</div>
			</section>
		</div>
	</div>
</template>

<script>
export default {
	layout: 'docs',
	async asyncData({ $content }) {
		const pageData = await $content('docs', 'index').fetch();
		return { pageData };
	},
	data() {
		return {
			currentRoute: this.$route.path
		};
	}
};
</script>

<style lang="scss" scoped>
h2 {
	font-weight: bold;
	font-size: 27px;
	line-height: 32px;
	letter-spacing: 0.01em;
	color: #000624;
	margin-bottom: 8px;
}
p.subtitle {
	font-size: 16px;
	line-height: 24px;
	color: #737a91;
	max-width: 671px;
	margin-bottom: 36px;
}
.banner {
	background: #fcfcfc;
	box-shadow: 0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08);
	border-radius: 8px;
	padding: 33px 0 0 61px;
	display: flex;
	justify-content: space-between;
	align-items: flex-end;
	h4 {
		font-weight: 600;
		font-size: 18px;
		line-height: 30px;
		color: #000624;
		margin-bottom: 16px;
	}
	p {
		font-size: 14px;
		line-height: 22px;
		color: #737a91;
		max-width: 470px;
		margin-bottom: 16px;
	}
	a {
		display: flex;
		align-items: center;
		font-weight: 500;
		font-size: 14px;
		line-height: 22px;
		color: #477db3;
		white-space: nowrap;
		margin-bottom: 49px;
		img {
			margin-left: 16px;
			height: 16px;
			width: 16px;
		}
		&:hover {
			cursor: pointer;
		}
	}
	&-img {
		max-height: 236px;
	}
}
.cards {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(333px, 1fr));
	gap: 25px;
	margin-top: 56px;
	margin-bottom: 56px;
	&_item {
		border-radius: 16px;
		padding: 32px;
		h3 {
			font-weight: 600;
			font-size: 18px;
			line-height: 30px;
			margin-bottom: 16px;
		}
		button {
			border-radius: 6px;
			font-weight: 500;
			font-size: 14px;
			line-height: 22px;
			padding: 9px 24px;
			color: #46548c;
			display: flex;
			align-items: center;
			img {
				margin-left: 11px;
				width: 16px;
				height: 16px;
			}
		}
		p {
			font-weight: 500;
			font-size: 14px;
			line-height: 24px;
			letter-spacing: 0.004em;
			color: #7987a3;
			max-width: 293px;
			margin-bottom: 24px;
		}
		&.green {
			background: #e7fbf4;

			button {
				background: #daeee7;
			}
		}
		&.yellow {
			background: #fcf5ec;
			button {
				background: #f2ebe2;
			}
		}
		&.purple {
			background: rgba(90, 83, 179, 0.1);
			button {
				background: #e0dfed;
			}
		}
	}
}
</style>
