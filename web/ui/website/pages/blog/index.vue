<template>
	<div class="main">
		<aside>
			<ul>
				<h3>CATEGORIES</h3>

				<li>
					<nuxt-link to="/blog">Blog</nuxt-link>
				</li>
				<li>
					<nuxt-link to="/docs">Docs</nuxt-link>
				</li>
			</ul>

			<form>
				<img src="~/assets/images/search-icon.svg" alt="search icon" />
				<input type="search" placeholder="Search" />
			</form>

			<div class="social">
				<h3>Follow Us</h3>

				<ul class="socials">
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://join.slack.com/t/convoy-community/shared_invite/zt-xiuuoj0m-yPp~ylfYMCV9s038QL0IUQ">
							<img src="~/assets/images/slack-grey-icon.svg" alt="slack logo" />
						</a>
					</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://twitter.com/fraindev"><img src="~/assets/images/twitter-grey-icon.svg" alt="twitter logo" /></a>
					</li>
					<li>
						<a target="_blank" rel="noopener noreferrer" href="https://github.com/frain-dev/convoy"><img src="~/assets/images/github-grey-icon.svg" alt="mail logo" /></a>
					</li>
				</ul>
			</div>
		</aside>

		<main>
			<div class="dropdown-container">
				<h1>
					All Posts
					<button @click="showCategories = !showCategories">
						<img src="~/assets/images/angle-down-black-icon.svg" alt="arrow down iconn" />
					</button>
				</h1>
				<ul class="dropdown" v-if="showCategories">
					<li>
						<nuxt-link to="#">All Posts</nuxt-link>
					</li>
					<li>
						<nuxt-link to="#">Engineering</nuxt-link>
					</li>
					<li>
						<nuxt-link to="#">Support</nuxt-link>
					</li>
					<li>
						<nuxt-link to="#">Marketing</nuxt-link>
					</li>
				</ul>
			</div>
			<p>Semper purus aliquam id sed. Egestas sit scelerisque sagittis leo blandit et viverra.</p>

			<div class="featured card posts">
				<div class="post">
					<div class="post--head">
						<div class="tag">FEATURED</div>
						<div class="date">{{ featurePosts[0].date | date }}</div>
					</div>
					<h3 class="post--title">{{ featurePosts[0].title }}</h3>
					<p class="post--body">{{ featurePosts[0].description }}</p>
					<div class="post--footer">
						<div class="post--author">
							<img src="~/assets/images/author-img.png" alt="author imge" />
							<div>
								<h5>{{ author(featurePosts[0].author).name }}</h5>
								<p>{{ author(featurePosts[0].author).role }} Convoy</p>
							</div>
						</div>
						<nuxt-link :to="'blog/' + featurePosts[0].slug">
							Read More
							<img src="~/assets/images/angle-right-primary.svg" alt="read more icon" />
						</nuxt-link>
					</div>
				</div>
				<div class="img">
					<img :src="'https://res.cloudinary.com/frain/image/upload/c_crop,f_auto,q_auto,w_367,h_350,x_41,y_41/' + featurePosts[0].featureImg" alt="featured post img" />
				</div>
			</div>

			<div class="posts">
				<div class="post" v-for="(post, index) in posts" :key="index">
					<div class="post--img">
						<img :src="'https://res.cloudinary.com/frain/image/upload/c_fill,g_north,h_179,w_461,x_0,y_0/' + post.thumbnail" alt="post image" />
					</div>
					<div class="tag clear">FEATURED</div>
					<h3 class="post--title small">{{ post.title }}</h3>
					<p class="post--body">{{ post.description }}</p>
					<div class="post--footer">
						<div class="post--author">
							<img src="~/assets/images/author-img.png" alt="author imge" />
							<div>
								<h5>{{ author(post.author).name }}</h5>
								<p>{{ author(post.author).role }} Convoy</p>
							</div>
						</div>
						<nuxt-link :to="'blog/' + post.slug">
							Read More
							<img src="~/assets/images/angle-right-primary.svg" alt="read more icon" />
						</nuxt-link>
					</div>
				</div>
			</div>

			<div class="newsletter card">
				<div>
					<h5>Join our newsletter</h5>
					<p>No spam! Just articles, events, and talks.</p>
					<form>
						<img src="~/assets/images/mail-primary-icon.svg" alt="mail icon" />
						<input type="email" placeholder="Your Emaill" />
						<button>
							<img src="~/assets/images/send-primary-icon.svg" alt="send icon" />
						</button>
					</form>
				</div>
				<img src="~/assets/images/mailbox.gif" alt="mailbox animation" />
			</div>
		</main>
	</div>
</template>

<script>
export default {
	layout: 'blog',
	data: () => {
		return {
			showCategories: false
		};
	},
	async asyncData({ $content }) {
		const posts = await $content('blog').only(['author', 'id', 'description', 'createdAt', 'featureImg', 'slug', 'thumbnail', 'title', 'tags', 'featurePost', 'date']).fetch();
		const featurePosts = posts.filter(post => post.featurePost === true);
		const authors = await $content('blog-authors').fetch();
		return { posts, authors, featurePosts };
	},
	methods: {
		author(authorSlug) {
			return this.authors.find(author => author.slug === authorSlug);
		}
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

.main {
	margin-top: 0;
	margin-bottom: 0;
	padding-bottom: 0;
}

main {
	max-width: 1035px;
	width: 100%;
	padding: 0 20px;

	h1 {
		font-weight: bold;
		font-size: 24px;
		line-height: 35px;
		color: #000624;
		margin-bottom: 1px;
		display: flex;
		align-items: center;

		button {
			height: fit-content;
			margin-top: 5px;
			margin-left: 8px;

			@media (min-width: $desktopBreakPoint) {
				display: none;
			}
		}
	}

	& > p {
		font-size: 16px;
		line-height: 24px;
		color: #5f5f68;
	}
}

.featured {
	margin-top: 48px;
	padding: 49px 11px 0;
	overflow: hidden;
	position: relative;
	max-width: 970px;

	@media (min-width: $desktopBreakPoint) {
		padding: 70px 0 0 46px;
		display: flex;
		justify-content: space-between;
		flex-wrap: wrap;
	}

	& > .img {
		margin-top: 50px;

		@media (min-width: $desktopBreakPoint) {
			width: 367px;
			right: 0;
			bottom: 0;
		}

		@media (max-width: 1111px) {
			width: 100%;
		}

		img {
			width: 100%;
		}
	}

	.post {
		max-width: unset;
		width: 100%;

		@media (min-width: $desktopBreakPoint) {
			max-width: 470px;
		}
	}
}

.card {
	background: #ffffff;
	box-shadow: 10px 20px 81px rgb(111 118 138 / 8%);
	border-radius: 32px;
}

.dropdown-container {
	position: relative;

	.dropdown {
		position: absolute;
		background: #ffffff;
		box-shadow: 0px 2px 4px rgba(12, 26, 75, 0.04), 0px 4px 20px -2px rgba(50, 50, 71, 0.08);
		border-radius: 10px;
		padding: 24px;
		z-index: 1;
		width: 217px;
		margin-top: 4px;

		li {
			margin: 0 0 32px;
			font-size: 14px;
			line-height: 22px;
			color: #5f5f68;

			&:last-of-type {
				margin-bottom: 0;
			}
		}
	}
}
</style>
