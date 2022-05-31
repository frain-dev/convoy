import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { AppDetailsService } from './app-details.service';

@Component({
	selector: 'app-app-details',
	templateUrl: './app-details.component.html',
	styleUrls: ['./app-details.component.scss']
})
export class AppDetailsComponent implements OnInit {
	showAddEndpointModal: boolean = false;
	showAddEventModal: boolean = false;
	showEndpointSecret: boolean = false;
	showPublicCopyText: boolean = false;
	showSecretCopyText: boolean = false;
	isSendingNewEvent: boolean = false;
	isCreatingNewEndpoint: boolean = false;
	loadingAppPotalToken: boolean = false;
	isLoadingAppDetails: boolean = false;

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
	eventTags!: string[];
	appsDetailsItem!: APP;
	apps!: { pagination: PAGINATION; content: APP[] };
	constructor(private formBuilder: FormBuilder, private appDetailsService: AppDetailsService, private route: ActivatedRoute) {}

	ngOnInit() {
		this.getAppId();
	}

	getAppId() {
		this.route.params.subscribe(res => {
			const appId = res.id;
			this.getAppDetails(appId)
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

	sendNewEvent() {}

	addNewEndpoint() {}

	viewEndpointSecretKey(secretKey: string) {
		this.showEndpointSecret = !this.showEndpointSecret;
		this.endpointSecretKey = secretKey;
	}

	async getAppDetails(appId: string) {
		this.isLoadingAppDetails = true;

		try {
			const response = await this.appDetailsService.getApp(appId);
			this.appsDetailsItem = response.data;
			this.getAppPortalToken({ redirect: false })
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

	loadEventsFromAppsTable(appId: string) {}
}
