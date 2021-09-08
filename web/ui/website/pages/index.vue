<template>
    <div class="page">
        <header>
            <nav>
                <div>
                    <div class="logo">
                        <img src="~/assets/images/logo.svg" alt="logo" />
                    </div>

                    <button class="menu-button" @click="showMenu = !showMenu">
                        <img src="~/assets/images/menu-icon.svg" alt="menu icon" />
                    </button>

                    <ul :class="showMenu ? 'show' : ''">
                        <li>
                            <a href="#features"> Features </a>
                        </li>
                        <li>
                            <a href="https://github.com/frain-dev/convoy/discussions"> Community </a>
                        </li>
                        <li>
                            <a href="https://github.com/frain-dev/convoy">
                                <img src="~/assets/images/github-logo.svg" alt="github logo" />
                            </a>
                        </li>
                    </ul>
                </div>
            </nav>

            <section class="hero-section">
                <div class="hero-section--cta">
                    <h1>A Cloud native Webhook Service</h1>
                    <p>With out-of-the-box security, reliability and scalability for your webhooks infrastructure.</p>
                    <!-- <button>Get Started</button> -->
                </div>
            </section>
        </header>

        <section class="section features" id="features">
            <div class="container">
                <h2 class="section--title">Why Convoy?</h2>

                <ul>
                    <li>
                        <img src="~/assets/images/secure-feature-icon.svg" alt="secure feature icon" />
                        <h4>Secure</h4>
                        <p>Create secrets, sign payload, verify event. Increase security by enabling rolling secrets.</p>
                    </li>
                    <li>
                        <img src="~/assets/images/ease-feature-icon.svg" alt="ease feature icon" />
                        <h4>Ease of use</h4>
                        <p>Easily filter & debug events sent to multiple applications & endpoints with Delivery Attempt Logs</p>
                    </li>
                    <li>
                        <img src="~/assets/images/cloud-feature-icon.svg" alt="cloud feature icon" />
                        <h4>Cloud & Language Agnostic</h4>
                        <p>Deploy to any environment and send events from any language (Ruby, Golang, JavaScript etc)</p>
                    </li>
                    <li>
                        <img src="~/assets/images/rate-feature-icon.svg" alt="rate feature icon" />
                        <h4>Rate Limiting</h4>
                        <p>Avoid blasting too much events to a single endpoint at once with flexible rate limiting controls</p>
                    </li>
                    <li>
                        <img src="~/assets/images/scalable-feature-icon.svg" alt="scalable feature icon" />
                        <h4>Scalable</h4>
                        <p>Independently scale Convoy as your system needs grows.</p>
                    </li>
                    <li>
                        <div class="features--dashboard-screenshot">
                            <img src="~/assets/images/dashboard-img.png" alt="dashboard image" />
                        </div>
                    </li>
                </ul>
                <div class="features--dashboard-screenshot stand-alone">
                    <img src="~/assets/images/dashboard-img.png" alt="dashboard image" />
                </div>
            </div>
        </section>

        <section class="section core-features">
            <div class="container">
                <div>
                    <h2 class="section--title">Core Features</h2>
                    <ul>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Independently scalable</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Signatures & Rolling secrets</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Retries</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Clustering</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Language Agnostic</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Idempotency</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>URL per Event Type</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Static IPs</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Rich UI - Event Logs & Querying.</p>
                        </li>
                        <li>
                            <img src="~/assets/images/feature-check-icon.svg" alt="check icon" />
                            <p>Payload Filtering</p>
                        </li>
                    </ul>
                </div>
                <div>
                    <div class="core-features--img-container">
                        <img src="~/assets/images/core-features-img.svg" alt="core flow" />
                    </div>
                </div>
            </div>
        </section>

        <footer>
            <div class="container">
                <nav>
                    <div class="logo">
                        <img src="~/assets/images/logo.svg" alt="logo" />
                    </div>

                    <ul>
                        <li>
                            <a href="https://twitter.com/getconvoy"><img src="~/assets/images/twitter-icon.svg" alt="twitter logovc" /></a>
                        </li>
                    </ul>
                </nav>
                <p>Copyright 2021, All Rights Reserved</p>
            </div>
        </footer>
    </div>
</template>

<script>
export default {
    data() {
        return {
            showMenu: false,
        };
    },
    async asyncData({ $content, params }) {
        const pageData = await $content('api-doc').fetch();
        return { pageData };
    },
    methods: {},
};
</script>

<style lang="scss" scoped>
$desktopBreakPoint: 880px;

