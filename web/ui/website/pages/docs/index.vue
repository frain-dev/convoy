<template>
	<div>
		<Header></Header>
		<header></header>

		<div class="page">
			<div class="cards">
				<div class="cards__header">
					<h1>Convoy Documentation</h1>
					<p>Find the guides, samples, and references you need to use Convoy to start sending out and receiving webhook events as fast as possible.</p>
				</div>
				<div class="cards__item">
					<h4>Convoy Core</h4>
					<p>This section is for those that want to self-host Convoy. This includes Configuring and deploying an instance.</p>
					<div class="cards__item__links">
						<nuxt-link to="/docs/guide" class="with-margin">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							Quick start
						</nuxt-link>
						<nuxt-link to="/docs/cli" class="with-margin">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							CLI
						</nuxt-link>
						<nuxt-link to="/docs/configuration">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							Configuration
						</nuxt-link>
						<nuxt-link to="/docs/deployment">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							Deployment
						</nuxt-link>
					</div>
				</div>
				<div class="cards__item">
					<h4>Convoy Cloud</h4>
					<p>This section helps you understand how to integrate Convoy Cloud.</p>
					<div class="cards__item__links">
						<a href="https://convoy.readme.io/reference/introduction" target="_blank" class="with-margin">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							Introduction
						</a>
						<a href="https://convoy.readme.io/reference/installation" target="_blank" class="with-margin">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							Authentication
						</a>
						<a href="">
							<img src="~/assets/images/blue-arrow.svg" alt="arrow right" />
							API Reference
						</a>
					</div>
				</div>
			</div>
		</div>
		<Footer></Footer>
	</div>
</template>
<style lang="scss" scoped>
$desktopBreakPoint: 880px;

header {
	background: url('~/assets/images/docs-bg.svg'), no-repeat;
	background-size: cover;
	height: 500px;
	width: 100%;
}
.page {
	max-width: 1150px;
	margin: auto;
	margin-top: -300px;
	margin-bottom: 30px;
	padding: 20px;

	@media (min-width: $desktopBreakPoint) {
		padding: 0px;
		margin-bottom: 130px;
	}
}
.cards {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(357px, 1fr));
	gap: 25px;
	margin-top: 24px;
	margin-bottom: 56px;

	&__header {
		color: #ffffff;
		padding-right: 1em;
		h1 {
			font-weight: 800;
			font-size: 40px;
			line-height: 48px;
			margin-bottom: 16px;
		}
		p {
			font-weight: 400;
			font-size: 16px;
			line-height: 24px;
			color: #fcfcfc;
			padding-right: 20px;
		}
	}
	&__item {
		background: #fcfcfc;
		box-shadow: 0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08);
		border-radius: 8px;
		padding: 32px 0 32px 32px;
		h4 {
			font-weight: 700;
			font-size: 24px;
			line-height: 35px;
			color: #000624;
			margin-bottom: 8px;
			padding-right: 32px;
		}
		p {
			font-weight: 500;
			font-size: 14px;
			line-height: 24px;
			color: #737a91;
			padding-bottom: 47px;
			padding-right: 32px;
			border-bottom: 1px solid #edeff5;
		}

		&__links {
			display: grid;
			grid-template-columns: 1fr 1fr;
			padding-top: 24px;
			a {
				font-weight: 500;
				font-size: 14px;
				line-height: 22px;
				color: #477db3;
				display: flex;
				align-items: center;
				justify-content: flex-start;
				img {
					height: 10px;
					width: 5px;
					margin-right: 5px;
				}
				&.with-margin {
					margin-bottom: 16px;
				}
			}
		}
	}
}
</style>
