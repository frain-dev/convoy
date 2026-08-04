import { Injectable } from '@angular/core';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class IframeGuard {
	canActivate(): boolean {
		// Allow embedded previews (e.g. IDE Simple Browser) during local development.
		if (!environment.production) return true;
		return window.self === window.top;
	}
}