header {
    background: url('~/assets/images/header-bg-pattern.svg') #1e252b no-repeat;
    background-size: cover;
    padding: 32px 20px 0;

    nav {
        max-width: 1376px;
        width: 100%;
        margin: auto;
        background: #222a31;
        border-radius: 16px;
        padding: 12px 20px;
        position: relative;

        & > div {
            display: flex;
            align-items: center;
            justify-content: space-between;
            max-width: 1106px;
            margin: auto;
        }

        .logo {
            font-weight: bold;
            font-size: 21px;
            line-height: 26px;
            color: #ffffff;
            width: 80%;

            img {
                width: 140px;
            }
        }

        .menu-button {
            display: block;

            img {
                width: 40px;
            }

            @media (min-width: $desktopBreakPoint) {
                display: none;
            }
        }

        ul {
            transition: all 0.5s;
            display: none;
            position: absolute;
            top: 55px;
            left: 0;
            background: #222a31;
            width: 100%;
            text-align: left;
            height: 0;
            overflow-y: hidden;

            &.show {
                padding-top: 20px;
                height: 360px;
                overflow-y: auto;
                display: block;
            }

            li {
                width: 100%;
                padding: 15px 20px;

                &:not(:last-of-type) {
                    margin-right: 40px;
                }

                a {
                    font-weight: 500;
                    font-size: 14px;
                    line-height: 17px;
                    color: #ffffff;
                    width: 100%;

                    img {
                        width: 24px;
                    }
                }

                button {
                    background: #477db3;
                    border-radius: 8px;
                    padding: 9px 29px;
                    color: #ffffff;
                    font-weight: 500;
                    font-size: 16px;
                    line-height: 28px;
                    min-width: 146px;
                }
            }

            @media (min-width: $desktopBreakPoint) {
                display: flex;
                align-items: center;
                justify-content: flex-end;
                position: initial;
                height: initial;
                overflow-y: unset;

                &.show {
                    display: flex;
                    height: unset;
                    padding-top: 0;
                    overflow-y: unset;
                }

                li {
                    padding: 0;
                    width: fit-content;
                }
            }
        }
    }

    .hero-section {
        max-width: 1106px;
        margin: auto;
        display: flex;
        align-items: center;
        min-height: 603px;
        height: 100%;

        &--cta {
            max-width: 533px;

            h1 {
                font-weight: bold;
                font-size: 48px;
                line-height: 64px;
                color: #ffffff;
                margin-bottom: 16px;
            }

            p {
                font-size: 18px;
                line-height: 30px;
                color: #ffffff;
            }

            button {
                background: #477db3;
                border-radius: 8px;
                padding: 9px 29px;
                color: #ffffff;
                font-weight: 500;
                font-size: 16px;
                line-height: 28px;
                margin-top: 40px;
            }
        }
    }
}

.section {
    .container {
        max-width: 1150px;
        margin: auto;
    }

    .section--title {
        font-weight: bold;
        font-size: 48px;
        line-height: 60px;
        letter-spacing: -1.584px;

        @media (max-width: 1089px) {
            font-size: 40px;
        }
    }

    .section--description {
        font-size: 18px;
        line-height: 30px;
    }
}

.features {
    padding: 160px 20px 600px;

    @media (max-width: 1089px) {
        padding: 60px 20px 100px;
    }

    .container {
        position: relative;
    }

    .section--title {
        margin-bottom: 26px;
    }

    .section--description {
        max-width: 508px;
        margin-bottom: 77px;
    }

    ul {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(238px, 1fr));
        gap: 80px 32px;

        img {
            width: 70px;
            margin-bottom: 36px;
        }

        li {
            position: relative;

            @media (max-width: 1089px) {
                .features--dashboard-screenshot {
                    display: none;
                }
            }
        }

        h4 {
            font-weight: bold;
            font-size: 18px;
            line-height: 22px;
            letter-spacing: -0.5px;
            margin-bottom: 16px;
        }

        p {
            font-size: 15px;
            line-height: 22px;

            @media (max-width: 1089px) {
                font-size: 16px;
            }
        }
    }

    .features--dashboard-screenshot {
        position: absolute;
        left: 0;
        top: 0;
        width: 800px;
        height: fit-content;

        img {
            width: 100%;
        }

        &.stand-alone {
            position: unset;
            width: 100%;
            text-align: center;
            display: none;

            @media (max-width: 1089px) {
                display: block;
            }

            img {
                width: 100%;
            }
        }
    }
}

.core-features {
    background: #2b4b6a;
    padding: 80px 20px;
    color: #ffffff;
    margin-bottom: 200px;

    .container {
        display: flex;
        align-items: center;
        justify-content: space-between;

        @media (max-width: 1089px) {
            display: unset;
        }
    }

    ul {
        margin-top: 32px;

        li {
            display: flex;
            align-items: center;
            margin-bottom: 29px;
            position: relative;
            font-weight: 500;
            font-size: 18px;
            line-height: 27px;

            &:not(:last-of-type)::after {
                content: '';
                position: absolute;
                height: 20px;
                width: 1px;
                background: #4b6680;
                bottom: -24px;
                left: 11px;
            }
        }

        img {
            width: 24px;
            margin-right: 24px;
        }
    }

    &--img-container {
        background: linear-gradient(0deg, #ffffff, #ffffff);
        box-shadow: 40px 44px 81px rgba(111, 118, 138, 0.160784);
        border-radius: 16px;
        height: 950px;
        max-width: 649px;
        width: 100%;
        margin-top: -160px;
        margin-bottom: -160px;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 40px;

        @media (max-width: 1089px) {
            margin: 100px auto 0;
            height: fit-content;
            padding: 40px 20px;
        }

        img {
            max-width: 467px;
            width: 100%;
        }
    }
}

footer {
    background: #1e252b;
    padding: 48px 20px 32px;
    color: #ffffff;

    .container {
        max-width: 1376px;
        width: 100%;
        margin: auto;
    }

    nav {
        display: flex;
        align-items: center;
        justify-content: space-between;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        padding-bottom: 29px;
    }

    p {
        font-weight: 500;
        font-size: 14px;
        line-height: 17px;
        margin-top: 24px;
        text-align: right;
    }
}
</style>
