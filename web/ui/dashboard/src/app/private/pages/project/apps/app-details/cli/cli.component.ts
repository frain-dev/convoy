import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { API_KEY, DEVICE } from 'src/app/models/app.model';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DeleteModalModule } from 'src/app/private/components/delete-modal/delete-modal.module';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppDetailsService } from '../app-details.service';

@Component({
	selector: 'convoy-cli',
	standalone: true,
	imports: [CommonModule, CardComponent, ButtonComponent, EmptyStateComponent, TagComponent, SkeletonLoaderComponent, ModalComponent, StatusColorModule, DeleteModalModule, ReactiveFormsModule, InputComponent, SelectComponent, CopyButtonComponent],
	templateUrl: './cli.component.html',
	styleUrls: ['./cli.component.scss']
})
export class CliComponent implements OnInit {
	tabs: ['cli keys', 'devices'] = ['cli keys', 'devices'];
	activeTab: 'cli keys' | 'devices' = 'cli keys';
	isGeneratingNewKey = false;
	isFetchingDevices = false;
	isFetchingApiKeys = false;
	showApiKey = false;
	showRevokeApiModal = false;
	isRevokingApiKey = false;
	generateKeyModal = false;
	apiKey!: string;
	apiKeys!: API_KEY[];
	selectedApiKey!: API_KEY;
	devices!: DEVICE[];
	loaderIndex: number[] = [0, 1, 2];
	appId: string = this.route.snapshot.params.id;
	expirationDates = [
		{ name: '7 days', uid: 7 },
		{ name: '14 days', uid: 14 },
		{ name: '30 days', uid: 30 },
		{ name: '90 days', uid: 90 }
	];
	generateKeyForm: FormGroup = this.formBuilder.group({
		name: [''],
		expiration: [''],
		key_type: ['cli']
	});

	constructor(private appDetailsService: AppDetailsService, private route: ActivatedRoute, private generalService: GeneralService, private formBuilder: FormBuilder) {}

	ngOnInit() {
		this.getDevices();
		this.getApiKeys();
	}

	async getDevices() {
		this.isFetchingDevices = true;
		try {
			const response = await this.appDetailsService.getAppDevices(this.appId);
			this.devices = response.data.content;
			this.isFetchingDevices = false;
		} catch {
			this.isFetchingDevices = false;
			return;
		}
	}

	async getApiKeys() {
		this.isFetchingApiKeys = true;
		try {
			const response = await this.appDetailsService.getApiKeys(this.appId);
			this.apiKeys = response.data.content;
			this.isFetchingApiKeys = false;
		} catch {
			this.isFetchingApiKeys = false;
			return;
		}
	}

	async generateNewKey() {
		this.isGeneratingNewKey = true;
		this.generateKeyForm.value.expiration = parseInt(this.generateKeyForm.value.expiration);
		try {
			const response = await this.appDetailsService.generateKey({ appId: this.appId, body: this.generateKeyForm.value });
			this.apiKey = response.data.key;
			this.generateKeyModal = false;
			this.showApiKey = true;
			this.generateKeyForm.reset();
			this.generateKeyForm.patchValue({
				key_type: 'cli'
			});
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isGeneratingNewKey = false;
		} catch {
			this.isGeneratingNewKey = false;
			return;
		}
	}

	async revokeApiKey() {
		this.isRevokingApiKey = true;
		try {
			const response = await this.appDetailsService.revokeApiKey({ appId: this.selectedApiKey?.role.app, keyId: this.selectedApiKey?.uid });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRevokingApiKey = false;
			this.showRevokeApiModal = false;
			this.getApiKeys();
		} catch {
			this.isRevokingApiKey = false;
		}
	}

	getKeyStatus(expiryDate: Date): string {
		const currentDate = new Date();
		if (currentDate > new Date(expiryDate)) return 'disabled';
		return 'active';
	}

	toggleActiveTab(tab: 'cli keys' | 'devices') {
		this.activeTab = tab;
	}
}
