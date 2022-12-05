import { Component } from '@angular/core';
import posthog from 'posthog-js';
import { environment } from 'src/environments/environment';

@Component({
	selector: 'app-root',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent {
	constructor() {
		posthog.init(environment.posthog, { api_host: 'https://app.posthog.com', ui_host: 'https://dashboard.getconvoy.io' });
	}
}
