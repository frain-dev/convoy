import GhostContentAPI from '@tryghost/content-api';

const api = new GhostContentAPI({
	url: 'https://convoy.ghost.io',
	key: 'b9904af5cf9365f3c647cf2d8b',
	version: 'v3'
});

export async function getPosts() {
	const posts = await api.posts
		.browse({
			limit: 'all',
			include: 'tags,authors',
			order: 'published_at DESC'
		})
		.catch(err => {
			console.error(err);
		});

	return posts;
}

export async function getLimitedPosts() {
	const posts = await api.posts
		.browse({
			limit: 'all',
			include: 'tags,authors',
			order: 'published_at DESC',
			filter: `featured:false`
		})
		.catch(err => {
			console.error(err);
		});

	return posts;
}

export async function getFeaturedPosts() {
	const posts = await api.posts
		.browse({
			limit: 'all',
			include: 'tags,authors',
			order: 'published_at DESC',
			filter: `featured:true`
		})
		.catch(err => {
			console.error(err);
		});

	return posts;
}

export async function getTagPosts(query) {
	const posts = await api.posts
		.browse({
			limit: 'all',
			include: 'tags,authors',
			order: 'published_at DESC',
			filter: `primary_tag:${query}`
		})
		.catch(err => {
			console.error(err);
		});

	return posts;
}

export async function getTwoPosts() {
	const posts = await api.posts
		.browse({
			limit: '2',
			include: 'tags,authors',
			order: 'published_at DESC'
		})
		.catch(err => {
			console.error(err);
		});

	return posts;
}

export async function getPost(postSlug) {
	const post = await api.posts
		.read({
			slug: postSlug,
			include: 'tags,authors'
		})
		.catch(err => {
			console.error(err);
		});
	return post;
}

export async function getTags() {
	const tags = await api.tags.browse().catch(err => {
		console.error(err);
	});
	return tags;
}
