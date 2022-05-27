import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
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
	projectId!: string;
	eventTags!: string[];
	appsDetailsItem: any = {
		created_at: '2022-04-22T12:16:51.86Z',
		endpoints: [
			{
				created_at: '2022-03-03T17:42:32.757Z',
				description: 'second new app endpoint',
				events: ['new new endpoint'],
				http_timeout: '',
				rate_limit: 0,
				rate_limit_duration: '',
				secret: '71GG_jZeYYC--c1Y5a1VMMVULUnoemUhYQ==',
				status: 'active',
				target_url: 'https://webhook.site/ac06134f-b969-4388-b663-1e55951a99a4',
				uid: '2f9c123a-1ae7-4cd5-bbc0-9c08d32cefc1',
				updated_at: '2022-03-03T17:42:32.757Z'
			}
		],
		events: 0,
		group_id: 'db78d6fe-b05e-476d-b908-cb6fff26a3ed',
		is_disabled: false,
		name: 'App D',
		support_email: '',
		uid: '6ab551cb-b6ac-4808-abd2-09e0570028b7',
		updated_at: '2022-04-22T12:16:51.86Z'
	};
	apps!: { pagination: PAGINATION; content: APP[] };
	constructor(private formBuilder: FormBuilder, private appDetailsService: AppDetailsService) {}

	ngOnInit(): void {}

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

	async getAppPortalToken(requestDetail: { redirect: boolean }) {
		this.loadingAppPotalToken = true;

		try {
			const appTokenResponse = await this.appDetailsService.getAppPortalToken({ appId: this.appsDetailsItem.uid, projectId: this.projectId });
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
