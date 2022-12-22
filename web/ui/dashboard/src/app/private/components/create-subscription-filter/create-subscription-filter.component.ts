import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CreateSubscriptionService } from '../create-subscription/create-subscription.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { MonacoComponent } from '../monaco/monaco.component';
import { ActivatedRoute } from '@angular/router';

@Component({
	selector: 'convoy-create-subscription-filter',
	standalone: true,
	imports: [CommonModule, CardComponent, ReactiveFormsModule, ButtonComponent, MonacoComponent],
	templateUrl: './create-subscription-filter.component.html',
	styleUrls: ['./create-subscription-filter.component.scss']
})
export class CreateSubscriptionFilterComponent implements OnInit {
	@ViewChild('requestHeaderEditor') requestHeaderEditor!: MonacoComponent;
	@ViewChild('headerSchemaEditor') headerSchemaEditor!: MonacoComponent;
	@ViewChild('requestEditor') requestEditor!: MonacoComponent;
	@ViewChild('schemaEditor') schemaEditor!: MonacoComponent;
	@Input('action') action: 'update' | 'create' = 'create';
	@Input('schema') schema?: any;
	@Output('filterSchema') filterSchema: EventEmitter<any> = new EventEmitter();
	tabs: ['body', 'header'] = ['body', 'header'];
	activeTab: 'body' | 'header' = 'body';
	subscriptionFilterForm: FormGroup = this.formBuilder.group({
		request: this.formBuilder.group({
			header: [null],
			body: [null]
		}),
		schema: this.formBuilder.group({
			header: [null],
			body: [null]
		})
	});
	isFilterTestPassed = false;
	payload: any;
	header: any;
	token: string = this.route.snapshot.queryParams.token;

	constructor(private formBuilder: FormBuilder, private createSubscriptionService: CreateSubscriptionService, private generalService: GeneralService, private route: ActivatedRoute) {}

	ngOnInit() {
		this.checkForExistingData();
	}

	toggleActiveTab(tab: 'body' | 'header') {
		this.activeTab = tab;
	}

	async testFilter() {
		this.isFilterTestPassed = false;
		this.subscriptionFilterForm.patchValue({
			request: {
				header: this.requestHeaderEditor?.getValue() ? this.convertStringToJson(this.requestHeaderEditor.getValue()) : null,
				body: this.requestEditor?.getValue() ? this.convertStringToJson(this.requestEditor.getValue()) : null
			},
			schema: {
				header: this.headerSchemaEditor?.getValue() ? this.convertStringToJson(this.headerSchemaEditor.getValue()) : null,
				body: this.schemaEditor?.getValue() ? this.convertStringToJson(this.schemaEditor.getValue()) : null
			}
		});

		try {
			const response = await this.createSubscriptionService.testSubsriptionFilter(this.subscriptionFilterForm.value, this.token);
			const testResponse = `The sample data was ${!response.data ? 'not' : ''} accepted by the filter`;
			this.generalService.showNotification({ message: testResponse, style: !response.data ? 'error' : 'success' });
			this.isFilterTestPassed = !!response.data;
		} catch (error) {
			this.isFilterTestPassed = false;
			return error;
		}
	}

	async setSubscriptionFilter() {
		await this.testFilter();

		if (this.isFilterTestPassed) {
			if (this.requestEditor?.getValue()) localStorage.setItem('EVENT_DATA', this.requestEditor.getValue());
			if (this.requestHeaderEditor?.getValue()) localStorage.setItem('EVENT_HEADERS', this.requestHeaderEditor.getValue());
			const filter = {
				bodySchema: this.schemaEditor?.getValue() ? this.convertStringToJson(this.schemaEditor?.getValue()) : null,
				headerSchema: this.headerSchemaEditor?.getValue() ? this.convertStringToJson(this.headerSchemaEditor?.getValue()) : null
			};
			this.filterSchema.emit(filter);
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

	checkForExistingData() {
		const eventData = localStorage.getItem('EVENT_DATA');
		const eventHeaders = localStorage.getItem('EVENT_HEADERS');
		if (eventData && eventData !== 'undefined') this.payload = JSON.parse(eventData);
		if (eventHeaders && eventHeaders !== 'undefined') this.header = JSON.parse(eventHeaders);
	}
}
