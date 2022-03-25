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
		<convoy-app token="CO.UvCLgr14E0zaNryL.i1zUDL5tCaDUiK1I6QA7AiKDdbzGfrEmCFkppdDJzAJZgVB57O7GmgbX3H3yhqZe" apiURL="http://localhost:5005" />
	</div>
</template>
