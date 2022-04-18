<template>
	<div class="main">
		<div class="blog-post">
			<div class="post-page--head">
				<div class="breadcrumb">
					<nuxt-link tag="button" to="/blog">Blog</nuxt-link>

					<span class="breadcrumb--divider">|</span>
					<span class="breadcrumb__tag">{{ blogPost.primary_tag.name }}</span>
				</div>

				<div class="date">
					{{ blogPost.reading_time }} min read
					<span><img src="~/assets/images/ellipse.svg" alt="ellipse" /></span>
					{{ blogPost.published_at | date }}
				</div>
			</div>

			<h3 class="post-page--title">{{ blogPost.title }}</h3>

			<div class="post-page--author">
				<a :href="blogPost.primary_author.twitter ? 'http://twitter.com/' + blogPost.primary_author.twitter : ''" target="_blank" class="author">
					<div class="img">
						<img :src="blogPost.primary_author.profile_image" alt="author imge" />
					</div>
					<div>
						<h5>{{ blogPost.primary_author.name }}</h5>
						<p>{{ blogPost.primary_author.meta_title }} Convoy</p>
					</div>
				</a>
				<div>
					<p>Share to:</p>
					<ul class="socials">
						<li>
							<a
								rel="noopener noreferrer"
								:href="'https://twitter.com/intent/tweet/?text=' + blogPost.title + '%20from%20@fraindev&url=https://getconvoy.io/blog/' + blogPost.slug + '&via=frainDev'"
								target="_blank"
							>
								<img src="~/assets/images/twitter-grey-icon.svg" alt="twitter logo" />
							</a>
						</li>

						<li>
							<a rel="noopener noreferrer" :href="'https://www.linkedin.com/sharing/share-offsite/?mini=true&url=https://getconvoy.io/blog/' + blogPost.slug + ''" target="_blank">
								<img src="~/assets/images/linkedin-grey-icon.svg" alt="linkedin logo" />
							</a>
						</li>
					</ul>
				</div>
			</div>

			<div class="post-page--content">
				<main>
					<div class="post-page--body">
						<div v-html="blogPost.html"></div>
					</div>
				</main>
			</div>
		</div>

		<div class="more-posts">
			<h1>More Posts</h1>
			<div class="posts">
				<Post v-for="(post, index) in posts" :key="index" :post="post" />
			</div>
		</div>
	</div>
</template>

<script>
import { getPost, getTwoPosts } from '../../api/blog';
import Prism from 'prismjs';

