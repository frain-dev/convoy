<template>
    <div class="page">
        <aside>
            <div class="logo">
                Courier.
                <span>Docs</span>
            </div>

            <ul>
                <h3>GETTING STARTED</h3>
            </ul>

            <ul>
                <h3>API</h3>
                <!-- <li v-for="(tag, index) in pageData.tags" :key="index">
                    <a href="#"> <img src="~/assets/images/angle-right-icon.svg" alt="angle right" />{{ tag.name }}</a>
                </li> -->
            </ul>

            <ul>
                <h3>CLI</h3>
                <!-- <li v-for="(tag, index) in pageData.tags" :key="index">
                    <a href="#"> <img src="~/assets/images/angle-right-icon.svg" alt="angle right" />{{ tag.name }}</a>
                </li> -->
            </ul>
        </aside>

        <div class="main">
            <header>
                <form>
                    <div class="input">
                        <img src="~/assets/images/search-icon.svg" alt="search icon" />
                        <input type="search" aria-label="search" id="search" name="search" placeholder="Search documentation" />
                    </div>
                </form>

                <div>
                    <a href="#">
                        <img src="~/assets/images/github-logo.svg" alt="github icon" />
                    </a>
                </div>
            </header>

            <main class="page--container">
                <h1>Quickstart</h1>
                <p>Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore.</p>

                <h2>Setup</h2>
                <p>
                    Duis iaculis quisque proin placerat vel ut feugiat dui. Elit placerat eget tincidunt egestas donec iaculis pharetra, eu egestas. Lacus varius lectus enim facilisi pharetra, consectetur arcu
                </p>

                <section v-for="(path, index) in apiData" :key="index">
                    <h2>{{ path.title }}</h2>
                    <div class="path"><img src="~/assets/images/link-icon.svg" alt="link icon" />{{ path.link }}</div>
                    <div class="code-snippet--title" v-if="path.requestBody">Request body (schema)</div>
                    <pre v-if="path.requestBody"><code class="language-json">{{ path.requestBody }}</code></pre>

                    <div class="code-snippet--title" v-if="path.requestBody">Request Responses (sample)</div>
                    <pre v-for="(response, index) in path.responses"><code class="language-json">{{ response.body }}</code></pre>
                    <!-- <p v-for="(item, index) in getPathKeys(pageData.paths[path][subPath].responses)">
                            {{ pageData.paths[path][subPath].responses[item].content }}
                            {{ getBody(pageData.paths[path][subPath].responses[item].content) }} -->
                    <!-- <p v-for="(itit, index) in getPathKeys(pageData.paths[path][subPath].responses[item].content)">{{itit}}</p></p> -->
                    <!-- {{ pageData.paths[path][subPath] }} -->
                    <!-- {{ getPathKeys(pageData.paths[path][subPath].responses) }} -->
                    <!-- </p> -->
                    <!-- {{ pageData.paths[path][subPath].responses['0'] }} -->
                    <!-- </div> -->

                    <!-- {{ pageData.paths[path] }} -->
                </section>
            </main>
        </div>
    </div>
</template>

<script>
import Prism from '~/plugins/prism';

export default {
    data() {
        return {
            something: [],
            est: 200,
        };
    },
    async asyncData({ $content, params }) {
        const pageData = await $content('api-doc').fetch();
        const keys = (item) => {
            const response = typeof item === 'object' ? Object?.keys(item) : item;
            return response;
        };
        const values = (item) => {
            const response = typeof item === 'object' ? Object?.values(item) : item;
            return response;
        };
        const newPaths = [];
        const paths = keys(pageData.paths);
        const pathItem = pageData.paths[paths[0]];

        paths.forEach((path) => {
            const pathItems = pageData.paths[path];
            const pathMethods = keys(pathItems);
            pathMethods.forEach((method) => {
                if (method !== 'parameters') {
                    const pathItem = {
                        title: pathItems[method].summary,
                        link: path,
                        requestBody: pathItems[method].requestBody ? pathItems[method].requestBody.content['application/json'].schema : null,
                        responses: [],
                    };

                    const responseCodes = keys(pathItems[method].responses);
                    const responseBodies = values(pathItems[method].responses);
                    responseCodes.forEach((code) => {
                        const response = {
                            code,
                            body: responseBodies[0].content ? pageData.components.schemas[responseBodies[0].content['application/json'].schema.$ref.split('/')[3]] : null,
                        };
                        pathItem.responses.push(response);
                    });
                    newPaths.push(pathItem);
                }
            });
        });
        const apiData = newPaths;

        return { apiData };
    },
    mounted() {
        Prism.highlightAll();
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

    & > ul {
        padding: 24px 0 24px 24px;

        h3 {
            font-weight: bold;
            font-size: 14px;
            line-height: 17px;
            font-variant: small-caps;
            color: rgba(255, 255, 255, 0.5);
            margin-bottom: 16px;
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

    form {
        max-width: 378px;
        width: 100%;
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
        }
    }
}

.page--container {
    padding: 36px 32px;
    max-width: 900px;
    width: 100%;

    h1 {
        font-weight: bold;
        font-size: 32px;
        line-height: 32px;
        letter-spacing: 0.01em;
        margin-bottom: 16px;
    }

    h2 {
        font-weight: bold;
        font-size: 24px;
        line-height: 32px;
        letter-spacing: 0.01em;
        margin: 32px 0 16px;

        & + .path {
            margin-top: -10px;
            font-size: 14px;
            display: flex;
            align-items: center;
            background: rgba(115, 122, 145, 0.16);
            width: fit-content;
            padding: 2px 10px;
            border-radius: 10px;
            margin-bottom: 20px;

            img {
                width: 15px;
                margin-right: 10px;
            }
        }
    }

    p {
        margin-bottom: 32px;
        font-size: 16px;
        line-height: 27px;
    }
}
</style>
