import { Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class GeneralService {
	constructor() {}
	apiURL(): string {
		return `${environment.production ? location.origin : 'http://localhost:5005'}`;
	}
}
