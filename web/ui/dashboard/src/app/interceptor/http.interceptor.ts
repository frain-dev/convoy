import {Injectable} from '@angular/core';
import { HttpErrorResponse, HttpEvent, HttpHandler, HttpInterceptor, HttpRequest } from '@angular/common/http';
import {Observable, throwError} from 'rxjs';
import {catchError, map} from 'rxjs/operators';
import {Router} from '@angular/router';
import {GeneralService} from '../services/general/general.service';
import { AuthSessionService } from '../services/auth-session/auth-session.service';

@Injectable()
export class HttpIntercepter implements HttpInterceptor {
	constructor(private router: Router, private generalService: GeneralService, private authSessionService: AuthSessionService) {}

	intercept(request: HttpRequest<unknown>, next: HttpHandler): Observable<HttpEvent<unknown>> {
		const modifiedRequest = this.addRootPathToApiCalls(request);

		return next.handle(modifiedRequest).pipe(
			map((httpEvent: HttpEvent<any>) => {
				return httpEvent;
			}),
			catchError((error: HttpErrorResponse) => {
				if (error.status === 401) {
					this.router.navigate(['/login'], { replaceUrl: true });
					this.authSessionService.clearLocalSession();
					return throwError(error);
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
