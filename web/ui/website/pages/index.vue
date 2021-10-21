<template>
	<div class="page">
		<header>
			<nav>
				<div>
					<div class="logo">
						<img src="~/assets/images/logo.svg" alt="logo" />
					</div>

					<button class="menu-button" @click="showMenu = !showMenu">
						<img src="~/assets/images/menu-icon.svg" alt="menu icon" />
					</button>

					<ul :class="showMenu ? 'show' : ''">
						<li>
							<a href="#features">Features</a>
						</li>
						<li>
							<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy/discussions">Community</a>
						</li>
						<li>
							<a href="https://github.com/frain-dev/convoy">
								<img src="~/assets/images/github-logo.svg" alt="github logo" />
							</a>
						</li>
					</ul>
				</div>
			</nav>

			<section class="hero-section">
				<div class="hero-section--cta">
					<h1>A Cloud native Webhook Service</h1>
					<p>With out-of-the-box security, reliability and scalability for your webhooks infrastructure.</p>

					<form @submit.prevent="requestAccess()">
						<div class="input">
							<input type="text" id="github" placeholder="Enter your github username" aria-label="Github Name" v-model="earlyAccessGithubName" />
						</div>
						<button :disabled="isSubmitingloadingEarlyAccessForm">{{ isSubmitingloadingEarlyAccessForm ? '...requesting' : earlyAccessFormButtonText }}</button>
					</form>
					<!-- <button>Get Started</button> -->
				</div>
				<div class="hero-section--img">
					<img src="~/assets/images/hero-img.svg" alt="hero" />
				</div>
			</section>
		</header>

		<section class="section companies">
			<p>Companies using us:</p>
			<ul>
				<li>
					<img src="~/assets/images/buycoins-logo.svg" style="width: 80px" alt="buycoins logo" />
				</li>
			</ul>
		</section>

		<section class="section features" id="features">
			<div class="container">
				<h2 class="section--title">Why Convoy</h2>
				<p class="section--description">
					Convoy enables you get started with delivering webhooks products in the space of minutes, along with a number of carefully thought out features to truely enable/enhance your webhooks
					delivery at scale.
				</p>

				<ul>
					<li>
						<img src="~/assets/images/secure-feature-icon.svg" alt="secure feature icon" />
						<h4>Secure</h4>
						<p>Create secrets, sign payload, verify event. Increase security by enabling rolling secrets.</p>
					</li>
					<li>
						<img src="~/assets/images/ease-feature-icon.svg" alt="ease feature icon" />
						<h4>Ease of use</h4>
						<p>Easily filter & debug events sent to multiple applications & endpoints with Delivery Attempt Logs</p>
					</li>
					<li>
						<img src="~/assets/images/cloud-feature-icon.svg" alt="cloud feature icon" />
						<h4>Cloud & Language Agnostic</h4>
						<p>Deploy to any environment and send events from any language (Ruby, Golang, JavaScript etc)</p>
					</li>
					<li>
						<img src="~/assets/images/rate-feature-icon.svg" alt="rate feature icon" />
						<h4>Rate Limiting</h4>
						<p>Avoid blasting too much events to a single endpoint at once with flexible rate limiting controls</p>
					</li>
					<li>
						<img src="~/assets/images/scalable-feature-icon.svg" alt="scalable feature icon" />
						<h4>Scalable</h4>
						<p>Independently scale Convoy as your system needs grows.</p>
					</li>
					<li>
						<div class="features--dashboard-screenshot">
							<img src="~/assets/images/dashboard-img.png" alt="dashboard image" />
						</div>
					</li>
				</ul>
				<div class="features--dashboard-screenshot stand-alone">
					<img src="~/assets/images/dashboard-img.png" alt="dashboard image" />
				</div>
			</div>
		</section>

		<section class="section core-features">
			<div class="container">
				<div>
					<h2 class="section--title">Core Features</h2>
					<ul>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Independently scalable</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Signatures & Rolling secrets</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Retries</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Clustering</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Language Agnostic</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Idempotency</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>URL per Event Type</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Static IPs</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Rich UI - Event Logs & Querying.</p>
						</li>
						<li>
							<img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
							<p>Payload Filtering</p>
						</li>
					</ul>
				</div>
				<div>
					<div class="core-features--img-container">
						<img src="~/assets/images/core-features-img.svg" alt="core flow" />
					</div>
				</div>
			</div>
		</section>

		<section class="section play-ground">
			<div class="container">
				<h2>Convoy Play Ground</h2>
				<p>We've deployed an instance of Convoy to the cloud so you can play with it and have a taste of how Convoy will enable your webhook service</p>
				<div class="img">
					<a href="https://playground.getconvoy.io/" target="_blank">
						Try it out free
						<img src="~/assets/images/arrow-right-icon.svg" alt="arrow right" />
					</a>
					<img src="~/assets/images/playground-img.png" alt="dashboard screenshot" />
				</div>
			</div>
			<div class="bg"></div>
		</section>

		<footer>
			<div class="container">
				<nav>
					<div class="logo">
						<img src="~/assets/images/logo.svg" alt="logo" />
						<p>
							by
							<a href="https://frain.dev">Frain</a>
						</p>
					</div>

					<ul>
						<li>
							<a target="_blank" rel="noopener noreferrer" href="https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ">
								<img src="~/assets/images/slack-icon.svg" alt="slack logo" />
							</a>
						</li>
						<li>
							<a target="_blank" rel="noopener noreferrer" href="https://twitter.com/getconvoy"><img src="~/assets/images/twitter-icon.svg" alt="twitter logo" /></a>
						</li>
						<li>
							<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy"><img src="~/assets/images/github-icon.svg" alt="mail logo" /></a>
						</li>
						<li>
							<a target="_blank" rel="noopener noreferrer" href="mailto:info@frain.dev"><img src="~/assets/images/mail-icon.svg" alt="mail logo" /></a>
						</li>
					</ul>
				</nav>
				<p>Copyright 2021, All Rights Reserved</p>
			</div>
		</footer>
	</div>
