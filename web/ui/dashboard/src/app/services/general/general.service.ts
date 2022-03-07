import { Injectable } from '@angular/core';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class GeneralService {
	constructor() {}

	showNotification(details: { message: string }) {
		if (!details.message) return;

		const notificationElement = document.querySelector('.app-notification');
		if (notificationElement) {
			notificationElement.classList.add('show');
			notificationElement.innerHTML = details.message;
		}

		setTimeout(() => {
			notificationElement?.classList.remove('show');
		}, 3000);
	}

	apiURL(): string {
		return `${environment.production ? location.origin : 'http://localhost:5005'}`;
	}
}
