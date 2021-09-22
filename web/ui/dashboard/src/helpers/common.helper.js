const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];

const getDate = date => {
	const _date = new Date(date);
	const day = _date.getDate();
	const month = _date.getMonth();
	const year = _date.getFullYear();
	return `${day} ${months[month]}, ${year}`;
};

const copyText = copyText => {
	const el = document.createElement('textarea');
	el.value = copyText;
	document.body.appendChild(el);
	el.select();
	document.execCommand('copy');
	el.style.display = 'none';
};

const logout = () => {
	localStorage.removeItem('CONVOY_AUTH');
	window.location.replace('/login');
};

export { getDate, copyText, logout };
