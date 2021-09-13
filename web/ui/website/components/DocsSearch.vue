<template>
    <div class="docs-search">
        <form>
            <div class="input">
                <img src="~/assets/images/search-icon.svg" alt="search icon" />
                <input v-model="searchQuery" autocomplete="off" type="search" aria-label="search" id="search" name="search" placeholder="Search documentation" />
            </div>
        </form>
        <ul v-if="articles.length" class="docs-search--dropdown">
            <li v-for="article of articles" :key="article.slug">
                <NuxtLink :to="'/docs/' + article.slug">
                    <img src="~/assets/images/link-icon-2.svg" alt="link icon" />
                    {{ article.title }}
                </NuxtLink>
            </li>
        </ul>
    </div>
</template>

<script>
export default {
    data() {
        return {
            searchQuery: '',
            articles: [],
        };
    },
    watch: {
        async searchQuery(searchQuery) {
            if (!searchQuery) {
                this.articles = [];
                return;
            }
            this.articles = await this.$content('docs').search(searchQuery).fetch();
        },
    },
};
</script>

<style lang="scss" scoped>
.docs-search {
    max-width: 378px;
    width: 100%;
    position: relative;
}

.docs-search--dropdown {
    position: absolute;
    background: #fff;
    width: 100%;
    border: 1px solid #edeff5;
    border-radius: 8px;
    top: 50px;

    li {
        padding: 15px 20px;
        font-size: 14px;

        img {
            width: 12px;
            margin-right: 10px;
        }

        &:not(:last-of-type) {
            border-bottom: 1px solid #eee;
        }
    }
}

.input {
    display: flex;
    align-items: center;
    background: #ffffff;
    border: 1px solid rgba(115, 122, 145, 0.16);
    box-sizing: border-box;
    border-radius: 4px;
    padding: 11px 16px;
    width: 100%;

    img {
        width: 18px;
    }

    input {
        border: none;
        margin-left: 10px;
        width: 100%;
        outline: none;
    }
}
</style>
