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
	@Input('schema') schema!: string;
	@Input('payload') payload!: string;
	@Output('filterSchema') filterSchema: EventEmitter<any> = new EventEmitter();
	subscriptionFilterForm: FormGroup = this.formBuilder.group({
		request: [null],
		schema: [null]
	});

	constructor(private formBuilder: FormBuilder, private createSubscriptionService: CreateSubscriptionService, private generalService: GeneralService) {}

	ngOnInit() {
		// trying to preserve the code sample indentation
		if (!this.schema) {
			this.schema = `{
    "tconclusion": "success"
}`;
		}
		if (!this.payload) {
			this.payload = ` {
    "tconclusion": "success",
    "started_at": "2022-11-04T13:58:20Z",
    "completed_at": "2022-11-04T14:06:13Z",
    "name": "test (1.17.x, ubuntu-latest, 5.0, 6.2.6)",
    "status": "completed",
    "conclusion": "success",
    "number": 1
}`;
		}
	}

	async testFilter() {
		this.subscriptionFilterForm.value.request = this.convertStringToJson(this.requestEditor.getValue());
		this.subscriptionFilterForm.value.schema = this.convertStringToJson(this.schemaEditor.getValue());
		try {
			const response = await this.createSubscriptionService.testSubsriptionFilter(this.subscriptionFilterForm.value);
			const testResponse = `The sample data was ${!response.data ? 'not' : ''} accepted by the filter`;
			this.generalService.showNotification({ message: testResponse, style: !response.data ? 'error' : 'success' });
		} catch (error) {
			return error;
		}
	}

	setSubscriptionFilter() {
		const filter = this.convertStringToJson(this.schemaEditor.getValue());
		this.filterSchema.emit(filter);
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
