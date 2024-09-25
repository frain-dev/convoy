import { Injectable } from '@angular/core';
import { CanActivate } from '@angular/router';

@Injectable({
	providedIn: 'root'
})
export class IframeGuard implements CanActivate {
	canActivate(): boolean {
		return window.self === window.top;
	}
}
