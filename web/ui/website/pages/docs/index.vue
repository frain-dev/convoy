<template>
    <nuxt-content :document="pageData"></nuxt-content>
</template>

<script>
export default {
    layout: 'docs',
    async asyncData({ $content, params }) {
        const pageData = await $content('docs', 'index').fetch();
        return { pageData };
    },
};
</script>

<style lang="scss" scoped></style>
