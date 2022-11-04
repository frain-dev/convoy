import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CreateSubscriptionService } from '../create-subscription/create-subscription.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { MonacoComponent } from '../monaco/monaco.component';

@Component({
	selector: 'convoy-create-subscription-filter',
	standalone: true,
	imports: [CommonModule, CardComponent, ReactiveFormsModule, ButtonComponent, MonacoComponent],
	templateUrl: './create-subscription-filter.component.html',
	styleUrls: ['./create-subscription-filter.component.scss']
})
export class CreateSubscriptionFilterComponent implements OnInit {
	@ViewChild('requestEditor') requestEditor!: MonacoComponent;
	@ViewChild('schemaEditor') schemaEditor!: MonacoComponent;
	@Input('action') action: 'update' | 'create' = 'create';
	@Input('schema') schema: any;
	@Output('filterSchema') filterSchema: EventEmitter<any> = new EventEmitter();
	subscriptionFilterForm: FormGroup = this.formBuilder.group({
		request: [null],
		schema: [null]
	});
	constructor(private formBuilder: FormBuilder, private createSubscriptionService: CreateSubscriptionService, private generalService: GeneralService) {}

	ngOnInit() {}

	async testFilter() {
		this.subscriptionFilterForm.value.request = this.convertStringToJson(this.requestEditor.getValue());
		this.subscriptionFilterForm.value.schema = this.convertStringToJson(this.schemaEditor.getValue());
		try {
			const response = await this.createSubscriptionService.testSubsriptionFilter(this.subscriptionFilterForm.value);
			this.generalService.showNotification({ message: response.message, style: 'success' });
		} catch (error) {
			return error;
		}
	}

	convertStringToJson(str: string) {
		try {
			const jsonObject = JSON.parse(str);
			return jsonObject;
		} catch {
			this.generalService.showNotification({ message: 'Data is not entered in correct JSON format', style: 'error' });
			return false;
		}
	}
}