</template>

<script>
export default {
	data() {
		return {
			showMenu: false,
			isSubmitingloadingEarlyAccessForm: false,
			earlyAccessFormButtonText: 'Get Early Access',
			earlyAccessGithubName: ''
		};
	},
	async asyncData({ $content, params }) {
		const pageData = await $content('api-doc').fetch();
		return { pageData };
	},
	methods: {
		async requestAccess() {
			this.isSubmitingloadingEarlyAccessForm = true;
			try {
				const response = await fetch('/.netlify/functions/subscribe', {
					method: 'POST',
					mode: 'cors',
					cache: 'no-cache',
					credentials: 'same-origin',
					headers: {
						'Content-Type': 'application/json'
					},
					redirect: 'follow',
					referrerPolicy: 'no-referrer',
					body: JSON.stringify({
						githubName: this.earlyAccessGithubName
					})
				});
				await response.json();
				this.earlyAccessFormButtonText = 'Request Sent';
				this.setDefaultAccessButtonText();
				this.isSubmitingloadingEarlyAccessForm = false;
			} catch (error) {
				this.earlyAccessFormButtonText = 'Error';
				this.setDefaultAccessButtonText();
				this.isSubmitingloadingEarlyAccessForm = false;
			}
		},
		setDefaultAccessButtonText() {
			setTimeout(() => {
				this.earlyAccessFormButtonText = 'Get Early Access';
			}, 5000);
		}
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

header {
	background: url('~/assets/images/header-bg-pattern.svg') #1e252b no-repeat;
	background-size: cover;
	padding: 32px 20px 0;

	nav {
		max-width: 1376px;
		width: 100%;
		margin: auto;
		background: #222a31;
		border-radius: 16px;
		padding: 12px 20px;
		position: relative;

		& > div {
			display: flex;
			align-items: center;
			justify-content: space-between;
			max-width: 1106px;
			margin: auto;
		}

		.logo {
			font-weight: bold;
			font-size: 21px;
			line-height: 26px;
			color: #ffffff;
			width: 80%;

			img {
				width: 140px;
			}
		}

		.menu-button {
			display: block;

			img {
				width: 40px;
			}

			@media (min-width: $desktopBreakPoint) {
				display: none;
			}
		}

		ul {
			transition: all 0.5s;
			display: none;
			position: absolute;
			top: 55px;
			left: 0;
			background: #222a31;
			width: 100%;
			text-align: left;
			height: 0;
			overflow-y: hidden;

			&.show {
				padding-top: 20px;
				height: 175px;
				overflow-y: auto;
				display: block;
			}

			li {
				width: 100%;
				padding: 15px 20px;

				&:not(:last-of-type) {
					margin-right: 40px;
				}

				a {
					font-weight: 500;
					font-size: 14px;
					line-height: 17px;
					color: #ffffff;
					width: 100%;

					img {
						width: 24px;
					}
				}

				button {
					background: #477db3;
					border-radius: 8px;
					padding: 9px 29px;
					color: #ffffff;
					font-weight: 500;
					font-size: 16px;
					line-height: 28px;
					min-width: 146px;
				}
			}

			@media (min-width: $desktopBreakPoint) {
				display: flex;
				align-items: center;
				justify-content: flex-end;
				position: initial;
				height: initial;
				overflow-y: unset;

				&.show {
					display: flex;
					height: unset;
					padding-top: 0;
					overflow-y: unset;
				}

				li {
					padding: 0;
					width: fit-content;
				}
			}
		}
	}

	.hero-section {
		max-width: 1106px;
		margin: auto;
		display: flex;
		justify-content: space-between;
		min-height: 603px;
		height: 100%;
		flex-direction: column;
		padding: 75px 0 100px;

		@media (min-width: $desktopBreakPoint) {
			flex-direction: row;
			align-items: center;
		}

		&--cta {
			max-width: 533px;

			h1 {
				font-weight: bold;
				font-size: 48px;
				line-height: 64px;
				color: #ffffff;
				margin-bottom: 16px;
			}

			p {
				font-size: 18px;
				line-height: 30px;
				color: #ffffff;
			}
		}

		&--img {
			margin-left: 50px;
			text-align: center;
			display: none;

			@media (min-width: $desktopBreakPoint) {
				display: block;
			}
		}

		form {
			background: #ffffff;
			max-width: 430px;
			display: flex;
			border-radius: 8px;
			margin-top: 40px;

			.input {
				width: 100%;
				display: flex;

				input {
					max-width: 278px;
					width: 100%;
					border: none;
					padding: 9px 23px;
					border-radius: 8px 0 0 8px;
					outline: none;
					margin: 0;

					&::placeholder {
						font-weight: 500;
						font-size: 14px;
						line-height: 28px;
						color: #bdbdbd;
					}
				}
			}

			button {
				background: #477db3;
				border-radius: 0 8px 8px 0;
				padding: 9px 21px;
				color: #ffffff;
				font-weight: 500;
				font-size: 14px;
				line-height: 28px;
				max-width: 162px;
				width: 100%;
				margin: 0;
			}
		}
	}
}

.section {
	.container {
		max-width: 1150px;
		margin: auto;
	}

	.section--title {
		font-weight: bold;
		font-size: 48px;
		line-height: 60px;
		letter-spacing: -1.584px;

		@media (max-width: 1089px) {
			font-size: 40px;
		}
	}

	.section--description {
		font-size: 18px;
		line-height: 30px;
	}
}

section.companies {
	display: flex;
	max-width: 1150px;
	margin: -30px auto 0;
	justify-content: space-between;
	padding: 25px 20px;
	background: #fafafe;
	border-radius: 8px 8px 0px 0px;

	p {
		font-weight: bold;
		font-size: 16px;
		line-height: 24px;
	}

	ul {
		display: flex;
		align-items: center;

		li {
			&:not(:last-of-type) {
				margin-right: 50px;
			}
		}
	}
}

.features {
	padding: 160px 20px 800px;

	@media (max-width: 1089px) {
		padding: 60px 20px 100px;
	}

	.container {
		position: relative;
	}

	.section--title {
		margin-bottom: 26px;
	}

	.section--description {
		max-width: 608px;
		margin-bottom: 77px;
	}

	ul {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(238px, 1fr));
		gap: 80px 32px;

		img {
			width: 70px;
			margin-bottom: 36px;
		}

		li {
			position: relative;

			@media (max-width: 1089px) {
				.features--dashboard-screenshot {
					display: none;
				}
			}
		}

		h4 {
			font-weight: bold;
			font-size: 18px;
			line-height: 22px;
			letter-spacing: -0.5px;
			margin-bottom: 16px;
		}

		p {
			font-size: 15px;
			line-height: 22px;

			@media (max-width: 1089px) {
				font-size: 16px;
			}
		}
	}

	.features--dashboard-screenshot {
		position: absolute;
		left: 0;
		top: 0;
		width: 1100px;
		height: fit-content;

		@media (max-width: 1644px) {
			width: 1000px;
		}

		@media (max-width: 1443px) {
			width: 800px;
		}

		img {
			width: 100%;
		}

		&.stand-alone {
			position: unset;
			width: 100%;
			text-align: center;
			display: none;

			@media (max-width: 1089px) {
				display: block;
			}

			img {
				width: 100%;
			}
		}
	}
}

.core-features {
	background: #2b4b6a;
	padding: 80px 20px;
	color: #ffffff;
	margin-bottom: 490px;

	@media (min-width: $desktopBreakPoint) {
		margin-bottom: 590px;
	}

	.container {
		display: flex;
		align-items: center;
		justify-content: space-between;

		@media (max-width: 1089px) {
			display: unset;
		}
	}

	ul {
		margin-top: 32px;

		li {
			display: flex;
			align-items: center;
			margin-bottom: 29px;
			position: relative;
			font-weight: 500;
			font-size: 18px;
			line-height: 27px;

			&:not(:last-of-type)::after {
				content: '';
				position: absolute;
				height: 20px;
				width: 1px;
				background: #4b6680;
				bottom: -24px;
				left: 11px;
			}
		}

		img {
			width: 24px;
			margin-right: 24px;
		}
	}

	&--img-container {
		background: linear-gradient(0deg, #ffffff, #ffffff);
		box-shadow: 40px 44px 81px rgba(111, 118, 138, 0.160784);
		border-radius: 16px;
		height: 950px;
		max-width: 649px;
		width: 100%;
		margin-top: -160px;
		margin-bottom: -160px;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0 40px;

		@media (max-width: 1089px) {
			margin: 100px auto 0;
			height: fit-content;
			padding: 40px 20px;
		}

		img {
			max-width: 467px;
			width: 100%;
		}
	}
}

.play-ground {
	padding-bottom: 300px;
	position: relative;

	@media (min-width: $desktopBreakPoint) {
		padding-bottom: 400px;
	}

	.bg {
		content: '';
		bottom: 0;
		left: -300px;
		right: -300px;
		height: 300px;
		background: #ffffff;
		border-radius: 100% 100% 0 0;
		margin-bottom: -400px;
		margin-right: -300px;
		margin-left: -300px;

		@media (min-width: $desktopBreakPoint) {
			height: 460px;
		}
	}

	.container {
		width: 90%;
		box-shadow: 0px 15px 50px rgba(21, 37, 72, 0.05);
		border-radius: 24px;
		text-align: center;
		padding: 80px 2rem 0;
		background: url('~/assets/images/playground-bg.svg') #ffffff no-repeat;
		background-size: contain;
		background-position: center;
		position: absolute;
		left: 50%;
		top: -300px;
		transform: translate(-50%, 0);

		@media (min-width: $desktopBreakPoint) {
			background-position: top center;
		}
	}

	h2 {
		font-weight: bold;
		font-size: 34px;
		line-height: 60px;

		@media (min-width: $desktopBreakPoint) {
			font-size: 48px;
		}
	}

	p {
		font-weight: 500;
		font-size: 18px;
		line-height: 30px;
		max-width: 620px;
		margin: 16px auto 26px;
	}

	.img {
		max-width: 830px;
		margin: auto;
		position: relative;

		img {
			width: 100%;
			margin-bottom: -4px;
		}

		a {
			position: absolute;
			left: 50%;
			top: 64%;
			transform: translate(-50%, -50%);
			padding: 17px 24px;
			background: #477db3;
			color: #ffffff;
			width: 212px;
			text-align: left;
			border-radius: 8px;
			display: flex;
			align-items: center;
			justify-content: space-between;
			font-weight: bold;
			font-size: 18px;
			line-height: 22px;

			&::before {
				content: '';
				position: absolute;
				width: 86px;
				height: 102px;
				right: -40px;
				background: url('~/assets/images/play-ground-button-bg.svg');
				background-size: contain;
				z-index: -1;
			}

			img {
				width: 12px;
			}
		}
	}
}

footer {
	background: #1e252b;
	padding: 48px 20px 32px;
	color: #ffffff;

	.logo {
		p {
			text-align: left;
			margin-top: 10px;
			opacity: 0.3;

			a {
				font-weight: bold;
				text-decoration: underline;
			}
		}
	}

	.container {
		max-width: 1376px;
		width: 100%;
		margin: auto;
	}

	nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		padding-bottom: 29px;

		ul {
			display: flex;
		}

		li {
			width: 42px;
			height: 42px;
			background: rgba(255, 255, 255, 0.1);
			padding: 11px;
			display: flex;
			align-items: center;
			justify-content: center;
			border-radius: 50%;

			&:not(:last-of-type) {
				margin-right: 16px;
			}
		}
	}

	p {
		font-weight: 500;
		font-size: 14px;
		line-height: 17px;
		margin-top: 24px;
		text-align: center;

		@media (min-width: $desktopBreakPoint) {
			text-align: right;
		}
	}
}
</style>
