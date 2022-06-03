import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { HttpService } from 'src/app/services/http/http.service';
import { ProjectService } from '../project.service';

@Injectable({
	providedIn: 'root'
})
export class AppsService {
	constructor(private http: HttpService) {}
}
