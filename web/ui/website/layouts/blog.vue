<template>
	<div class="page blog">
		<header>
			<Header></Header>
		</header>

		<div class="main">
			<Nuxt />
		</div>
		<Footer></Footer>
	</div>
</template>

<script>
export default {
	data: () => {
		return {
			showMenu: false,
			pages: []
		};
	},
	async mounted() {
		let pages = await this.$content('docs').only(['title', 'id', 'toc', 'order']).sortBy('order', 'asc').fetch();
		pages = pages.sort((a, b) => a.order - b.order);
		this.pages = pages;
	}
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

.page.blog {
	flex-wrap: wrap;
	height: 100vh;
	font-family: 'Inter', sans-serif !important;
}

header {
	width: 100%;
	background: transparent;
	margin-top: -110px;
}

.main {
	margin: calc(20px + 32px + 58.23px) auto 0;
	padding: 150px 0 100px;
	width: 100%;
	max-width: calc(1035px + 170px + 32px);
}
</style>
