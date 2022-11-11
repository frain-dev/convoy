import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { ActivatedRoute } from '@angular/router';
import { API_KEY } from 'src/app/models/app.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { InputDirective, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { CliKeysService } from './cli-keys.service';

@Component({
	selector: 'convoy-cli-keys',
	standalone: true,
	imports: [
		CommonModule,
		ReactiveFormsModule,
		CardComponent,
		ModalComponent,
		ButtonComponent,
		SkeletonLoaderComponent,
		EmptyStateComponent,
		TagComponent,
		StatusColorModule,

		CopyButtonComponent,
		SelectComponent,
		DeleteModalComponent,
		InputFieldDirective,
		InputDirective,
		LabelComponent
	],
	templateUrl: './cli-keys.component.html',
	styleUrls: ['./cli-keys.component.scss']
})
export class CliKeysComponent implements OnInit {
	@Output() cliError = new EventEmitter<any>();
	isFetchingApiKeys = false;
	showApiKey = false;
	showRevokeApiModal = false;
	isRevokingApiKey = false;
	generateKeyModal = false;
	isGeneratingNewKey = false;
	isloadingAppPortalAppDetails = false;
	showError = false;
	apiKey!: string;
	apiKeys!: API_KEY[];
	selectedApiKey?: API_KEY;
	loaderIndex: number[] = [0, 1, 2];
	appId: string = this.route.snapshot.params.id;
	token: string = this.route.snapshot.params.token;
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

	constructor(private route: ActivatedRoute, private generalService: GeneralService, private formBuilder: FormBuilder, private cliKeyService: CliKeysService) {}

	ngOnInit() {
		this.token ? this.getAppPortalApp() : this.getApiKeys();
	}

	async getAppPortalApp() {
		this.cliError.emit(false);
		this.showError = false;
		this.isloadingAppPortalAppDetails = true;

		try {
			const app = await this.cliKeyService.getAppPortalApp(this.token);
			this.appId = app.data.uid;
			this.getApiKeys();
			return;
		} catch (error) {
			this.cliError.emit(true);
			this.showError = true;
			this.isloadingAppPortalAppDetails = false;
			return error;
		}
	}

	async getApiKeys() {
		this.cliError.emit(false);
		this.showError = false;
		this.isFetchingApiKeys = true;
		try {
			const response = await this.cliKeyService.getApiKeys({ appId: this.appId, token: this.token });
			this.apiKeys = response.data.content;
			this.isFetchingApiKeys = false;
		} catch {
			this.cliError.emit(true);
			this.showError = true;
			this.isFetchingApiKeys = false;
			return;
		}
	}

	async generateNewKey() {
		this.isGeneratingNewKey = true;
		this.generateKeyForm.value.expiration = parseInt(this.generateKeyForm.value.expiration);
		try {
			const response = await this.cliKeyService.generateKey({ appId: this.appId, body: this.generateKeyForm.value, token: this.token });
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
		if (!this.selectedApiKey) return;

		this.isRevokingApiKey = true;
		try {
			const response = await this.cliKeyService.revokeApiKey({ appId: this.selectedApiKey?.role.app, keyId: this.selectedApiKey?.uid, token: this.token });
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
}
