import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { MonacoComponent } from '../monaco/monaco.component';
import { DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { CreateSubscriptionService } from '../create-subscription/create-subscription.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrismModule } from '../prism/prism.module';
import { CreateSourceService } from '../create-source/create-source.service';

@Component({
	selector: 'convoy-create-transform-function',
	standalone: true,
	imports: [CommonModule, CardComponent, ReactiveFormsModule, ButtonComponent, MonacoComponent, DialogHeaderComponent, PrismModule],
	templateUrl: './create-transform-function.component.html',
	styleUrls: ['./create-transform-function.component.scss']
})
export class CreateTransformFunctionComponent implements OnInit {
	@Output('close') close: EventEmitter<any> = new EventEmitter();
	@ViewChild('payloadEditor') payloadEditor!: MonacoComponent;
	@ViewChild('headerPayloadEditor') headerPayloadEditor!: MonacoComponent;
	@ViewChild('functionEditor') functionEditor!: MonacoComponent;
	@ViewChild('headerFunctionEditor') headerFunctionEditor!: MonacoComponent;
	@Input('transformFunction') transformFunction: any;
	@Input('headerTransformFunction') headerTransformFunction: any;

	@Input('transformType') transformType: 'source' | 'subscription' = 'subscription';
	@Output('updatedTransformFunction') updatedTransformFunction: EventEmitter<any> = new EventEmitter();
	tabs = ['output', 'diff'];
	activeTab = 'output';
	transformForm: FormGroup = this.formBuilder.group({
		payload: [null],
		function: [null],
		type: ['']
	});
	isTransformFunctionPassed = false;
	isTestingFunction = false;
	showConsole = true;
	logs = [];
	headerLogs = [];
	payload: any = {
		id: 'Sample-1',
		name: 'Sample 1',
		description: 'This is sample data #1'
	};
	headerPayload = {
		id: 'Sample-1',
		name: 'Sample 1',
		description: 'This is sample data #1'
	};
	setFunction = `/* 1. While you can write multiple functions, the main function
    called for your transformation is the transform function.

2. The only argument acceptable in the transform function is
    the payload data.

3. The transform method must return a value.

4. Console logs lust be written like this
    console.log('%j', logged_item) to get printed in the log below. */

function transform(payload) {
    // Transform function here
    return payload;
}`;
	headerSetFunction = `/* 1. While you can write multiple functions, the main function
called for your transformation is the transform function.

2. The only argument acceptable in the transform function is
the payload data.

3. The transform method must return a value.

4. Console logs lust be written like this
console.log('%j', logged_item) to get printed in the log below. */

function transform(payload) {
// Transform function here
return payload;
}`;

	output: any;
	headerOutput: any;
	eventTabs: ['body', 'header'] = ['body', 'header'];
	eventActiveTab: 'body' | 'header' = 'body';

	constructor(private createSubscriptionService: CreateSubscriptionService, private createSourceService: CreateSourceService, public generalService: GeneralService, private formBuilder: FormBuilder) {}

	ngOnInit(): void {
		this.checkForExistingData();
	}

	async testTransformFunction() {
		this.isTransformFunctionPassed = false;
		this.isTestingFunction = true;

		this.payload = this.generalService.convertStringToJson(this.payloadEditor.getValue());
		this.headerPayload = this.generalService.convertStringToJson(this.headerPayloadEditor.getValue());

		this.transformForm.patchValue({
			payload: this.eventActiveTab === 'body' ? this.payload : this.headerPayload,
			function: this.eventActiveTab === 'body' ? this.functionEditor.getValue() : this.headerFunctionEditor.getValue(),
			type: this.eventActiveTab === 'body' ? 'body' : 'header'
		});

		try {
			const response = this.transformType === 'subscription' ? await this.createSubscriptionService.testTransformFunction(this.transformForm.value) : await this.createSourceService.testTransformFunction(this.transformForm.value);

			this.generalService.showNotification({ message: response.message, style: 'success' });

			this.eventActiveTab === 'body' ? (this.output = response.data.payload) : (this.headerOutput = response.data.payload);

			this.eventActiveTab === 'body' ? (this.logs = response.data.log.reverse()) : (this.headerLogs = response.data.log.reverse());

			if (this.logs.length > 0 || this.headerLogs.length > 0) this.showConsole = true;

			this.isTransformFunctionPassed = true;
			this.isTestingFunction = false;
		} catch (error) {
			this.isTestingFunction = false;
			this.isTransformFunctionPassed = false;
		}
	}

	async saveFunction() {
		await this.testTransformFunction();

		if (this.isTransformFunctionPassed) {
			if (this.payloadEditor?.getValue()) localStorage.setItem(this.transformType === 'subscription' ? 'PAYLOAD' : 'SOURCE_PAYLOAD', this.payloadEditor.getValue());
			if (this.headerPayloadEditor?.getValue()) localStorage.setItem('HEADER_PAYLOAD', this.headerPayloadEditor.getValue());

			if (this.functionEditor?.getValue()) localStorage.setItem(this.transformType === 'subscription' ? 'FUNCTION' : 'SOURCE_FUNCTION', this.functionEditor.getValue());
			if (this.headerFunctionEditor?.getValue()) localStorage.setItem('HEADER_FUNCTION', this.headerFunctionEditor.getValue());

			const subscriptionTransformFunction = this.functionEditor.getValue();
			const sourceTransform = {
				header: this.headerFunctionEditor.getValue(),
				body: this.functionEditor.getValue()
			};

			if (this.transformType === 'source') this.updatedTransformFunction.emit(sourceTransform);
			else this.updatedTransformFunction.emit(subscriptionTransformFunction);
		}
	}

	checkForExistingData() {
		if (this.transformType === 'source' && !this.transformFunction)
			this.setFunction = `/*  1. While you can write multiple functions, the main function called for your transformation is the transform function.

2. The only argument acceptable in the transform function is the payload data.

3. The transform method must return a value.

4. Console logs lust be written like this console.log('%j', logged_item) to get printed in the log below.

5. The output payload from the function should be in this format
    {
        "owner_id": "string, optional",
        "event_type": "string, required",
        "data": "object, required",
        "custom_headers": "object, optional",
        "idempotency_key": "string, optional"
        "endpoint_id": "string, depends",
    }

6. The endpoint_id field is only required when sending event to a single endpoint. */

function transform(payload) {
    // Transform function here
    return payload;
}`;
		if (this.transformFunction) this.setFunction = this.transformFunction;
		if (this.headerTransformFunction) this.headerSetFunction = this.headerTransformFunction;

		// const payload = this.transformType === 'subscription' ? localStorage.getItem('PAYLOAD') : this.eventActiveTab === 'body' ? localStorage.getItem('SOURCE_PAYLOAD') : localStorage.getItem('HEADER_PAYLOAD');
		// const headerPayload = localStorage.getItem('HEADER_PAYLOAD');
		// if (headerPayload && headerPayload !== 'undefined') this.headerPayload = JSON.parse(headerPayload);
		// if (payload && payload !== 'undefined') this.payload = JSON.parse(payload);

		// const updatedTransformFunction = this.transformType === 'subscription' ? localStorage.getItem('FUNCTION') : this.eventActiveTab === 'body' ? localStorage.getItem('SOURCE_FUNCTION') : localStorage.getItem('HEADER_FUNCTION');
		// const headerFunction = localStorage.getItem('HEADER_FUNCTION');
		// if (headerFunction && headerFunction !== 'undefined' && !this.headerTransformFunction) this.headerSetFunction = headerFunction;
		// if (updatedTransformFunction && updatedTransformFunction !== 'undefined' && !this.transformFunction) this.setFunction = updatedTransformFunction;
	}

	parseLog(log: string) {
		return JSON.parse(log);
	}
}
