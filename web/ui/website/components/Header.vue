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
					<!-- <li>
						<nuxt-link to="/blog">Blog</nuxt-link>
					</li> -->
					<li>
						<nuxt-link to="/docs">Docs</nuxt-link>
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

				<a href="https://github.com/frain-dev/convoy">
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
	max-width: 1376px;
	width: calc(100% - 40px);
	margin: auto;
	background: #222a31;
	border-radius: 16px;
	padding: 12px 20px;
	z-index: 10;
	position: fixed;
	left: 50%;
	transform: translate(-50%, 0);

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
		top: 75px;
		left: 0;
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

			&:last-of-type {
				display: none;
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
