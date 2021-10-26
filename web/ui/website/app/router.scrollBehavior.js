export default async function (to, from, savedPosition) {
	if (savedPosition) {
		return savedPosition;
	}

	const findEl = async (hash, x = 0) => {
		return (
			document.querySelector(hash) ||
			new Promise(resolve => {
				if (x > 50) {
					return resolve(document.querySelector('#app'));
				}
				setTimeout(() => {
					resolve(findEl(hash, ++x || 1));
				}, 100);
			})
		);
	};

	const main = document.querySelector('.main');

	if (to.hash) {
		let el = await findEl(to.hash);
		if ('scrollBehavior' in document.documentElement.style) {
			return main.scrollTo({ top: el.offsetTop, behavior: 'smooth' });
		} else {
			return main.scrollTo(0, el.offsetTop);
		}
	}

	main.scrollTo({ top: 0, behavior: 'smooth' });
	return { x: 0, y: 0 };
}
