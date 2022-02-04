<template>
	<div class="page">
		<header>
			<Header></Header>

			<section class="hero-section">
				<div class="hero-section--cta">
					<h1>A Cloud native Webhook Serviced</h1>
					<p>With out-of-the-box security, reliability and scalability for your webhooks infrastructure.</p>
					<!-- <nuxt-link to="/docs">Get Started</nuxt-link> -->

					<form @submit.prevent="requestAccess()">
						<div class="input">
							<input type="email" id="email" placeholder="Enter your email" aria-label="Email" v-model="earlyAccessEmail" />
						</div>
						<button :disabled="isSubmitingloadingEarlyAccessForm">{{ isSubmitingloadingEarlyAccessForm ? '...requesting' : earlyAccessFormButtonText }}</button>
					</form>
				</div>
				<div class="hero-section--img">
					<img src="~/assets/images/hero-img.png" alt="hero" />
				</div>
			</section>
			<section class="section companies">
				<ul>
					<li>Backed By:</li>
					<li>
						<img src="~/assets/images/y-combinator.svg" class="y-combinator" alt="y-combinator" />
					</li>
				</ul>
				<ul>
					<li>Trusted By:</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://buycoins.africa/">
							<img src="~/assets/images/buycoins-logo.svg" style="width: 70px" alt="buycoins logo" />
						</a>
					</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://termii.com/">
							<img src="~/assets/images/termii-logo.png" style="width: 80px" alt="termii logo" />
						</a>
					</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://www.getwallets.co/">
							<img src="~/assets/images/getwallets-logo.svg" style="width: 80px" alt="getwallets logo" />
						</a>
					</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://www.dojah.io/">
							<img src="~/assets/images/dojah-logo.png" style="width: 50px" alt="dojah logo" />
						</a>
					</li>
				</ul>
			</section>
		</header>
		<section class="github-star" v-if="githubStar">
			<span>Give us a star on GitHub</span>
			<img src="~/assets/images/github-icon-white.svg" class="github-icon" alt="github icon" />
			<button>
				<img src="~/assets/images/github-star.svg" alt="github star" />
				3490
			</button>
			<a @click="githubStar = false">
				<img src="~/assets/images/close-icon.svg" alt="close" />
			</a>
		</section>
		<section class="section features" id="features">
			<div class="container">
				<ul>
					<li class="no-style">
						<h2 class="section--title">Why use Convoy</h2>
						<p class="section--description">Easily filter & debug events sent to multiple applications & endpoints with Delivery Attempt Logs</p>
					</li>
					<li>
						<img src="~/assets/images/secure-feature-icon.svg" class="a" alt="secure feature icon" />
						<h4>Secure</h4>
						<p>Create secrets, sign payload, verify event. Increase security by enabling rolling secrets.</p>
					</li>
					<li>
						<img src="~/assets/images/ease-feature-icon.svg" class="b" alt="ease feature icon" />
						<h4>Ease of use</h4>
						<p>Easily filter & debug events sent to multiple applications & endpoints with Delivery Attempt Logs</p>
					</li>
					<li>
						<img src="~/assets/images/cloud-feature-icon.svg" class="c" alt="cloud feature icon" />
						<h4>Cloud & Language Agnostic</h4>
						<p>Deploy to any environment and send events from any language (Ruby, Golang, JavaScript etc)</p>
					</li>
					<li>
						<img src="~/assets/images/rate-feature-icon.svg" class="d" alt="rate feature icon" />
						<h4>Rate Limiting</h4>
						<p>Avoid blasting too much events to a single endpoint at once with flexible rate limiting controls</p>
					</li>
					<li>
						<img src="~/assets/images/scalable-feature-icon.svg" class="e" alt="scalable feature icon" />
						<h4>Scalable</h4>
						<p>Independently scale Convoy as your system needs grows.</p>
					</li>
					<!-- <li>
						<div class="features--dashboard-screenshot">
							<img src="~/assets/images/dashboard-img.png" alt="dashboard image" />
						</div>
					</li> -->
				</ul>
				<!-- <div class="features--dashboard-screenshot stand-alone">
					<img src="~/assets/images/dashboard-img.png" alt="dashboard image" />
				</div> -->
			</div>
		</section>

		<section class="section core-features">
			<div class="container">
				<h2 class="section--title">Core Features</h2>
				<div class="core-features--list">
					<div>
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
						</ul>
					</div>
					<div>
						<ul>
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
								<p>Flexible Configuration</p>
							</li>
						</ul>
					</div>
				</div>
				<div class="core-features--list expand">
					<div>
						<ul>
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
			</div>
		</section>

		<section class="section cloud">
			<div class="container">
				<h2>Convoy Cloud</h2>
				<p>
					We built convoy core which you already love, now we've hosted on the cloud for you to simply create an account and send webhooks without the hassle of downloading, deployment and management.
				</p>
				<div class="cloud-features">
					<div class="cloud-features_item" v-for="feature of cloudFeatures" :key="feature">
						<img src="~/assets/images/green_check.svg" alt="check" />
						<span>{{ feature }}</span>
					</div>
				</div>
				<div class="cloud-group-img">
					<img src="~/assets/images/groups-img.svg" alt="group" />
					<img src="~/assets/images/groups-img-small.svg" class="small" alt="group" />
				</div>
			</div>
			<div class="cloud-get-started">
				<a class="primary">
					Get started for free
					<img src="~/assets/images/arrow-right-icon.svg" alt="arrow right" />
				</a>
			</div>
		</section>

		<section class="section offers">
			<div class="container">
				<h3>What Convoy Offers</h3>

				<div class="offers-list">
					<div class="offers-list-item">
						<h4>Create multiple projects</h4>
						<img src="~/assets/images/projects.svg" alt="projects" />
					</div>
					<div class="offers-list-item">
						<h4>Manage multiple organizations with a single account.</h4>
						<img src="~/assets/images/organisation.svg" alt="organisation" />
					</div>
					<div class="offers-list-item">
						<h4>Manage your team members access.</h4>
						<img src="~/assets/images/teams.svg" alt="teams" />
					</div>
					<div class="offers-list-item">
						<h4>Dashboard that enables you to do even more.</h4>
						<img src="~/assets/images/dashboard.svg" alt="dashboard" />
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

		<section class="section blog">
			<div class="container">
				<img src="~/assets/images/Featured.svg" alt="blogpost" />
				<img src="~/assets/images/Featured-small.svg" class="small" alt="blogpost" />
			</div>
		</section>

		<footer>
			<div class="container">
				<nav>
					<div>
						<div class="logo">
							<img src="~/assets/images/logo.svg" alt="logo" />
						</div>
						<ul class="socials">
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy"><img src="~/assets/images/github-icon.svg" alt="mail logo" /></a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy"><img src="~/assets/images/linkedin.svg" alt="linkedin logo" /></a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="mailto:info@frain.dev"><img src="~/assets/images/mail-icon.svg" alt="mail logo" /></a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ">
									<img src="~/assets/images/slack-icon.svg" alt="slack logo" />
								</a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://twitter.com/fraindev"><img src="~/assets/images/twitter-icon.svg" alt="twitter logo" /></a>
							</li>
						</ul>
					</div>
					<div>
						<div class="join-news-letter">
							<div>
								<p>Join our newsletter</p>
								<p>No spam! Just articles, events, and talks.</p>
							</div>
							<div>
								<img src="~/assets/images/mailbox.gif" alt="mailbox icon" />
							</div>
						</div>
						<div class="input has-icon">
							<img src="~/assets/images/email.svg" class="prepend-icon" alt="email" />
							<input type="text" placeholder="Your Email" />
							<button>
								<img src="~/assets/images/send-email.svg" alt="send email" />
							</button>
						</div>
					</div>
				</nav>
				<p class="copyright">Copyright {{ currentYear }}, All Rights Reserved</p>
			</div>
		</footer>
	</div>
