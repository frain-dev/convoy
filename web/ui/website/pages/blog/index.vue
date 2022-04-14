<template>
	<div class="main blog-home">
		<aside class="categories">
			<ul>
				<h3>CATEGORIES</h3>

				<li v-for="(tag, index) in tags" :key="'tag' + index">
					<nuxt-link :to="'/blog?tag=' + tag.slug">{{ tag.name }}</nuxt-link>
				</li>
			</ul>

			<!-- Pending when there is enough content for this -->
			<!-- <form>
				<img src="~/assets/images/search-icon.svg" alt="search icon" />
				<input type="search" placeholder="Search" />
			</form> -->

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
					<li v-for="(tag, index) in tags" :key="'tag' + index">
						<nuxt-link :to="'/blog?tag=' + tag.slug">{{ tag.name }}</nuxt-link>
					</li>
				</ul>
			</div>

			<div class="featured card posts" v-if="featurePosts.length > 0 && !tag">
				<div class="post">
					<div class="post--head">
						<div class="tag">FEATURED</div>
						<div class="date">{{ featurePosts[0].published_at | date }}</div>
					</div>
					<nuxt-link :to="'/blog/' + featurePosts[0].slug">
						<h3 class="post--title single-feature">{{ featurePosts[0].title }}</h3>
					</nuxt-link>
					<p class="post--body single-feature">{{ featurePosts[0].excerpt }}...</p>
					<div class="post--footer single-feature">
						<a :href="featurePosts[0].primary_author.twitter ? 'http://twitter.com/' + featurePosts[0].primary_author.twitter : ''" target="_blank" class="post--author">
							<div class="img">
								<img :src="featurePosts[0].primary_author.profile_image" alt="author imge" />
							</div>
							<div>
								<h5>{{ featurePosts[0].primary_author.name }}</h5>
								<p>{{ featurePosts[0].primary_author.meta_title }} Convoy</p>
							</div>
						</a>
						<nuxt-link :to="'/blog/' + featurePosts[0].slug">
							Read More
							<img src="~/assets/images/angle-right-primary.svg" alt="read more icon" />
						</nuxt-link>
					</div>
				</div>
				<div class="img">
					<img :src="featurePosts[0].feature_image" alt="featured post img" />
				</div>
			</div>

			<div class="posts">
				<Post v-for="(post, index) in posts.slice(0, 2)" :key="index" :post="post" />
			</div>

			<div class="newsletter card">
				<div>
					<h5>Join our newsletter</h5>
					<p>No spam! Just articles, events, and talks.</p>
					<form @submit.prevent="requestAccess()">
						<img src="~/assets/images/mail-primary-icon.svg" alt="mail icon" />
						<input type="email" id="email" placeholder="Your email" aria-label="Email" v-model="earlyAccessEmail" />
						<button>
							<img src="~/assets/images/send-primary-icon.svg" alt="send icon" />
						</button>
					</form>
				</div>
				<img src="~/assets/images/mailbox.gif" alt="mailbox animation" />
			</div>

			<div class="posts">
				<Post v-for="(post, index) in posts.slice(2)" :key="index" :post="post" />
			</div>
		</main>
	</div>
</template>

<script>
import { getFeaturedPosts, getTags, getTagPosts, getLimitedPosts } from '../../api/blog';

export default {
	layout: 'blog',
	watch: {
		async '$route.query'(route) {
			this.posts = await getTagPosts(route.tag);
		}
	},
	data: () => {
		return {
			showCategories: false,
			earlyAccessEmail: '',
			isSubmitingloadingEarlyAccessForm: false
		};
	},
	async asyncData({ route }) {
		const posts = route.query?.tag ? await getTagPosts(route.query?.tag) : await getLimitedPosts();
		const featurePosts = await getFeaturedPosts();
		const tags = await getTags();
		return { posts, featurePosts, tags, tag: route.query?.tag };
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
		},
		author(authorSlug) {
			return this.authors.find(author => author.slug === authorSlug);
		}
	},
	head() {
		return {
			__dangerouslyDisableSanitizers: ['meta', 'script'],
			script: [
				{
					innerHTML: `
				{
					"@context": "https://schema.org",
					"@type": "WebSite",
					"publisher": {
						"@type": "Organization",
						"name": "Convoy",
						"url": "https://getconvoy.io/blog",
						"logo": {
							"@type": "ImageObject",
							"url": "https://getconvoy.io/favicon.ico",
							"width": 48,
							"height": 48
						}
					},
					"mainEntityOfPage": {
						"@type": "WebPage",
						"@id": "https://getconvoy.io/blog"
					},
					"description": "A Cloud native Webhook Service with out-of-the-box security, reliability and scalability for your webhooks infrastructure.",
					"url": "https://getconvoy.io/blog"
				}`,
					type: 'application/ld+json'
				},
				{
					type: 'application/rss+xml',
					rel: 'alternate',
					title: 'Convoy RSS Feed',
					href: 'https://getconvoy.io/blog/rss'
				},
				{
					type: 'application/json',
					rel: 'alternate',
					title: 'Convoy Json Feed',
					href: 'https://getconvoy.io/blog/json'
				}
			]
		};
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

.main {
	margin: 0 auto;
	padding-bottom: 0;
	display: flex;
	justify-content: space-between;
	height: unset;
	padding-top: 0;
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
	align-items: flex-end;

	@media (min-width: $desktopBreakPoint) {
		padding: 56px 0 0 56px;
		display: flex;
		justify-content: space-between;
		flex-wrap: wrap;
	}

	& > .img {
		margin-top: 20px;

		@media (min-width: $desktopBreakPoint) {
			width: 380px;
			right: 0;
			bottom: 0;
			margin-top: 0;

			img {
				border-radius: 10px 0 0 0;
			}
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
		box-shadow: unset;

		@media (min-width: $desktopBreakPoint) {
			max-width: 470px;
		}
	}
}

.card {
	background: #ffffff;
	box-shadow: 10px 20px 81px rgb(111 118 138 / 8%);
	border-radius: 8px;
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
