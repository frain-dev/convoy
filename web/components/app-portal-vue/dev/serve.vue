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
		<convoy-app token="CO.oerRGsRYWXl9YJDn.1cu93uXTKj79AgH3EtwQ7BCFZWzQeWpRhHPsBL1mLBMejgWTNwx76nz3dqNFAN9h" apiURL="http://localhost:5005" />
	</div>
</template>