export default {
	layout: 'blog',
	data: () => {
		return {};
	},
	async asyncData({ params }) {
		const blogPost = await getPost(params.slug);
		const posts = await getTwoPosts(params.slug);
		return { blogPost, posts };
	},
	mounted() {
		Prism.highlightAll();
	},
	head() {
		return {
			title: this.blogPost.title,
			__dangerouslyDisableSanitizers: ['meta', 'script'],
			meta: [
				{ hid: 'description', name: 'description', content: this.blogPost.excerpt },
				{
					hid: 'article:tag',
					name: 'article:tag',
					content: this.blogPost.primary_tag.name
				},
				{
					hid: 'twitter:label1',
					name: 'twitter:label1',
					content: 'Written by'
				},
				{
					hid: 'twitter:data1',
					name: 'twitter:data1',
					content: this.blogPost.primary_author.name
				},
				{
					hid: 'twitter:label2',
					name: 'twitter:label2',
					content: 'Filed under'
				},
				{
					hid: 'twitter:data2',
					name: 'twitter:data2',
					content: `Convoy`
				},
				{
					hid: 'apple-mobile-web-app-title',
					name: 'apple-mobile-web-app-title',
					content: this.blogPost.title
				},
				{ hid: 'og:title', name: 'og:title', content: this.blogPost.title },
				{ hid: 'og:site_name', name: 'og:site_name', content: 'Convoy' },
				{ hid: 'og:type', name: 'og:type', content: 'article' },
				{
					hid: 'og:description',
					name: 'og:description',
					content: this.blogPost.excerpt
				},
				{
					hid: 'og:url',
					name: 'og:url',
					content: `https://getconvoy.io/blog/${this.blogPost.slug}`
				},
				{
					hid: 'article:published_time',
					name: 'article:published_time',
					content: this.blogPost.published_at
				},
				{
					hid: 'article:modified_time',
					name: 'article:modified_time',
					content: this.blogPost.updated_at
				},
				{
					hid: 'article:publisher',
					name: 'article:publisher',
					content: 'http://twitter.com/' + this.blogPost.primary_author.twitter
				},
				{
					hid: 'twitter:title',
					name: 'twitter:title',
					content: this.blogPost.title
				},
				{
					hid: 'twitter:card',
					name: 'twitter:card',
					content: 'summary_large_image'
				},
				{
					hid: 'twitter:url',
					name: 'twitter:url',
					content: `https://getconvoy.io/blog/${this.blogPost.slug}`
				},
				{
					hid: 'twitter:text:title',
					name: 'twitter:text:title',
					content: this.blogPost.title
				},
				{
					hid: 'twitter:description',
					name: 'twitter:description',
					content: this.blogPost.excerpt
				},
				{
					hid: 'og:image',
					property: 'og:image',
					content: this.blogPost.feature_image
				},
				{
					hid: 'twitter:image',
					property: 'twitter:image',
					content: this.blogPost.feature_image
				},
				{
					hid: 'twitter:url',
					name: 'twitter:url',
					content: `https://getconvoy.io/blog/${this.postId}`
				}
			],
			link: [{ rel: 'canonical', href: `https://getconvoy.io/blog/${this.blogPost.slug}` }],
			script: [
				{
					innerHTML: `
				{
					"@context": "https://schema.org",
					"@type": "Article",
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
					"author": {
						"@type": "Person",
						"name": "${this.blogPost.primary_author.name}",
						"url": "http://twitter.com/${this.blogPost.primary_author.twitter}",
						"sameAs": []
					},
					"headline": "Introducing Convoy",
					"url": "https://getconvoy.io/blog/${this.blogPost.slug}",
					"datePublished": "${this.blogPost.published_at}",
					"dateModified": "${this.blogPost.updated_at}",
					"image": {
						"@type": "ImageObject",
						"url": "${this.blogPost.feature_image}",
						"width": 1400,
						"height": 1086
					},
					"keywords": "Convoy",
					"description": "${this.blogPost.excerpt}"",
					"mainEntityOfPage": {
						"@type": "WebPage",
						"@id": "https://getconvoy.io/"
					}
				}
			`,
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
	padding: 0;
}
.blog-post {
	max-width: 780px;
	width: 100%;
	margin: 0 auto;
	padding: 0 20px;
	@media (min-width: $desktopBreakPoint) {
		padding: 0;
	}
}

aside {
	position: sticky;
	top: 0;

	& > div.fix {
		position: fixed;
		top: 190px;
	}

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
		// move up since content list isn't available yet
		// margin-top: 40px;
		// padding-top: 16px;
		// border-top: 1px dashed rgba(7, 71, 166, 0.08);

		h3 {
			font-weight: bold;
			font-size: 14px;
			line-height: 17px;
			color: #000624;
		}
	}
}

main {
	max-width: 780px;
	width: 100%;

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
		width: 100%;
		flex-flow: column;

		@media (min-width: $desktopBreakPoint) {
			flex-flow: row;
			align-items: center;
			justify-content: space-between;
		}
		button {
			padding: 0;
			font-weight: 500;
		}
		.breadcrumb {
			font-weight: 500;
			font-size: 14px;
			line-height: 22px;
			color: #31323d;
			&__tag {
				color: #477db3;
			}
			&--divider {
				margin-left: 16px;
				margin-right: 16px;
			}
		}

		.date {
			font-weight: 500;
			font-size: 14px;
			line-height: 24px;
			color: #000624;
			display: flex;
			margin-top: 16px;
			img {
				width: 4px;
				height: 4px;
				margin: 0 7px 2px 7px;
			}
			@media (min-width: $desktopBreakPoint) {
				margin-top: unset;
			}
		}
	}

	&--loader {
		width: 80%;
		left: 10%;
		height: 5px;
		background: #e6e6e6;
		position: sticky;
		margin-bottom: 52px;
		overflow: hidden;

		&.fix {
			position: fixed;
			max-width: calc(1035px + 170px + 32px);
			top: 90px;
			z-index: 5;
		}

		div {
			position: absolute;
			background: #5cc685;
			left: 0;
			height: 100%;
		}
	}

	&--body {
		font-weight: 400;
		font-size: 16px;
		line-height: 24px;
		color: #31323d;
	}

	&--title {
		font-weight: 700;
		font-size: 24px;
		line-height: 32px;
		color: #000624;
		margin: 16px 0 40px 0;

		@media (min-width: $desktopBreakPoint) {
			font-weight: 700;
			font-size: 48px;
			line-height: 58px;
			margin: 31px 0 25px 0;
		}

		&.small {
			font-size: 24px;
		}
	}

	&--author {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		margin-bottom: 56px;

		@media (min-width: $desktopBreakPoint) {
			margin-bottom: 45px;
		}

		.author {
			display: flex;
			align-items: flex-start;
			img {
				width: 100% !important;
				border-radius: 50%;
				margin-right: 12px;
			}
		}
		.img {
			width: 40px;
			height: 40px;
			border-radius: 50%;
			background: #f5f5f5;
			margin-right: 16px;
			overflow: hidden;
			display: flex;
			align-items: center;
		}

		h5 {
			font-weight: 600;
			font-size: 14px;
			line-height: 22px;
			color: #477db3;
			margin-bottom: 3px;
		}

		p {
			font-weight: 400;
			font-size: 14px;
			line-height: 22px;
			color: #31323d;
			margin-bottom: 7px;
		}

		.socials {
			li {
				width: 32px;
				height: 32px;
			}
		}
	}
}

.more-posts {
	padding: 0 20px;
	max-width: 970px;
	margin: 80px auto 0;

	@media (min-width: $desktopBreakPoint) {
		margin-top: 130px;
	}

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
