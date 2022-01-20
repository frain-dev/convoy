<template>
	<div class="post">
		<div class="post--img">
			<img :src="'https://res.cloudinary.com/frain/image/upload/c_fill,g_north,h_179,w_461,x_0,y_0/' + post.thumbnail" alt="post image" />
		</div>
		<div class="tag clear">{{ post.tag }}</div>
		<nuxt-link :to="'/blog/' + post.slug">
			<h3 class="post--title small">{{ post.title }}</h3>
		</nuxt-link>
		<p class="post--body">{{ post.description }}</p>
		<div class="post--footer">
			<div class="post--author">
				<!-- Pending when we have icon for authors -->
				<!-- <img src="~/assets/images/author-img.png" alt="author imge" /> -->
				<div>
					<h5>{{ author(post.author).name }}</h5>
					<p>{{ author(post.author).role }} Convoy</p>
				</div>
			</div>
			<nuxt-link :to="'/blog/' + post.slug">
				Read More
				<img src="~/assets/images/angle-right-primary.svg" alt="read more icon" />
			</nuxt-link>
		</div>
	</div>
</template>

<script>
export default {
	data() {
		return {
			showMenu: false
		};
	},
	props: ['post', 'authors'],
	methods: {
		author(authorSlug) {
			return this.authors.find(author => author.slug === authorSlug);
		}
	}
};
</script>

<style lang="scss" scoped></style>
