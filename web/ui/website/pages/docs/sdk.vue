<template>
	<div class="nuxt-content">
		<h1>SDK</h1>
		<p>
			This is the Convoy SDK. This SDK contains methods for easily interacting with Convoy's API. Below are examples to get you started in Javascript, PHP, or Python. For additional examples, please see
			our official documentation at (
			<a href="https://convoy.readme.io/reference" target="_blank">https://convoy.readme.io/reference</a>
			)
		</p>
		<div class="tabs tabs__line margin-top__32px">
			<li v-for="tab of tabs" :key="tab.id">
				<button class="has-icon" :class="activeTab === tab.id ? 'active' : ''" @click="switchTabs(tab.id)">
					<img :src="require(`~/assets/images/${tab.id}.svg`)" :alt="tab.label" />
					<span>{{ tab.label }}</span>
				</button>
			</li>
		</div>
		<div>
			<nuxt-content :document="pageData"></nuxt-content>
		</div>
	</div>
</template>
<script>
export default {
	layout: 'docs',
	data() {
		return {
			pageData: '',
			tabs: [
				{ label: 'Javascript', id: 'javascript' },
				{ label: 'Python', id: 'python' },
				{ label: 'PHP', id: 'php' }
				// { label: 'Ruby', id: 'ruby' }
			],
			activeTab: 'javascript'
		};
	},
	mounted() {
		this.fetchPageData('convoyjs#installation');
	},
	methods: {
		async fetchPageData(param) {
			const pageData = await this.$content('docs/' + param).fetch();
			this.pageData = pageData;
		},
		switchTabs(activeTab) {
			switch (activeTab) {
				case 'javascript':
					this.activeTab = 'javascript';
					this.fetchPageData('convoyjs');
					break;
				case 'python':
					this.activeTab = 'python';
					this.fetchPageData('convoy-pyhton');
					break;
				case 'php':
					this.activeTab = 'php';
					this.fetchPageData('convoy-php');
					break;
				case 'ruby':
					this.activeTab = 'ruby';
					this.fetchPageData('convoy-ruby');
					break;
				default:
					break;
			}
		}
	}
};
</script>
<style lang="scss" scoped></style>
