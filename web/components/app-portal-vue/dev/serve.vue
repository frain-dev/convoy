<script>
import { defineComponent } from 'vue';
// Uncomment import and local "components" registration if library is not registered globally.
// import { ConvoyAppVueSample } from '@/entry.esm';

export default defineComponent({
	name: 'ServeDev'
	// components: {
	//  ConvoyAppVueSample,
	// }
});
</script>

<template>
	<div id="app">
		<convoy-app token="CO.be5zR6Wgt9PmLt0p.z6loCYaL86jz4xVuRlDzYN2BForQQUV7CQktnvpsmSz2ZTEI1X8HUzOEpMkudpIW" apiURL="http://localhost:5005" />
	</div>
</template>
