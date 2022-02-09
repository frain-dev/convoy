<template>
	<div>
		<nav>
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
						<a href="#features">Features</a>
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
					<li class="github">
						<a href="https://github.com/frain-dev/convoy">
							<img src="~/assets/images/github-logo.svg" alt="github logo" />
						</a>
					</li>
					<li class="ml-auto">
						<a href="https://dev--convoy-saas.netlify.app/login">Login</a>
						<button>
							Sign up for free
							<img src="~/assets/images/arrow-right-white.svg" alt="arrow right" />
						</button>
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
			showMenu: false
		};
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

	@media (min-width: $desktopBreakPoint) {
		padding: 32px 20px;
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
			height: 233px;
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
			button {
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
				
				img {
					height: 24px;
					width: 24px;
				}

				@media(min-width:  $desktopBreakPoint){
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
