import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { ENDPOINT } from 'src/app/models/app.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppDetailsService } from '../app-details.service';

@Component({
	selector: 'app-create-endpoint',
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreateEndpointComponent implements OnInit {
	@Input() appId!: string;
	@Input() selectedEndpoint?: ENDPOINT;
	@Output() onAction = new EventEmitter<any>();
	savingEndpoint = false;
	addNewEndpointForm: FormGroup = this.formBuilder.group({
		url: ['', Validators.required],
		http_timeout: [null],
		description: ['', Validators.required]
	});
	token: string = this.route.snapshot.params.token;

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private appDetailsService: AppDetailsService, private route: ActivatedRoute) {}

	ngOnInit() {
		if (this.selectedEndpoint) this.updateEndpointForm();
	}

	async saveEndpoint() {
		if (this.addNewEndpointForm.invalid) return this.addNewEndpointForm.markAsTouched();
		this.savingEndpoint = true;

		try {
			const response = this.selectedEndpoint
				? await this.appDetailsService.editEndpoint({ appId: this.appId, endpointId: this.selectedEndpoint?.uid || '', body: this.addNewEndpointForm.value, token: this.token })
				: await this.appDetailsService.addNewEndpoint({ appId: this.appId, body: this.addNewEndpointForm.value, token: this.token });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.onAction.emit({ action: 'savedEndpoint' });
			this.addNewEndpointForm.reset();
			this.savingEndpoint = false;
			return;
		} catch {
			this.savingEndpoint = false;
			return;
		}
	}

	updateEndpointForm() {
		if (this.selectedEndpoint) {
			this.addNewEndpointForm.patchValue({
				url: this.selectedEndpoint.target_url,
				description: this.selectedEndpoint.description,
				http_timeout: this.selectedEndpoint.http_timeout,
				rate_limit: this.selectedEndpoint.rate_limit,
				rate_limit_duration: this.selectedEndpoint.rate_limit_duration
			});
		}
	}

	cancel() {
		this.onAction.emit({ action: 'discardEndpointForm' });
	}
}
