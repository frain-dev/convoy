import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { format } from 'date-fns';
import { PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AccountService } from '../account.service';

@Component({
	selector: 'personal-settings',
	templateUrl: './personal-settings.component.html',
	styleUrls: ['./personal-settings.component.scss']
})
export class PersonalSettingsComponent implements OnInit {
	@ViewChild('settingsDialog', { static: true }) settingsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('tokenDialog', { static: true }) tokenDialog!: ElementRef<HTMLDialogElement>;

	showCreateNewTokenForm = false;
	isFetchingKeys = false;
	isGeneratingNewKey = false;
	showRevokeKeyModal = false;
	isRevokingKey = false;
	selectedKey!: any;
	userId!: string;
	accessKey!: string;
	loaderIndex: number[] = [0, 1, 2];
	personalAccessKeys?: { content: any; pagination: PAGINATION };
	expirationDates = [
		{ name: '7 days', uid: 7 },
		{ name: '14 days', uid: 14 },
		{ name: '30 days', uid: 30 },
		{ name: '90 days', uid: 90 }
	];
	generateKeyForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		expiration: [null]
	});

	constructor(private formBuilder: FormBuilder, private accountService: AccountService, private generalService: GeneralService, private router: Router, private route: ActivatedRoute) {}

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
		if (this.generateKeyForm.invalid) {
			this.generateKeyForm.markAllAsTouched();
			return;
		}
		this.generateKeyForm.value.expiration = parseInt(this.generateKeyForm.value.expiration);
		this.isGeneratingNewKey = true;
		try {
			const response = await this.accountService.generatePersonalKey(this.userId, this.generateKeyForm.value);
			this.accessKey = response.data.key;
			this.showCreateNewTokenForm = false;
			this.tokenDialog.nativeElement.showModal();
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
			const response = await this.accountService.fetchPersonalKeys({ userId: this.userId, page: page });
			this.personalAccessKeys = response.data;
			this.isFetchingKeys = false;
		} catch {
			this.isFetchingKeys = false;
		}
	}

	async revokeKey() {
		this.isRevokingKey = true;
		try {
			const response = await this.accountService.revokeKey({ userId: this.userId, keyId: this.selectedKey?.uid });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRevokingKey = false;
			this.settingsDialog.nativeElement.close();
			this.fetchPersonalKeys();
		} catch {
			this.isRevokingKey = false;
		}
	}

	getSelectedDate(date?: any) {
		const selectedDate = `${format(date, 'yyyy-MM-dd')}T11:59:59Z`;
		this.generateKeyForm.patchValue({
			expires_at: selectedDate
		});
	}

	getKeyStatus(expiryDate: Date): string {
		const currentDate = new Date();
		if (currentDate > new Date(expiryDate)) return 'disabled';
		return 'active';
	}
}
