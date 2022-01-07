<template>
	<div class="main">
		<div class="post-page--head">
			<nuxt-link tag="button" to="/blog" class="back-button">
				<img src="~/assets/images/angle-left-black-icon.svg" alt="back icon" />
			</nuxt-link>
			<div class="tag">{{ blogPost.tag }}</div>
			<div class="date">{{ blogPost.date | date }}</div>
		</div>

		<h3 class="post-page--title">{{ blogPost.title }}</h3>

		<div class="post-page--author">
			<img src="~/assets/images/author-img.png" alt="author imge" />
			<div>
				<h5>{{ author(blogPost.author).name }}</h5>
				<p>{{ author(blogPost.author).role }} Convoy</p>
			</div>
		</div>

		<div class="post-page--loader"></div>

		<div class="post-page--content">
			<aside>
				<ul>
					<h3>CONTENTS</h3>

					<li v-for="(heading, index) in blogPost.toc" :key="'heading' + index">
						<nuxt-link :to="{ path: '/blog/' + blogPost.slug, hash: '#' + heading.id }">{{ heading.text }}</nuxt-link>
					</li>
				</ul>

				<div class="social">
					<h3>Share Via</h3>

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
				<div class="post-page--body">
					<nuxt-content :document="blogPost"></nuxt-content>
				</div>
			</main>
		</div>

		<div class="more-posts">
			<h1>More Posts</h1>
			<div class="posts">
				<div class="post" v-for="(post, index) in posts" :key="'post' + index">
					<div class="post--img">
						<img src="~/assets/images/post-img.png" alt="post image" />
					</div>
					<div class="post--head">
						<div class="tag clear">FEATURED</div>
					</div>
					<h3 class="post--title small">Understanding The Convoy UI</h3>
					<p class="post--body">
						One of the major issues and problems of webhook over the years has been the ability to monitor and understand the state of your webhooks service at any time, and that’s a major part of
						what we’re solving with Convoy as opposed to waiting for users to report failures before…
					</p>
					<div class="post--footer">
						<div class="post--author">
							<img src="~/assets/images/author-img.png" alt="author imge" />
							<div>
								<h5>Emmanuel Aina</h5>
								<p>Co-Founder Convoy</p>
							</div>
						</div>
						<nuxt-link to="#">
							Read More
							<img src="~/assets/images/angle-right-primary.svg" alt="read more icon" />
						</nuxt-link>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>

<script>
export default {
	layout: 'blog',
	data: () => {
		return {};
	},
	async asyncData({ $content, params }) {
		try {
			const blogPost = await $content('blog/' + params.slug || 'index').fetch();
			const morePosts = await $content('blog').only(['author', 'description', 'slug', 'thumbnail', 'title', 'date', 'tag']).fetch();
			const posts = morePosts.slice(0, 3);
			const authors = await $content('blog-authors').fetch();
			return { blogPost, authors, posts };
		} catch (error) {
			console.log('🚀 ~ file: _slug.vue ~ line 173 ~ asyncData ~ error', error);
			// const pageData = await $content('404').fetch();
			// return { pageData };
		}
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
	margin: 0 auto;
	display: block;
	height: unset;
	padding-top: 0;
	max-width: calc(1035px + 170px + 32px);
	padding-bottom: 100px;
}

aside {
	position: sticky;
	top: 0;

	h3 {
		font-size: 14px;
		line-height: 17px;
	}

	li {
		margin-bottom: 16px;
		font-size: 13px;
		line-height: 16px;
	}

	.social {
		margin-top: 40px;
		padding-top: 16px;
		border-top: 1px dashed rgba(7, 71, 166, 0.08);

		h3 {
			font-weight: bold;
			font-size: 14px;
			line-height: 17px;
			color: #000624;
		}
	}
}

main {
	max-width: 750px;
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

.post-page {
	.date {
		font-weight: 500;
		font-size: 14px;
		line-height: 22px;
	}

	&--head {
		display: flex;
		align-items: center;
		max-width: 320px;
		width: 100%;
		justify-content: space-between;
		padding: 0 20px;

		button {
			padding: 0;
		}

		.tag {
			font-weight: bold;
			font-size: 15px;
			line-height: 140%;
			text-transform: uppercase;
		}

		.date {
			font-weight: 500;
			font-size: 14px;
			line-height: 24px;
		}
	}

	&--loader {
		width: 100%;
		height: 5px;
		background: #e6e6e6;
		position: relative;
		margin-bottom: 52px;

		&::after {
			width: 40%;
			content: '';
			position: absolute;
			background: #5cc685;
			left: 0;
			height: 100%;
		}
	}

	&--body {
		font-size: 16px;
		line-height: 24px;
		color: #5f5f68;

		ul {
			list-style-type: disc;
			margin-left: 20px;
			margin-bottom: 24px;

			li {
				list-style-type: disc;
				margin-bottom: 15px;
			}
		}

		p {
			font-size: 16px;
			line-height: 24px;
			margin-bottom: 24px;
			color: #737a91;
		}

		h1 {
			font-size: 26px;
		}

		h2 {
			font-size: 24px;
		}

		h3 {
			font-size: 20px;
		}

		h4 {
			font-size: 18px;
		}

		h5 {
			font-size: 16px;
		}

		h6 {
			font-size: 14px;
		}

		h3,
		h1,
		h2,
		h4,
		h5,
		h6 {
			font-weight: bold;
			line-height: 32px;
			margin-bottom: 24px;
		}

		img {
			margin: 0;
		}

		a {
			color: #477db3;
		}

		blockquote {
			border-radius: 16px;
			padding: 100px 64px 64px;
			background: url('~assets/images/blockquote-bg.svg') no-repeat #477db3;
			background-position: top right;
			margin: 0 0 44px;
			position: relative;

			&::after {
				position: absolute;
				content: url('~assets/images/quote-left.svg');
				top: 67px;
				left: 50%;
				transform: translate(0, -50%);
			}

			p {
				font-size: 26px;
				line-height: 60px;
				text-align: center;
				letter-spacing: 0.09px;
				color: #ffffff;
			}
		}
	}

	&--title {
		font-weight: bold;
		line-height: 42px;
		color: #16192c;
		font-size: 24px;
		padding: 0 20px;
		margin: 60px 0 40px;

		@media (min-width: $desktopBreakPoint) {
			font-size: 48px;
			margin: 35px 0 24px 55px;
			line-height: 58px;
		}

		&.small {
			font-size: 24px;
		}
	}

	&--author {
		display: flex;
		align-items: flex-start;
		padding: 0 20px;
		margin-bottom: 56px;

		@media (min-width: $desktopBreakPoint) {
			margin-left: 55px;
			margin-bottom: 45px;
		}

		img {
			width: 40px;
			margin-right: 12px;
		}

		h5 {
			font-weight: 500;
			font-size: 16px;
			line-height: 20px;
			margin-bottom: 3px;
		}

		p {
			font-size: 12px;
			line-height: 20px;
			color: #31323d;
		}
	}

	&--content {
		display: flex;
	}
}

.more-posts {
	padding: 0 20px;
	max-width: 970px;
	margin: 100px auto 0;

	h1 {
		font-weight: bold;
		font-size: 32px;
		line-height: 130%;
	}

	.posts {
		margin-top: 32px;
		justify-content: center;
	}
}
</style>
