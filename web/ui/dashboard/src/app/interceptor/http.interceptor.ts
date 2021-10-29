import { Injectable } from '@angular/core';
import { HttpRequest, HttpHandler, HttpEvent, HttpInterceptor, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { Router } from '@angular/router';

@Injectable()
export class HttpIntercepter implements HttpInterceptor {
	constructor(private router: Router) {}

	intercept(request: HttpRequest<unknown>, next: HttpHandler): Observable<HttpEvent<unknown>> {
		return next.handle(request).pipe(
			map((httpEvent: HttpEvent<any>) => {
				return httpEvent;
			}),
			catchError((error: HttpErrorResponse) => {
				if (error.status === 401) this.router.navigate(['/login'], { replaceUrl: true });
				return throwError(error);
			})
		);
	}
}
