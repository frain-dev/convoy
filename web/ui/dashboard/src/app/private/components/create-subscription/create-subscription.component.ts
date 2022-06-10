import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { APP, ENDPOINT } from 'src/app/models/app.model';
import { GROUP, SOURCE } from 'src/app/models/group.model';
import { ProjectService } from '../../pages/project/project.service';
import { PrivateService } from '../../private.service';
import { CreateSubscriptionService } from './create-subscription.service';

@Component({
	selector: 'app-create-subscription',
	templateUrl: './create-subscription.component.html',
	styleUrls: ['./create-subscription.component.scss']
})
export class CreateSubscriptionComponent implements OnInit {
	subscriptonForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		type: ['', Validators.required],
		app_id: ['', Validators.required],
		source_id: ['', Validators.required],
		endpoint_id: ['', Validators.required],
		group_id: ['', Validators.required]
	});
	apps!: APP[];
	sources!: SOURCE[];
	endPoints?: ENDPOINT[];
	showCreateAppModal = false;
	showCreateSourceModal = false;
	isCreatingSubscription = false;
	@Output() onAction = new EventEmitter();
	projectType: 'incoming' | 'outgoing' = 'incoming';
	isLoadingForm = true;

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private createSubscriptionService: CreateSubscriptionService, private router: Router) {}

	async ngOnInit() {
		this.isLoadingForm = true;
		await Promise.all([this.getApps(), this.getSources(), this.getGetProjectDetails()]);
		this.isLoadingForm = false;
	}

	async getApps() {
		try {
			const appsResponse = await this.privateService.getApps();
			this.apps = appsResponse.data.content;
			return;
		} catch (error) {
			return error;
		}
	}

	async getSources() {
		try {
			const sourcesResponse = await this.privateService.getSources();
			this.sources = sourcesResponse.data.content;
			return;
		} catch (error) {
			return;
		}
	}

	async getGetProjectDetails() {
		try {
			const response = await this.privateService.getProjectDetails();
			this.subscriptonForm.patchValue({
				group_id: response.data.uid,
				type: 'incoming'
			});
			this.projectType = response.data.type;
			return;
		} catch (error) {
			return;
		}
	}

	onUpdateAppSelection() {
		const app = this.apps.find(app => app.uid === this.subscriptonForm.value.app_id);
		this.endPoints = app?.endpoints;
	}

	async onCreateSource(newSource: SOURCE) {
		await this.getSources();
		this.subscriptonForm.patchValue({ source_id: newSource.uid });
	}

	async createSubscription() {
		if (this.projectType === 'incoming' && this.subscriptonForm.invalid) return this.subscriptonForm.markAllAsTouched();
		if (
			this.subscriptonForm.get('name')?.invalid &&
			this.subscriptonForm.get('type')?.invalid &&
			this.subscriptonForm.get('app_id')?.invalid &&
			this.subscriptonForm.get('endpoint_id')?.invalid &&
			this.subscriptonForm.get('group_id')?.invalid
		) {
			return this.subscriptonForm.markAllAsTouched();
		}

		const subscription = this.subscriptonForm.value;
		if (this.projectType === 'outgoing') delete subscription.source_id;
		this.isCreatingSubscription = true;

		try {
			const response = await this.createSubscriptionService.createSubscription(this.subscriptonForm.value);
			this.isCreatingSubscription = false;
			this.onAction.emit(response.data);
		} catch (error) {
			this.isCreatingSubscription = false;
		}
	}

	async onCreateNewApp(newApp: APP) {
		await this.getApps();
		this.subscriptonForm.patchValue({ app_id: newApp.uid });
	}
}
