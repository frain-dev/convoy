import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { SettingsService } from '../settings.service';

@Component({
	selector: 'personal-keys',
	templateUrl: './personal-keys.component.html',
	styleUrls: ['./personal-keys.component.scss']
})
export class PersonalKeysComponent implements OnInit {
	showCreateNewTokenForm = false;
	isFetchingKeys = false;
	isGeneratingNewKey = false;
	showAccessKey = false;
	showRevokeKeyModal = false;
	isRevokingKey = false;
	selectedKey!: any;
	userId!: string;
	accessKey!: string;
	loaderIndex: number[] = [0, 1, 2];
	personalAccessKeys!: { content: any; pagination: PAGINATION };
	expirationDates = [
		{ name: '7 days', uid: 7 },
		{ name: '14 days', uid: 14 },
		{ name: '30 days', uid: 30 },
		{ name: '90 days', uid: 90 }
	];
	generateKeyForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		expiration: ['']
	});
	constructor(private formBuilder: FormBuilder, private settingService: SettingsService, private generalService: GeneralService, private router: Router, private route: ActivatedRoute) {}

	ngOnInit() {
		this.getUserId();
	}

	getUserId() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		if (authDetails && authDetails !== 'undefined') {
			this.userId = JSON.parse(authDetails)?.uid;
			this.fetchPersonalKeys();
		} else {
			this.router.navigateByUrl('/login');
		}
	}

	async generateNewKey() {
		this.isGeneratingNewKey = true;
		this.generateKeyForm.value.expiration = parseInt(this.generateKeyForm.value.expiration);
		try {
			const response = await this.settingService.generatePersonalKey(this.userId, this.generateKeyForm.value);
			this.accessKey = response.data.key;
			this.showCreateNewTokenForm = false;
			this.showAccessKey = true;
			this.generateKeyForm.reset();
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isGeneratingNewKey = false;
		} catch {
			this.isGeneratingNewKey = false;
			return;
		}
	}

	async fetchPersonalKeys(requestDetails?: { page?: number }) {
		this.isFetchingKeys = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		try {
			const response = await this.settingService.fetchPersonalKeys({ userId: this.userId, pageNo: page });
			this.personalAccessKeys = response.data;
			console.log(response);
			this.isFetchingKeys = false;
		} catch {
			this.isFetchingKeys = false;
		}
	}

	async revokeKey() {
		this.isRevokingKey = true;
		try {
			const response = await this.settingService.revokeKey({ userId: this.userId, keyId: this.selectedKey?.uid });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRevokingKey = false;
			this.showRevokeKeyModal = false;
			this.fetchPersonalKeys();
		} catch {
			this.isRevokingKey = false;
		}
	}

	getKeyStatus(expiryDate: Date): string {
		const currentDate = new Date();
		if (currentDate > new Date(expiryDate)) return 'disabled';
		return 'active';
	}
}
