import { Injectable } from '@angular/core';
import { CanActivate } from '@angular/router';

@Injectable({
	providedIn: 'root'
})
export class IframeGuard implements CanActivate {
	canActivate(): boolean {
		const isIframe = window.self !== window.top;
		if (isIframe) return false;
		return true;
	}
}
