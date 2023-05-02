import { Injectable } from '@angular/core';
import { PROJECT } from 'src/app/models/project.model';

@Injectable({
	providedIn: 'root'
})
export class ProjectService {
	activeProjectDetails?: PROJECT;

	constructor() {}
}
