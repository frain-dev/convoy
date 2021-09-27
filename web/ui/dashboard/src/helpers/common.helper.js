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

// Work in progress here

// const getTimeDifference = date => {
// 	const startDate = new Date();
// 	const endDate = new Date(date);
// 	var diff = endDate.getTime() - startDate.getTime();
// 	var hours = Math.floor(diff / 1000 / 60 / 60);
// 	diff -= hours * 1000 * 60 * 60;
// 	console.log('🚀 ~ file: common.helper.js ~ line 27 ~ hours', hours);
// 	var minutes = Math.floor(diff / 1000 / 60);
// 	const seconds = Math.floor((diff / 1000) % 60);
// 	console.log('🚀 ~ file: common.helper.js ~ line 30 ~ seconds', seconds);

// 	// If using time pickers with 24 hours format, add the below line get exact hours
// 	if (hours < 0) hours = hours + 24;

// 	console.log((hours <= 9 ? '0' : '') + hours + ':' + (minutes <= 9 ? '0' : '') + minutes);
// 	return (hours <= 9 ? '0' : '') + hours + 'h:' + (minutes <= 9 ? '0' : '') + minutes + 'm' + seconds + 's';
// 	// return `${dateDifference}`;
// };

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
