<template>
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
			<p class="copyright">Copyright {{ currentYear }}, All Rights Reserved</p>
		</div>
	</footer>
</template>
<script>
export default {
	data() {
		return {
			currentYear: '',
			earlyAccessEmail: '',
			isSubmitingloadingEarlyAccessForm: false
		};
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
input{
	color: #fff;
}
	input::placeholder {
		color: #ebf4f1;
		opacity: 1;
	}
}
footer {
	.container {
		max-width: 1170px;
	}
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
