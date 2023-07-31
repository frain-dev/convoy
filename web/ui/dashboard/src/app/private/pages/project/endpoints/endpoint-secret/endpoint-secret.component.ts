import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { ENDPOINT, SECRET } from 'src/app/models/endpoint.model';
import { EndpointsService } from '../endpoints.service';

@Component({
	selector: 'convoy-endpoint-secret',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, ButtonComponent, SelectComponent, CopyButtonComponent],
	templateUrl: './endpoint-secret.component.html',
	styleUrls: ['./endpoint-secret.component.scss']
})
export class EndpointSecretComponent implements OnInit {
	@Input('endpointDetails') endpointDetails?: ENDPOINT;
	@Output() expireCurrentSecret = new EventEmitter<any>();
	@Output() closeSecretModal = new EventEmitter<any>();
	expireSecretForm: FormGroup = this.formBuilder.group({
		expiration: ['', Validators.required]
	});
	expirationDates = [
		{ name: '1 hour', uid: 1 },
		{ name: '2 hour', uid: 2 },
		{ name: '4 hour', uid: 4 },
		{ name: '8 hour', uid: 8 },
		{ name: '12 hour', uid: 12 },
		{ name: '16 hour', uid: 16 },
		{ name: '20 hour', uid: 20 },
		{ name: '24 hour', uid: 24 }
	];
	showExpireSecret = false;
	isExpiringSecret = false;

	constructor(private formBuilder: FormBuilder, private endpointService: EndpointsService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async expireSecret() {
		if (this.expireSecretForm.invalid) {
			this.expireSecretForm.markAllAsTouched();
			return;
		}

		this.expireSecretForm.value.expiration = parseInt(this.expireSecretForm.value.expiration);
		this.isExpiringSecret = true;
		try {
			const response = await this.endpointService.expireSecret({ endpointId: this.endpointDetails?.uid || '', body: this.expireSecretForm.value });
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.isExpiringSecret = false;
			this.expireCurrentSecret.emit();
			this.showExpireSecret = false;
		} catch {
			this.isExpiringSecret = false;
		}
	}

	get endpointSecret(): SECRET | undefined {
		return this.endpointDetails?.secrets?.find(secret => !secret.expires_at);
	}
}
