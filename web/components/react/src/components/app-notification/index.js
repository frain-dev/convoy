import React from 'react';
import './style.scss';

const AppNotification = () => {
	return <div className="app-notification"></div>;
};

const showNotification = ({ message }) => {
	if (!message) return;

	const notificationElement = document.querySelector('.app-notification');
	notificationElement.classList.add('show');
	notificationElement.innerHTML = message;

	setTimeout(() => {
		document.querySelector('.app-notification').classList.remove('show');
	}, 3000);
};
export { AppNotification, showNotification };
