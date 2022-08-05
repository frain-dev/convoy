import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { API_KEY, DEVICE } from 'src/app/models/app.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppDetailsService } from '../app-details.service';

@Component({
	selector: 'convoy-cli',
	templateUrl: './cli.component.html',
	styleUrls: ['./cli.component.scss']
})
export class CliComponent implements OnInit {
	tabs: ['api keys', 'devices'] = ['api keys', 'devices'];
	activeTab: 'api keys' | 'devices' = 'api keys';
	isGeneratingNewKey = false;
	isFetchingDevices = false;
	isFetchingApiKeys = false;
	apiKeys!: API_KEY[];
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
            this.getApiKeys()
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isGeneratingNewKey = false;
		} catch {
			this.isGeneratingNewKey = false;
			return;
		}
	}

	toggleActiveTab(tab: 'api keys' | 'devices') {
		this.activeTab = tab;
	}
}
