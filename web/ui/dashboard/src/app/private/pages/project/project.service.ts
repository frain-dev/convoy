import { Injectable } from '@angular/core';

@Injectable({
	providedIn: 'root'
})
export class ProjectService {
	activeProject!: string;

	constructor() {}
}
