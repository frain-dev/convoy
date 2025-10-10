import { Injectable } from '@angular/core';
import { HttpRequest, HttpHandler, HttpEvent, HttpInterceptor, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { Router } from '@angular/router';
import { GeneralService } from '../services/general/general.service';

@Injectable()
export class HttpIntercepter implements HttpInterceptor {
	constructor(private router: Router, private generalService: GeneralService) {}

	intercept(request: HttpRequest<unknown>, next: HttpHandler): Observable<HttpEvent<unknown>> {
		const modifiedRequest = this.addRootPathToApiCalls(request);

		return next.handle(modifiedRequest).pipe(
			map((httpEvent: HttpEvent<any>) => {
				return httpEvent;
			}),
			catchError((error: HttpErrorResponse) => {
				if (error.status === 401) {
					this.router.navigate(['/login'], { replaceUrl: true });
					localStorage.removeItem('CONVOY_AUTH');
				}
				let errorMessage: string;
				error.error?.message ? (errorMessage = error.error?.message) : (errorMessage = 'An error occured, please try again');
				this.generalService.showNotification({
					message: errorMessage,
					style: 'error'
				});
				return throwError(error);
			})
		);
	}

	private addRootPathToApiCalls(request: HttpRequest<unknown>): HttpRequest<unknown> {
		const baseElement = document.querySelector('base');
		const baseHref = baseElement?.getAttribute('href') || '/';

		const rootPath = baseHref.replace(/\/$/, '');

		const insertRootPath = (url: string) => {
			if (rootPath === '/') return url;
			try {
				const urlObj = new URL(url, window.location.origin);
				urlObj.pathname = rootPath + urlObj.pathname;
				return urlObj.pathname + urlObj.search + urlObj.hash;
			} catch (e) {
				return rootPath + url;
			}
		};

		const modifiedUrl = insertRootPath(request.url);

		return request.clone({
			url: modifiedUrl
		});
	}
}
