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
						<form>
							<img src="~/assets/images/mail-primary-icon.svg" alt="mail icon" />
							<input type="email" placeholder="Your Emaill" />
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
			pages: []
		};
	},
	async mounted() {
		let pages = await this.$content('docs').only(['title', 'id', 'toc', 'order']).sortBy('order', 'asc').fetch();
		pages = pages.sort((a, b) => a.order - b.order);
		this.pages = pages;
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
	max-width: calc(1035px + 170px + 32px);
	margin: calc(20px + 32px + 58.23px) auto 0;
	width: 100%;
	overflow-y: auto;
	padding-bottom: 100px;
	display: flex;
	justify-content: space-between;
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
	}
}
</style>
