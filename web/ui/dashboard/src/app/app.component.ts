import { Component } from '@angular/core';
import posthog from 'posthog-js';
import { environment } from 'src/environments/environment';
import { isConvoyCloud } from './utils/cloud.util';

@Component({
    selector: 'app-root',
    templateUrl: './app.component.html',
    styleUrls: ['./app.component.scss'],
    standalone: false
})
export class AppComponent {
	constructor() {
		if (isConvoyCloud()) posthog.init(environment.posthog, { api_host: 'https://app.posthog.com' });
	}
}
