import { Injectable } from '@angular/core';
import { GROUP } from 'src/app/models/group.model';

@Injectable({
	providedIn: 'root'
})
export class ProjectService {
	activeProjectDetails?: GROUP;

	constructor() {}
}
