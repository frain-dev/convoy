<template>
	<div>
		<Header></Header>
		<header></header>
		<div class="page">
			<h2>Download Convoy</h2>
			<p class="subtitle">Download Convoy with your favorite package manager.</p>
			<div class="tabs tabs__light">
				<li v-for="tab of tabs" :key="tab.id">
					<button :class="activeTab === tab.id ? 'active' : ''" @click="switchTabs(tab.id)">
						<span>{{ tab.label }}</span>
					</button>
				</li>
			</div>

			<section class="download">
				<div v-if="activeTab == 'mac'" class="download--title">Homebrew</div>
				<div v-if="activeTab == 'linux'" class="download--title">Package manager</div>
				<div v-if="activeTab == 'window'" class="download--title">Windows binary download</div>
				<div v-if="activeTab == 'mac'" class="code">
					<div>
						<span>$</span>
						<code>brew tap frain-dev/tools</code>
					</div>
					<div>
						<span>$</span>
						<code>brew install convoy</code>
					</div>
				</div>
				<div v-if="activeTab == 'linux'">
					<ul class="tabs tabs__line">
						<li v-for="tab of linuxTabs" :key="tab.id">
							<button :class="linuxActiveTab === tab.id ? 'active' : ''" @click="switchLinuxTabs(tab.id)">
								<span>{{ tab.label }}</span>
							</button>
						</li>
					</ul>
					<div v-if="linuxActiveTab == 'ubuntu'" class="code">
						<div>
							<span>$</span>
							<code>echo "deb [trusted=yes] https://apt.packages.getconvoy.io/ /" | sudo tee -a /etc/apt/sources.list.d/convoy.list</code>
						</div>
						<div>
							<span>$</span>
							<code>sudo apt update</code>
						</div>
						<div>
							<span>$</span>
							<code>sudo apt install convoy</code>
						</div>
					</div>
					<div v-if="linuxActiveTab == 'cent'" class="code">
						<div class="code--flex">
							<span>$</span>
							<div class="code--flex-code">
								<code>echo '[convoy]</code>
								<code>name=Convoy</code>
								<code>baseurl=https://yum.packages.getconvoy.io/</code>
								<code>enabled=1</code>
								<code>gpgcheck=0' | sudo tee -a /etc/yum.repos.d/convoy.repo</code>
							</div>
						</div>
						<div>
							<span>$</span>
							<code>sudo yum install convoy</code>
						</div>
					</div>
				</div>
				<div class="download--view-more" v-if="activeTab != 'window'">
					<nuxt-link to="/docs">
						View our Docs to learn more
						<img src="~/assets/images/angle-right-primary.svg" alt="right" />
					</nuxt-link>
				</div>
				<div class="download--view-more flex-between" v-if="activeTab == 'window'">
					<div class="download--view-more--links">
						<a target="_blank" rel="noopener noreferrer" href="https://brew.packages.getconvoy.io/releases/v0.5.2/convoy_0.5.2_windows_amd64.tar.gz" class="underlined" download="">Amd64</a>
						<a target="_blank" rel="noopener noreferrer" href="https://brew.packages.getconvoy.io/releases/v0.5.2/convoy_0.5.2_windows_arm64.tar.gz" class="underlined" download>Arm64</a>
					</div>
					<nuxt-link to="/docs">
						View our Docs to learn more
						<img src="~/assets/images/angle-right-primary.svg" alt="right" />
					</nuxt-link>
				</div>
			</section>
		</div>
		<Footer></Footer>
	</div>
</template>
<script>
export default {
	data() {
		return {
			tabs: [
				{ label: 'MacOS', id: 'mac' },
				{ label: 'Linux', id: 'linux' },
				{ label: 'Windows', id: 'window' }
			],
			linuxTabs: [
				{ label: 'Ubuntu/Debian', id: 'ubuntu' },
				{ label: 'CentOS/RHEL ', id: 'cent' }
			],
			activeTab: 'mac',
			linuxActiveTab: 'ubuntu'
		};
	},
	methods: {
		switchTabs(activeTab) {
			switch (activeTab) {
				case 'mac':
					this.activeTab = 'mac';
					break;
				case 'linux':
					this.activeTab = 'linux';
					break;
				case 'window':
					this.activeTab = 'window';
					break;
				default:
					break;
			}
		},
		switchLinuxTabs(activeTab) {
			switch (activeTab) {
				case 'ubuntu':
					this.linuxActiveTab = 'ubuntu';
					break;
				case 'cent':
					this.linuxActiveTab = 'cent';
					break;
				case 'home':
					this.linuxActiveTab = 'home';
					break;
				default:
					break;
			}
		}
	}
};
</script>
<style lang="scss" scoped>
h2 {
	font-weight: bold;
	font-size: 27px;
	line-height: 32px;
	letter-spacing: 0.01em;
	color: #000624;
	color: #ffffff;
	margin-bottom: 8px;
	text-align: center;
	width: 100%;
}
p.subtitle {
	font-size: 16px;
	line-height: 24px;
	color: #737a91;
	max-width: 671px;
	margin-bottom: 36px;
	color: #ffffff;
	text-align: center;
	max-width: 590px;
	margin: auto;
	margin-bottom: 40px;
}

header {
	background: url('~/assets/images/BG.png'), no-repeat;
	background-size: cover;
	height: 350px;
	width: 100%;
}
.page {
	max-width: 1150px;
	margin: auto;
	margin-top: -130px;
	margin-bottom: 88px;
}
.download {
	background: #f3f3f8;
	border-radius: 4px;
	padding: 24px;
	max-width: 724px;
	width: 100%;
	margin: auto;

	&--title {
		font-weight: 600;
		font-size: 14px;
		line-height: 24px;
		letter-spacing: 0.03em;
		text-transform: uppercase;
		color: #737a91;
		margin-bottom: 16px;
	}
	.code {
		background: #000624;
		border-radius: 4px;
		padding: 24px;
		width: 100%;
		color: #ffffff;
		flex-flow: column nowrap;
		font-size: 13px;
		line-height: 20px;
		span {
			color: #477db3;
		}
		&--flex {
			display: flex;
			span {
				margin-right: 4px;
			}
			&-code {
				display: flex;
				flex-flow: column;
			}
		}
	}
	&--view-more {
		display: flex;
		justify-content: flex-end;
		margin-top: 27px;

		&--links {
			display: flex;
			align-items: center;
		}

		&.flex-between {
			justify-content: space-between;
		}
		a {
			display: flex;
			align-items: center;
			font-weight: 500;
			font-size: 14px;
			line-height: 22px;
			color: #477db3;
			white-space: nowrap;
			img {
				width: 16px;
				height: 16px;
				margin-left: 8px;
			}

			&:not(:last-of-type) {
				margin-right: 32px;
			}
			&.underlined {
				text-decoration: underline;
			}
		}
	}
}
</style>