</template>

<script>
export default {
	data() {
		return {
			isSubmitingloadingEarlyAccessForm: false,
			earlyAccessFormButtonText: 'Get Early Access',
			earlyAccessEmail: '',
			cloudFeatures: ['Create Account', 'Manage Webhooks', 'View Metrics', 'Create Endpoints', 'Create Groups'],
			currentYear: '',
			githubStar: true
		};
	},
	async asyncData({ $content, params }) {
		const pageData = await $content('docs').fetch();
		return { pageData };
	},
	mounted() {
		this.getCurrentYear();
	},
	methods: {
		getCurrentYear() {
			const currentDate = new Date();
			this.currentYear = currentDate.getFullYear();
		},
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
						email: this.earlyAccessEmail
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
				this.earlyAccessFormButtonText = 'Convoy Cloud Early Access';
			}, 5000);
		}
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

header {
	background: linear-gradient(180deg, #2c2f3e 0%, #422f41 100%);
	padding: 0 20px 0;
	position: relative;

	.hero-section {
		max-width: 1106px;
		margin: auto;
		display: flex;
		justify-content: space-between;
		min-height: 760px;
		height: 100%;
		flex-direction: column;
		padding: 175px 0 100px;

		@media (min-width: $desktopBreakPoint) {
			flex-direction: row;
			align-items: center;
		}

		&--cta {
			max-width: 533px;

			h1 {
				font-weight: 800;
				font-size: 32px;
				line-height: 48px;
				color: #ffffff;
				margin-bottom: 16px;
				max-width: 320px;

				@media (min-width: $desktopBreakPoint) {
					font-weight: bold;
					font-size: 48px;
					line-height: 64px;
					max-width: 600px;
				}
			}

			p {
				font-size: 16px;
				line-height: 30px;

				color: #ffffff;
				@media (min-width: $desktopBreakPoint) {
					font-size: 18px;
					line-height: 30px;
				}
			}
		}

		&--img {
			text-align: center;
			margin-top: 80px;

			img {
				max-width: 330px;
			}

			@media (min-width: $desktopBreakPoint) {
				margin-left: 50px;
				margin-top: unset;
			}
		}

		button,
		a {
			background: #477db3;
			padding: 9px 21px;
			color: #ffffff;
			font-weight: 500;
			font-size: 14px;
			line-height: 28px;
			margin: 20px 0 0;
			max-width: 200px;
			width: 100%;
			display: block;
			border-radius: 5px;
			text-align: center;
		}

		form {
			background: #ffffff;
			max-width: 430px;
			display: flex;
			border-radius: 8px;
			margin-top: 40px;
			flex-direction: row;

			@media (min-width: $desktopBreakPoint) {
				flex-direction: row;
			}

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
				max-width: 164px;
				width: 100%;
				margin: 0;
				border-radius: 0 8px 8px 0;
				font-size: 12px;

				@media (min-width: $desktopBreakPoint) {
					max-width: 220px;
					width: 100%;
					margin-top: 0;
				}
			}
		}
	}

	.companies {
		display: flex;
		max-width: 1150px;
		margin: 0 auto;
		justify-content: space-between;
		padding: 20px 20px 25px;
		background: #473749;
		border-radius: 24px 24px 0px 0px;
		align-items: center;
		flex-wrap: wrap;

		ul {
			display: flex;
			align-items: center;
			margin-bottom: 10px;
			flex-wrap: wrap;

			li {
				font-size: 16px;
				line-height: 24px;
				color: #fcfcfc;
				&:not(:last-of-type) {
					margin-right: 50px;
				}

				img {
					filter: brightness(0) invert(1);
					transition: all 0.3s ease-in-out;
					margin-bottom: -7px;

					&.y-combinator {
						filter: none;
					}
					&:hover {
						filter: grayscale(0%);
					}
				}
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


.features {
	padding: 160px 20px 130px;

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
		grid-template-columns: repeat(auto-fill, minmax(333px, 1fr));
		gap: 80px 32px;
		margin-bottom: 50px;

		img {
			width: 70px;
			margin-bottom: 36px;
			&.a {
				box-shadow: 0px 22px 24px rgba(65, 111, 244, 0.2);
			}
			&.b {
				box-shadow: 0px 22px 24px rgba(43, 214, 123, 0.2);
			}
			&.c {
				box-shadow: 0px 22px 24px rgba(247, 227, 109, 0.2);
			}
			&.d {
				box-shadow: 0px 22px 24px rgba(247, 227, 109, 0.2);
			}
			&.e {
				box-shadow: 0px 22px 24px rgba(43, 214, 123, 0.2);
			}
		}

		li {
			position: relative;
			background: #fcfcfc;
			box-shadow: 0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08);
			border-radius: 8px;
			padding: 32px;

			&.no-style {
				background: none;
				padding: 0;
				box-shadow: none;
			}

			p {
				font-size: 16px;
				line-height: 24px;
				color: #737a91;
			}

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
	background: linear-gradient(180deg, #2c2f3e 0%, #422f41 100%);
	padding: 80px 20px 196px;
	color: #ffffff;
	margin-bottom: 244px;

	.container {
		@media (min-width: $desktopBreakPoint) {
			padding: 75px;
		}
	}

	@media (min-width: $desktopBreakPoint) {
		padding: 80px 20px 40px;
		margin-bottom: 227px;
	}

	&--list {
		display: flex;
		align-items: center;
		justify-content: space-between;
		max-width: 750px;

		&.expand {
			max-width: 950px;
		}
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
				background: rgba(252, 252, 252, 0.08);
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
		height: 377px;
		width: 100%;
		margin-bottom: -250px;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0 40px;

		@media (max-width: 1089px) {
			height: fit-content;
			padding: 40px 20px;
			margin-top: +70px;
			margin-bottom: -310px;
		}

		img {
			max-width: 467px;
			width: 100%;
		}
	}
}
.cloud {
	margin: 20px auto;
	margin-bottom: 150px;

	@media (min-width: $desktopBreakPoint) {
		margin-bottom: 300px;
	}
	.container {
		background: linear-gradient(180deg, #2c2f3e 0%, #422f41 100%);
		box-shadow: 0px 4px 8px rgba(12, 26, 75, 0.1), 0px 10px 16px rgba(20, 37, 63, 0.06);
		border-radius: 16px;
		padding: 40px 20px;
		text-align: center;
		min-height: 500px;
		color: #fff;
		position: relative;
		h2 {
			font-weight: bold;
			font-size: 32px;
			line-height: 40px;
			letter-spacing: 0.2px;
			margin-bottom: 16px;
		}
		p {
			font-size: 16px;
			line-height: 24px;
			margin-bottom: 32px;
		}

		@media (max-width: $desktopBreakPoint) {
			margin-left: 20px;
			margin-right: 20px;
		}
		@media (min-width: $desktopBreakPoint) {
			padding: 64px 120px;
			min-height: 600px;

			h2 {
				font-size: 40px;
				line-height: 60px;
				letter-spacing: 0.2px;
			}
			p {
				font-size: 18px;
				line-height: 32px;
			}
		}
	}
	&-group-img {
		margin-top: +60px;
		margin-bottom: -500px;
		img {
			display: none;
			&.small {
				display: block;
			}
		}
		@media (min-width: $desktopBreakPoint) {
			img {
				display: block;
				&.small {
					display: none;
				}
			}
		}
	}
	&-features {
		display: flex;
		@media (max-width: $desktopBreakPoint) {
			overflow-x: scroll;
		}
		&_item {
			background: rgba(255, 255, 255, 0.1);
			border: 1px solid rgba(237, 241, 255, 0.05);
			box-sizing: border-box;
			box-shadow: 0px 1px 5px rgba(0, 0, 0, 0.04);
			border-radius: 5px;
			font-style: normal;
			font-weight: 500;
			font-size: 14px;
			line-height: 22px;
			color: #fcfcfc;
			padding: 7px 19px 7px 10px;
			width: fit-content;
			display: flex;
			align-items: center;
			white-space: nowrap;
			&:not(:last-of-type) {
				margin-right: 20px;
			}
			img {
				height: 24px;
				width: 24px;
				margin-right: 12px;
			}
		}
	}
	&-get-started {
		background: rgba(8, 43, 145, 0.07);
		width: 92%;
		margin: 0 auto;
		height: 400px;
		box-shadow: 0px 16px 16px rgba(12, 26, 75, 0.05), 0px 30px 40px rgba(20, 37, 63, 0.08);
		border-radius: 0 0 24px 24px;
		display: flex;
		align-items: flex-end;
		justify-content: center;
		padding-bottom: 32px;
		@media (max-width: 1080px) {
			width: 92%;
			height: 400px;
		}
		@media (min-width: 1080px) {
			height: 250px;
			width: 1064px;
		}
		@media (max-width: 425px) {
			width: 86%;
			height: 200px;
		}
	}
}
.get-started {
	background: linear-gradient(180deg, #2c2f3e 0%, #422f41 100%);
	box-shadow: 0px 4px 8px rgba(12, 26, 75, 0.1), 0px 10px 16px rgba(20, 37, 63, 0.06);
	text-align: center;
	color: #fff;
	border-radius: 16px;
	@media (min-width: $desktopBreakPoint) {
		padding: 56px 196px;
		margin-bottom: 700px;

		h3 {
			font-weight: bold;
			font-size: 40px;
			line-height: 60px;
			letter-spacing: 0.2px;
		}
		p {
			font-size: 18px;
			line-height: 32px;
		}
		a {
			height: 56px;
			background: #477db3;
			border-radius: 8px;
			display: flex;
			justify-content: center;
			font-weight: 600;
			font-size: 16px;
			line-height: 24px;
			min-width: 227px;
			img {
				margin-left: 16px;
				height: 12px;
			}
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
button,
a {
	&.primary {
		height: 56px;
		background: #477db3;
		border-radius: 8px;
		display: flex;
		justify-content: center;
		align-items: center;
		font-weight: 600;
		font-size: 16px;
		line-height: 24px;
		min-width: 227px;
		width: fit-content;
		white-space: nowrap;
		color: #fff;
		img {
			margin-left: 12px;
			height: 12px;
			width: 12px;
		}
	}
}
.offers {
	margin-bottom: 300px;
	@media (max-width: 1085px) {
		padding: 20px;
	}
	h3 {
		font-weight: bold;
		font-size: 48px;
		line-height: 58px;
		color: #000624;
		margin-bottom: 53px;
	}
	&-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(500px, 1fr));
		gap: 48px;

		@media (max-width: 1085px) {
			grid-template-columns: repeat(auto-fill, minmax(333px, 1fr));
			gap: 48px;
		}
		&-item {
			background: #ffffff;
			border: 1px solid #edf2f7;
			box-sizing: border-box;
			border-radius: 8px;
			padding: 64px 0 0 64px;
			@media (max-width: $desktopBreakPoint) {
				padding: 40px 0 0 40px;
			}
			h4 {
				font-weight: bold;
				font-size: 28px;
				line-height: 40px;
				max-width: 450px;
				color: #000624;
				margin-bottom: 32px;

				@media (max-width: $desktopBreakPoint) {
					font-size: 19px;
					line-height: 30px;
				}
			}

			img {
				max-width: 497px;
				max-height: 242px;
			}

			&:first-child {
				h4 {
					width: 230px;
					@media (max-width: $desktopBreakPoint) {
						width: 180px;
					}
				}
			}
		}
	}
}
.blog {
	background: linear-gradient(180deg, #2c2f3e 0%, #422f41 100%);
	padding: 80px 0px 0 40px;

	img {
		max-width: 970px;
		display: block;
		&.small {
			display: none;
		}
		@media (max-width: 425px) {
			display: none;
			&.small {
				display: block;
			}
		}
	}
	@media (max-width: $desktopBreakPoint) {
		padding: 60px 0px 0 40px;
	}
	@media (max-width: 425px) {
		padding: 80px 0 0 0;
	}
}
.github-star {
	background: #477db3;
	border-radius: 100px;
	max-width: 400px;
	margin: 0 auto;
	height: 50px;
	padding: 7px 11px;
	display: flex;
	align-items: center;
	justify-content: center;
	font-weight: 500;
	font-size: 14px;
	line-height: 22px;
	color: #fff;
	position: relative;
	top: -20px;
	@media (min-width: $desktopBreakPoint) {
		font-size: 18px;
		line-height: 30px;
		height: 60px;
		padding: 12px 16px;
		max-width: 434px;
	}
	.github-icon {
		height: 20px;
		width: 20px;
		margin-left: 13px;
	}

	button {
		background: #ffffff;
		border: 1px solid #edeff5;
		box-shadow: 0px 2px 8px rgba(12, 26, 75, 0.08), 0px 3px 8px -1px rgba(50, 50, 71, 0.05);
		border-radius: 4px;
		color: #477db3;
		font-weight: 500;
		font-size: 14px;
		line-height: 20px;
		height: 36px;
		padding: 10px;
		display: flex;
		align-items: center;
		margin-left: 13px;
		img {
			height: 16px;
			width: 16px;
			margin-right: 5px;
		}
	}
	a {
		height: 20px;
		width: 20px;
		margin-left: 13px;
		&:hover {
			cursor: pointer;
		}
	}
}
footer {
	nav {
		display: flex;
		justify-content: space-between;
		flex-wrap: wrap;
	}
	.logo {
		height: 28px;
		margin-bottom: 78px;
		img {
			height: 28px;
			width: 109px;
		}
	}
	.join-news-letter {
		display: flex;
		align-items: flex-start;
		@media (max-width: 425px) {
			margin-top: 41px;
		}
		p {
			font-weight: 500;
			font-size: 14px;
			line-height: 22px;
			color: #ebf4f1;
			margin-bottom: 10px;
			margin-top: unset;
			text-align: left;
		}
		img {
			height: 125px;
		}
	}
	.input {
		margin-top: -25px;
		max-width: 460px;
		width: 100%;
		display: flex;
		align-items: center;
		position: relative;
		input {
			background: #1c2126;
			border: 1px solid #262f37;
			box-sizing: border-box;
			border-radius: 8px;
			height: 56px;
			width: 100%;
			padding-left: 40px;
			font-size: 16px;
			line-height: 180%;
			color: #fff;
			&::placeholder {
				color: #ebf4f1;
			}
			&:focus {
				outline: none;
				box-shadow: none;
				border: 1px solid #477db3;
				transition: 0.3 ease-in-out;
			}
		}

		.prepend-icon {
			height: 12px;
			width: 16px;
			margin-right: -30px;
			z-index: 11;
		}

		button {
			margin-left: -50px;
		}
	}
	p.copyright {
		margin-top: 24px;
		@media (max-width: 425px) {
			text-align: left;
		}
	}
}
</style>
