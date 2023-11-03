import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { Router } from '@angular/router';
import { PROJECT } from 'src/app/models/project.model';
import { PrivateService } from '../../private.service';
import { Location } from '@angular/common';

export type STAGES = 'createProject' | 'setupSDK' | 'createSource' | 'createApplication' | 'createSubscription';
@Component({
	selector: 'app-create-project',
	templateUrl: './create-project.component.html',
	styleUrls: ['./create-project.component.scss']
})
export class CreateProjectComponent implements OnInit {
	@ViewChild('projectDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;

	constructor(private router: Router, public privateService: PrivateService, private location: Location) {}

	ngOnInit() {
		this.dialog.nativeElement.showModal();
	}

	ngOnDestroy() {
		this.dialog.nativeElement.close();
	}

	async createProject(newProjectData: { action: string; data: PROJECT }) {
		this.router.navigateByUrl('/projects/' + newProjectData.data.uid);
	}

	cancel() {
		this.location.back();
		this.dialog.nativeElement.close();
	}
}
