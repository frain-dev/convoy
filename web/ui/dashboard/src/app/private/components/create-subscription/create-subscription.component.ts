import { Component, OnInit } from '@angular/core';
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
	subscriptionForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		type: ['', Validators.required],
		app_id: ['', Validators.required],
		source_id: ['', Validators.required],
		endpoint_id: ['', Validators.required],
		group_id: ['', Validators.required],
		alert_config: this.formBuilder.group({
			theshold: [''],
			time: ['']
		}),
		retry_config: this.formBuilder.group({
			type: ['', Validators.required],
			retry_count: ['', Validators.required],
			interval_seconds: ['', Validators.required]
		}),
		filter_config: this.formBuilder.group({
			event_types: ['']
		})
	});
	apps!: APP[];
	sources!: SOURCE[];
	endPoints?: ENDPOINT[];
	eventTags: string[] = [];
	showCreateAppModal = false;
	showCreateSourceModal = false;
	retryLogicTypes = [
		{ id: 'linear', type: 'Linear time retry' },
		{ id: 'exponential', type: 'Exponential time backoff' }
	];

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private createSubscriptionService: CreateSubscriptionService, private router: Router) {}

	ngOnInit(): void {
		Promise.all([this.getApps(), this.getSources(), this.getGetProjectDetails()]);
	}

	async getApps() {
		try {
			const appsResponse = await this.privateService.getApps();
			this.apps = appsResponse.data.content;
		} catch (error) {
			console.log(error);
		}
	}

	async getSources() {
		try {
			const sourcesResponse = await this.privateService.getSources();
			this.sources = sourcesResponse.data.content;
		} catch (error) {
			console.log(error);
		}
	}

	async getGetProjectDetails() {
		try {
			const response = await this.privateService.getProjectDetails();
			this.subscriptionForm.patchValue({
				group_id: response.data.uid,
				type: 'incoming'
			});
		} catch (error) {
			console.log(error);
		}
	}

	onUpdateAppSelection() {
		const app = this.apps.find(app => app.uid === this.subscriptionForm.value.app_id);
		this.endPoints = app?.endpoints;
	}

	async onCreateSource(newSource: SOURCE) {
		await this.getSources();
		this.subscriptionForm.patchValue({ source_id: newSource.uid });
	}

	async createSubscription() {
		console.log(this.subscriptionForm.value);
		this.subscriptionForm.patchValue({
			filter_config: { event_types: this.eventTags }
		});

		if (this.subscriptionForm.invalid) return this.subscriptionForm.markAllAsTouched();

		try {
			const response = await this.createSubscriptionService.createSubscription(this.subscriptionForm.value);
			this.router.navigateByUrl('/projects/' + this.privateService.activeProjectId + '/subscriptions');
		} catch (error) {
			console.log(error);
		}
	}

	async onCreateNewApp(newApp: APP) {
		await this.getApps();
		this.subscriptionForm.patchValue({ app_id: newApp.uid });
	}

	removeEventTag(tag: string) {
		this.eventTags = this.eventTags.filter(e => e !== tag);
	}

	addTag() {
		const addTagInput = document.getElementById('tagInput');
		const addTagInputValue = document.getElementById('tagInput') as HTMLInputElement;
		addTagInput?.addEventListener('keydown', e => {
			if (e.which === 188) {
				if (this.eventTags.includes(addTagInputValue?.value)) {
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				} else {
					this.eventTags.push(addTagInputValue?.value);
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				}
				e.preventDefault();
			}
		});
	}

	focusInput() {
		document.getElementById('tagInput')?.focus();
	}
}
