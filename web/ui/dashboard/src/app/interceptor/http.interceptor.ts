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
		return next.handle(request).pipe(
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
}
