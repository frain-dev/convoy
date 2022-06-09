import { Location } from '@angular/common';
import { Component, HostListener, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppDetailsService } from './app-details.service';

@Component({
	selector: 'app-app-details',
	templateUrl: './app-details.component.html',
	styleUrls: ['./app-details.component.scss']
})
export class AppDetailsComponent implements OnInit {
	showAddEndpointModal: boolean = false;
	showAddEventModal = false;
	showEndpointSecret = false;
	showPublicCopyText = false;
	showSecretCopyText = false;
	isSendingNewEvent = false;
	isCreatingNewEndpoint = false;
	loadingAppPotalToken = false;
	isLoadingAppDetails = false;
	shouldRenderSmallSize = false;
	screenWidth = window.innerWidth;
	addNewEndpointForm: FormGroup = this.formBuilder.group({
		url: ['', Validators.required],
		events: [''],
		description: ['', Validators.required]
	});
	sendEventForm: FormGroup = this.formBuilder.group({
		app_id: ['', Validators.required],
		data: ['', Validators.required],
		event_type: ['', Validators.required]
	});
	appPortalLink!: string;
	endpointSecretKey!: string;
	eventTags: string[] = [];
	appsDetailsItem!: APP;
	apps!: { pagination: PAGINATION; content: APP[] };
	constructor(
		private formBuilder: FormBuilder,
		private appDetailsService: AppDetailsService,
		private generalService: GeneralService,
		private route: ActivatedRoute,
		private location: Location,
		private router: Router
	) {}

	async ngOnInit() {
		await Promise.all([this.checkScreenSize(), this.getAppId(), this.getApps()]);
	}

	goBack() {
		this.location.back();
	}

	getAppId() {
		this.route.params.subscribe(res => {
			const appId = res.id;
			this.getAppDetails(appId);
		});
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

	// copy code snippet
	copyKey(key: string, type: 'public' | 'secret') {
		const text = key;
		const el = document.createElement('textarea');
		el.value = text;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		type === 'public' ? (this.showPublicCopyText = true) : (this.showSecretCopyText = true);
		setTimeout(() => {
			type === 'public' ? (this.showPublicCopyText = false) : (this.showSecretCopyText = false);
		}, 3000);
		document.body.removeChild(el);
	}

	async sendNewEvent() {
		if (this.sendEventForm.invalid) {
			(<any>Object).values(this.sendEventForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.isSendingNewEvent = true;
		try {
			const response = await this.appDetailsService.sendEvent({ body: this.sendEventForm.value });

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.sendEventForm.reset();
			this.showAddEventModal = false;
			this.isSendingNewEvent = false;
			const projectId = this.appDetailsService.projectId;
			this.router.navigate(['/projects/' + projectId + '/events'], { queryParams: { eventsApp: this.appsDetailsItem?.uid } });
		} catch {
			this.isSendingNewEvent = false;
		}
	}

	async addNewEndpoint() {
		if (this.addNewEndpointForm.invalid) {
			(<any>Object).values(this.addNewEndpointForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.isCreatingNewEndpoint = true;

		this.addNewEndpointForm.patchValue({
			events: this.eventTags
		});

		try {
			const response = await this.appDetailsService.addNewEndpoint({ appId: this.appsDetailsItem?.uid, body: this.addNewEndpointForm.value });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getAppDetails(this.appsDetailsItem?.uid);
			this.addNewEndpointForm.reset();
			this.eventTags = [];
			this.showAddEndpointModal = false;
			this.isCreatingNewEndpoint = false;
			return;
		} catch {
			this.isCreatingNewEndpoint = false;
			return;
		}
	}

	async getApps() {
		try {
			const appsResponse = await this.appDetailsService.getApps();

			this.apps = appsResponse.data;
		} catch (error: any) {
			return error;
		}
	}

	viewEndpointSecretKey(secretKey: string) {
		this.showEndpointSecret = !this.showEndpointSecret;
		this.endpointSecretKey = secretKey;
	}

	async getAppDetails(appId: string) {
		this.isLoadingAppDetails = true;

		try {
			const response = await this.appDetailsService.getApp(appId);
			this.appsDetailsItem = response.data;
			this.getAppPortalToken({ redirect: false });
			this.isLoadingAppDetails = false;
		} catch {
			this.isLoadingAppDetails = false;
		}
	}

	async getAppPortalToken(requestDetail: { redirect: boolean }) {
		this.loadingAppPotalToken = true;

		try {
			const appTokenResponse = await this.appDetailsService.getAppPortalToken({ appId: this.appsDetailsItem.uid });
			this.appPortalLink = `<iframe style="width: 100%; height: 100vh; border: none;" src="${appTokenResponse.data.url}"></iframe>`;
			if (requestDetail.redirect) window.open(`${appTokenResponse.data.url}`, '_blank');
			this.loadingAppPotalToken = false;
		} catch (error) {
			this.loadingAppPotalToken = false;
			return error;
		}
	}

	setEventAppId() {
		this.showAddEventModal = !this.showAddEventModal;
		this.sendEventForm.patchValue({
			app_id: this.appsDetailsItem?.uid
		});
	}

	loadEventsFromAppsTable(appId: string) {
		const projectId = this.appDetailsService.projectId;
		this.router.navigate(['/projects/' + projectId + '/events'], { queryParams: { eventsApp: appId } });
	}

	checkScreenSize() {
		this.screenWidth > 1010 ? (this.shouldRenderSmallSize = false) : (this.shouldRenderSmallSize = true);
	}

	focusInput() {
		document.getElementById('tagInput')?.focus();
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}
}
