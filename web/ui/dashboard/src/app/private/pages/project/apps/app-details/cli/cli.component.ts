import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
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
	imports: [CommonModule, CardComponent, ButtonComponent, EmptyStateComponent, TagComponent, SkeletonLoaderComponent, ModalComponent, StatusColorModule, DeleteModalModule],
	templateUrl: './cli.component.html',
	styleUrls: ['./cli.component.scss']
})
export class CliComponent implements OnInit {
	tabs: ['api keys', 'devices'] = ['api keys', 'devices'];
	activeTab: 'api keys' | 'devices' = 'api keys';
	isGeneratingNewKey = false;
	isFetchingDevices = false;
	isFetchingApiKeys = false;
	showApiKey = false;
	showRevokeApiModal = false;
	showSecretCopyText = false;
	isRevokingApiKey = false;
	apiKey!: string;
	apiKeys!: API_KEY[];
	selectedApiKey!: API_KEY;
	devices!: DEVICE[];
	loaderIndex: number[] = [0, 1, 2];
	appId: string = this.route.snapshot.params.id;

	constructor(private appDetailsService: AppDetailsService, private route: ActivatedRoute, private generalService: GeneralService) {}

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

		const payload = {
			key_type: 'cli'
		};

		try {
			const response = await this.appDetailsService.generateKey({ appId: this.appId, body: payload });
			this.apiKey = response.data.key;
			this.showApiKey = true;
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

	copyKey(key: string) {
		const text = key;
		const el = document.createElement('textarea');
		el.value = text;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		this.showSecretCopyText = true;
		setTimeout(() => {
			this.showSecretCopyText = false;
		}, 3000);

		document.body.removeChild(el);
	}

	getKeyStatus(expiryDate: Date): string {
		const currentDate = new Date();
		if (currentDate > new Date(expiryDate)) return 'disabled';
		return 'active';
	}

	toggleActiveTab(tab: 'api keys' | 'devices') {
		this.activeTab = tab;
	}
}
