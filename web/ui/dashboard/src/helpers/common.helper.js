const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];

const getDate = date => {
	const _date = new Date(date);
	const day = _date.getDate();
	const month = _date.getMonth();
	const year = _date.getFullYear();
	return `${day} ${months[month]}, ${year}`;
};

const getTime = date => {
	const _date = new Date(date);
	const hours = _date.getHours();
	const minutes = _date.getMinutes();
	const seconds = _date.getSeconds();

	const hour = hours > 12 ? hours - 12 : hours;
	return `${hour} : ${minutes} : ${seconds} ${hours > 12 ? 'AM' : 'PM'}`;
};

const getDateDifference = date => {
	const dayOfYear = date => {
		const start = new Date(date.getFullYear(), 0, 0);
		const diff = date - start + (start.getTimezoneOffset() - date.getTimezoneOffset()) * 60 * 1000;
		const oneDay = 1000 * 60 * 60 * 24;
		const day = Math.floor(diff / oneDay);
		return day;
	};
	const dateDifference = dayOfYear(new Date(date)) - dayOfYear(new Date());
	return `${dateDifference}`;
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

export { getDate, getTime, copyText, logout, getDateDifference };
