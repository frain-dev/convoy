<template>
	<div>
		<nav :class="{ extraPadding: githubStar }">
			<section class="github-star" v-if="githubStar">
				<span>Give us a star on GitHub</span>
				<a class="github-icon" target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy">
					<img src="~/assets/images/github-icon-white.svg" alt="github icon" />
				</a>
				<button>
					<img src="~/assets/images/github-star.svg" alt="github star" />
					222
				</button>
				<a @click="closeStar()">
					<img src="~/assets/images/close-icon.svg" alt="close" />
				</a>
			</section>
			<div>
				<button class="menu-button" @click="showMenu = !showMenu">
					<img v-if="!showMenu" src="~/assets/images/menu-icon.svg" alt="menu icon" width="24" />
					<img v-if="showMenu" src="~/assets/images/close-icon.svg" alt="close icon" width="24" />
				</button>

				<div class="logo">
					<nuxt-link to="/">
						<img src="~/assets/images/logo.svg" alt="logo" />
					</nuxt-link>
				</div>

				<ul :class="showMenu ? 'show' : ''">
					<li>
						<a href="/#features">Features</a>
					</li>
					<li>
						<nuxt-link to="/blog">Blog</nuxt-link>
					</li>
					<li>
						<nuxt-link to="/docs">Docs</nuxt-link>
					</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy/discussions">Community</a>
					</li>
					<li>
						<nuxt-link to="/download">Download</nuxt-link>
					</li>
					<li class="github">
						<a href="https://github.com/frain-dev/convoy">
							<img src="~/assets/images/github-logo.svg" alt="github logo" />
						</a>
					</li>
					<li class="ml-auto">
						<a target="_blank" rel="noopener noreferrer" href="https://app.getconvoy.io/login">Login</a>
						<a class="primary" target="_blank" rel="noopener noreferrer" href="https://app.getconvoy.io/signup">
							Sign up for free
							<img src="~/assets/images/arrow-right-white.svg" alt="arrow right" />
						</a>
					</li>
				</ul>

				<a href="https://github.com/frain-dev/convoy" class="small">
					<img src="~/assets/images/github-logo.svg" alt="github logo" />
				</a>
			</div>
		</nav>
		<div class="overlay" :class="showMenu ? 'show' : ''" @click="showMenu = !showMenu"></div>
	</div>
</template>

<script>
export default {
	data() {
		return {
			githubStar: null,
			showMenu: false
		};
	},
	mounted() {
		this.checkForGithubStar();
	},
	methods: {
		closeStar() {
			this.githubStar = false;
			localStorage.setItem('githubStar', false);
		},
		checkForGithubStar() {
			const starStatus = localStorage.getItem('githubStar');
			if (starStatus != null) {
				if (starStatus == 'true') {
					this.githubStar = true;
				} else {
					this.githubStar = false;
				}
			} else {
				localStorage.setItem('githubStar', true);
				this.githubStar = true;
			}
		}
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

nav {
	width: 100%;
	margin: auto;
	background: #302f3f;
	box-shadow: inset 0px -3px 8px rgba(255, 255, 255, 0.07);
	backdrop-filter: blur(36px);
	padding: 53px 20px 21px 20px;
	z-index: 10;
	position: fixed;
	left: 50%;
	transform: translate(-50%, 0);

	&.extraPadding {
		padding: 63px 20px 21px 20px;
	}
	@media (min-width: $desktopBreakPoint) {
		padding: 32px 20px;
		&.extraPadding {
			padding: 80px 20px 21px 20px;
		}
	}

	& > div {
		display: flex;
		align-items: center;
		max-width: 1106px;
		margin: auto;
	}

	.logo {
		font-weight: bold;
		font-size: 21px;
		line-height: 26px;
		color: #ffffff;
		width: 80%;
		margin-left: 50px;

		@media (min-width: $desktopBreakPoint) {
			width: 22%;
			margin-left: 0px;
		}

		img {
			width: 110px;
		}
	}

	.menu-button {
		display: block;
		position: absolute;

		@media (min-width: $desktopBreakPoint) {
			display: none;
		}
	}

	ul {
		transition: all 0.5s;
		display: none;
		position: absolute;
		top: 105px;
		left: 20px;
		width: 256px;
		text-align: left;
		height: 0;
		overflow-y: hidden;
		background: #ffffff;
		box-shadow: 0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08);
		border-radius: 10px;
		z-index: 5;

		&.show {
			padding-top: 20px;
			height: 390px;
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
				color: #5f5f68;
				width: 100%;

				img {
					width: 24px;
				}
			}
			&.github {
				display: none;
			}
			&.ml-auto {
				button {
					margin-top: 30px;
				}
			}
			button,
			a.primary {
				background: #477db3;
				border-radius: 8px;
				padding: 9px 20px;
				color: #ffffff;
				font-weight: 500;
				font-size: 16px;
				line-height: 28px;
				min-width: 175px;
				display: flex;
				align-items: center;
				white-space: nowrap;
				margin-top: 15px;
				img {
					height: 24px;
					width: 24px;
				}

				@media (min-width: $desktopBreakPoint) {
					margin-top: unset;
					margin-left: 24px;
				}
			}
		}

		& + a {
			margin-top: 3px;
		}

		@media (min-width: $desktopBreakPoint) {
			display: flex;
			align-items: center;
			justify-content: flex-end;
			position: initial;
			height: initial;
			overflow-y: unset;
			width: 100%;
			background: transparent;

			&.show {
				display: flex;
				height: unset;
				padding-top: 0;
				overflow-y: unset;
			}

			li {
				padding: 0;
				width: fit-content;

				a {
					color: rgba(255, 255, 255, 0.6);

					&.nuxt-link-active {
						color: #ffffff;
					}
				}

				&:last-of-type {
					display: none;
				}

				&.github {
					display: block;
				}

				&.ml-auto {
					margin-left: auto;
					display: flex;
					align-items: center;
					button {
						margin-left: 24px;
						margin-top: unset;
					}
				}
			}
		}
	}

	a.small {
		display: block;
		@media (min-width: $desktopBreakPoint) {
			display: none;
		}
	}
	.github-star {
		position: fixed;
		top: 0;
		left: 0;
		background: #477db3;
		width: 100vw;
		height: 40px;
		padding: 7px 11px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 500;
		font-size: 12px;
		line-height: 20px;
		color: #fff;
		z-index: 99;

		@media (min-width: $desktopBreakPoint) {
			font-size: 16px;
			line-height: 24px;
		}
		.github-icon {
			img {
				height: 20px;
				width: 20px;
			}
			margin-right: 13px;
		}

		button {
			background: #ffffff;
			border: 1px solid #edeff5;
			box-shadow: 0px 2px 8px rgba(12, 26, 75, 0.08), 0px 3px 8px -1px rgba(50, 50, 71, 0.05);
			border-radius: 4px;
			color: #477db3;
			font-weight: 500;
			font-size: 12px;
			line-height: 20px;
			height: 24px;
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
}

.overlay {
	position: fixed;
	top: 0;
	left: 0;
	width: 100%;
	height: 100%;
	background: rgba(0, 0, 0, 0.1);
	backdrop-filter: blur(25px);
	transition: all 0.5s;
	opacity: 0;
	pointer-events: none;
	z-index: 2;

	&.show {
		opacity: 1;
		pointer-events: all;
	}
}
</style>
