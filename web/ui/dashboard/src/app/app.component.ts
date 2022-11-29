import { Component } from '@angular/core';
import posthog from 'posthog-js';

@Component({
	selector: 'app-root',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent {
	constructor() {
		posthog.init('phc_lPJnjN5hrM8Dh7kgujIccs2xnGL2lmRv6UdOmOTCqEc', { api_host: 'https://app.posthog.com' });
	}
}
