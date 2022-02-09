<template>
	<div>
		<div>
			<h2>Convoy as Open Source</h2>
			<p class="subtitle">Find the guides, samples, and references you need to use Convoy to start sending out and receiving webhook events as fast as possible.</p>
			<section class="banner">
				<div>
					<h4 class="title">Quick Start Guide</h4>
					<p>We've put together a short easy to follow steps to get started with deploying Convoy to your dev environment and sending out webhook events in few minutes.</p>
					<nuxt-link to="/docs/guide" class="get-started">
						Get started
						<img src="~/assets/images/arrow-circle-right.svg" alt="arrow right" />
					</nuxt-link>
				</div>
				<div class="banner-img">
					<img src="~/assets/images/doc-banner.svg" alt="banner" />
				</div>
			</section>

			<section class="cards">
				<div class="cards_item green">
					<h3>Overview</h3>
					<p>A detail breakdown of Convoy, explaining some technical terms used in this documentation.</p>
					<nuxt-link to="/docs/overview">
						<button>
							Read more
							<img src="~/assets/images/dark-arrow-circle-right.svg" alt="arrow right" />
						</button>
					</nuxt-link>
				</div>
				<div class="cards_item yellow">
					<h3>Configuration</h3>
					<p>Step by step explanation on how to configure Convoy to best suite your preference and use case.</p>
					<nuxt-link to="/docs/configuration">
						<button>
							Read more
							<img src="~/assets/images/dark-arrow-circle-right.svg" alt="arrow right" />
						</button>
					</nuxt-link>
				</div>
				<div class="cards_item purple">
					<h3>Deployment</h3>
					<p>Learn how to deploy Convoy to any environment of your choice.</p>
					<br />
					<nuxt-link to="/docs/deployment">
						<button>
							Read more
							<img src="~/assets/images/dark-arrow-circle-right.svg" alt="arrow right" />
						</button>
					</nuxt-link>
				</div>
			</section>
		</div>
	</div>
</template>

<script>
export default {
	layout: 'docs',
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
	flex-wrap: wrap;

	@media (max-width: 425px) {
		padding: 25px 20px;
	}
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
