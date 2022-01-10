<template>
	<div class="page blog">
		<header>
			<Header></Header>
		</header>

		<div class="main">
			<Nuxt />
		</div>

		<footer>
			<div class="container">
				<nav>
					<div>
						<div class="logo">
							<img src="~/assets/images/logo.svg" alt="logo" />
							<p>
								by
								<a href="https://frain.dev">Frain</a>
							</p>
						</div>

						<ul class="socials">
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ">
									<img src="~/assets/images/slack-icon.svg" alt="slack logo" />
								</a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://twitter.com/fraindev"><img src="~/assets/images/twitter-icon.svg" alt="twitter logo" /></a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy"><img src="~/assets/images/github-icon.svg" alt="mail logo" /></a>
							</li>
							<li>
								<a target="_blank" rel="noopener noreferrer" href="mailto:info@frain.dev"><img src="~/assets/images/mail-icon.svg" alt="mail logo" /></a>
							</li>
						</ul>
					</div>

					<div class="newsletter">
						<div>
							<div>
								<h5>Join our newsletter</h5>
								<p>No spam! Just articles, events, and talks.</p>
							</div>
							<img src="~/assets/images/mailbox.gif" alt="mailbox animation" />
						</div>
						<form @submit.prevent="requestAccess()">
							<img src="~/assets/images/mail-primary-icon.svg" alt="mail icon" />
							<input type="email" id="email" placeholder="Your email" aria-label="Email" v-model="earlyAccessEmail" />
							<button>
								<img src="~/assets/images/send-primary-icon.svg" alt="send icon" />
							</button>
						</form>
					</div>
				</nav>
				<p>Copyright 2022, All Rights Reserved</p>
			</div>
		</footer>
	</div>
</template>

<script>
export default {
	data: () => {
		return {
			showMenu: false,
			pages: [],
			earlyAccessEmail: '',
			isSubmitingloadingEarlyAccessForm: false
		};
	},
	async mounted() {
		let pages = await this.$content('docs').only(['title', 'id', 'toc', 'order']).sortBy('order', 'asc').fetch();
		pages = pages.sort((a, b) => a.order - b.order);
		this.pages = pages;
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
						email: this.earlyAccessEmail
					})
				});
				await response.json();
				this.earlyAccessEmail = '';
				this.isSubmitingloadingEarlyAccessForm = false;
			} catch (error) {
				this.isSubmitingloadingEarlyAccessForm = false;
			}
		}
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

.page.blog {
	flex-wrap: wrap;
	height: 100vh;
	font-family: 'Inter', sans-serif !important;
}

header {
	padding: 32px 20px 0;
	width: 100%;
	background: transparent;
	margin: 0 auto;
}

.main {
	margin: calc(20px + 32px + 58.23px) auto 0;
	padding: 0 0 100px;
	width: 100%;
	max-width: calc(1035px + 170px + 32px);
}

.newsletter {
	display: unset;
	width: 100%;
	padding: 0;
	margin-top: 41px;

	@media (min-width: $desktopBreakPoint) {
		max-width: 430px;
	}

	h5,
	p {
		text-align: left;
	}

	& > div {
		display: flex;
		justify-content: space-between;
		margin: 0;
		width: 100%;
		max-width: unset;
		align-items: center;

		div {
			order: 1;
			max-width: unset;
			margin: unset;
		}

		img {
			order: 2;
			width: 125px;
		}
	}

	form {
		background: #1c2126;
		border: 1px solid #262f37;
		box-sizing: border-box;
		border-radius: 8px;
		margin-top: -10px;
	}

	input::placeholder {
		color: #ebf4f1;
		opacity: 1;
	}
}

footer {
	.socials {
		margin-top: 50px;

		@media (min-width: $desktopBreakPoint) {
			margin-top: 70px;
		}
	}

	nav {
		flex-wrap: wrap;
		max-width: 1106px;
		margin: auto;

		& + p {
			max-width: 1106px;
			margin-left: auto;
			margin-right: auto;
			text-align: left;

			@media (min-width: $desktopBreakPoint) {
				text-align: right;
			}
		}
	}
}
</style>
