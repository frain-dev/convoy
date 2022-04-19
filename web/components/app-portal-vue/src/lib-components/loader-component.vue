<script>
export default {
	data() {
		return {
			count: 0
		};
	},
	props: ['isTransparent']
};
</script>

<template>
	<div :class="'loader ' + (isTransparent ? 'transparent' : '')">
		<img src="https://res.cloudinary.com/frain/image/upload/v1649343844/loader_q1svx1.gif" alt="loader" />
	</div>
</template>

<style>
.loader {
	position: absolute;
	left: 0;
	right: 0;
	top: 0;
	bottom: 0;
	display: flex;
	justify-content: center;
	align-items: center;
	background: #fff;
	border-radius: 8px;
	z-index: 1;
}

.loader img {
	width: 100%;
	height: 100%;
	max-width: 150px;
	max-height: 110px;
}

.loader.transparent {
	opacity: 0.5;
}
</style>
