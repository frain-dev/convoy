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
	const blog = document.querySelector('body');

	if (to.hash) {
		let el = await findEl(to.hash);
		if ('scrollBehavior' in document.documentElement.style) {
			main.scrollTo({ top: el.offsetTop - 170, behavior: 'smooth' });
			blog.scrollTo({ top: el.offsetTop - 170, behavior: 'smooth' });
			return;
		} else {
			main.scrollTo(0, el.offsetTop);
			blog.scrollTo(0, el.offsetTop);
			return;
		}
	}

	main.scrollTo({ top: 0, behavior: 'smooth' });
	blog.scrollTo({ top: 0, behavior: 'smooth' });
	return { x: 0, y: 0 };
}
