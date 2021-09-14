<template>
    <nuxt-content :document="pageData"></nuxt-content>
</template>

<script>
export default {
    layout: 'docs',
    async asyncData({ $content, params }) {
        try {
            const pageData = await $content('docs/' + params.slug || 'index').fetch();
            return { pageData };
        } catch (error) {
            const pageData = await $content('404').fetch();
            return { pageData };
        }
    },
};
</script>

<style lang="scss" scoped></style>
