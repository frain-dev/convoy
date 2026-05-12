import { Injectable } from '@angular/core';


@Injectable({
	providedIn: 'root'
})
export class IframeGuard  {
	canActivate(): boolean {
		return window.self === window.top;
	}
}
