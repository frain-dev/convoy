<template>
    <div class="page">
        <aside>
            <div class="logo">
                Convoy.
                <span>Docs</span>
            </div>

            <nuxt-link to="/docs"><h3>GETTING STARTED</h3></nuxt-link>

            <ul>
                <h3>API Docs</h3>
                <li v-for="(page, index) in pages" :key="index">
                    <nuxt-link :to="page.path"> <img src="~/assets/images/angle-right-icon.svg" alt="angle right" />{{ page.title }}</nuxt-link>
                </li>
            </ul>
        </aside>

        <div class="main">
            <header>
                <DocsSearch />

                <div>
                    <a href="#">
                        <img src="~/assets/images/github-logo.svg" alt="github icon" />
                    </a>
                </div>
            </header>

            <main class="page--container">
                <nuxt-content :document="pageData"></nuxt-content>
            </main>
        </div>
    </div>
</template>

<script>
export default {
    async asyncData({ $content, params }) {
        try {
            const pageData = await $content('docs', params.slug || 'index').fetch();
            const pages = await $content('docs').only(['title']).fetch();
            return { pageData, pages };
        } catch (error) {
            const pages = await $content('docs').only(['title']).fetch();
            const pageData = await $content('404').fetch();
            return { pageData, pages };
        }
    },
    methods: {
        getPathKeys(paths) {
            const response = typeof paths === 'object' ? Object?.keys(paths) : paths;
            return response;
        },
        getBody(paths) {
            const response = typeof paths === 'object' ? Object?.values(paths) : paths;
            return response;
        },
    },
};
</script>

<style lang="scss" scoped>
.page {
    display: flex;
    height: 100vh;
}

aside {
    max-width: 270px;
    width: 100%;
    background: #16192c;
    color: #ffffff;

    .logo {
        font-weight: bold;
        font-size: 21px;
        line-height: 26px;
        color: #ffffff;
        padding: 20px 24px;
        border-bottom: 1px solid rgba(236, 233, 241, 0.1);

        span {
            font-weight: 500;
            font-size: 16px;
            line-height: 20px;
            color: #47b38d;
        }
    }

    a.nuxt-link-exact-active {
        color: #ffffff;
        font-weight: bold;

        h3 {
            color: #ffffff;
        }
    }

    h3 {
        font-weight: bold;
        font-size: 14px;
        line-height: 17px;
        font-variant: small-caps;
        color: rgba(255, 255, 255, 0.5);
        margin-bottom: 0;
        padding: 24px 0 0 24px;
    }

    & > ul {
        padding: 24px 0 24px 24px;

        h3 {
            padding: 0;
            margin-bottom: 16px;
            margin-top: 0;
        }

        li {
            font-size: 14px;
            line-height: 16px;
            margin-bottom: 16px;

            a,
            button {
                display: flex;
                align-items: center;
            }

            img {
                width: 16px;
                margin-right: 10px;
            }
        }

        ul {
            margin: 16px 0 16px 40px;
        }
    }
}

.main {
    width: 100%;
    overflow-y: auto;
}

header {
    padding: 13px 24px;
    background: #ffffff;
    display: flex;
    align-items: center;
    justify-content: space-between;
}

.page--container {
    padding: 36px 32px;
    max-width: 900px;
    width: 100%;
    margin: auto;
}
</style>
