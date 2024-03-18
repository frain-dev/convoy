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
	@ViewChild('functionEditor') functionEditor!: MonacoComponent;
	@Input('transformFunction') transformFunction: any;
	@Input('transformType') transformType: 'source' | 'subscription' = 'subscription';
	@Output('updatedTransformFunction') updatedTransformFunction: EventEmitter<any> = new EventEmitter();
	tabs = ['output', 'diff'];
	activeTab = 'output';
	transformForm: FormGroup = this.formBuilder.group({
		payload: [null],
		function: [null]
	});
	isTransformFunctionPassed = false;
	isTestingFunction = false;
	showConsole = true;
	logs: any;
	payload: any = {
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
}`;
	output: any;

	constructor(private createSubscriptionService: CreateSubscriptionService, private createSourceService: CreateSourceService, public generalService: GeneralService, private formBuilder: FormBuilder) {}

	ngOnInit(): void {
		this.checkForExistingData();
	}

	async testTransformFunction() {
		this.isTransformFunctionPassed = false;
		this.isTestingFunction = true;
		this.payload = this.generalService.convertStringToJson(this.payloadEditor.getValue());
		this.transformForm.patchValue({
			payload: this.generalService.convertStringToJson(this.payloadEditor.getValue()),
			function: this.functionEditor.getValue()
		});

		try {
			const response = this.transformType === 'subscription' ? await this.createSubscriptionService.testTransformFunction(this.transformForm.value) : await this.createSourceService.testTransformFunction(this.transformForm.value);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.output = response.data.payload;
			this.logs = response.data.log.reverse();
			if (this.logs.length > 0) this.showConsole = true;
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
			if (this.functionEditor?.getValue()) localStorage.setItem(this.transformType === 'subscription' ? 'FUNCTION' : 'SOURCE_PAYLOAD', this.functionEditor.getValue());
			const updatedTransformFunction = this.functionEditor.getValue();
			this.updatedTransformFunction.emit(updatedTransformFunction);
		}
	}

	checkForExistingData() {
		if (this.transformFunction) this.setFunction = this.transformFunction;

		const payload = this.transformType === 'subscription' ? localStorage.getItem('PAYLOAD') : localStorage.getItem('SOURCE_PAYLOAD');
		const updatedTransformFunction = this.transformType === 'subscription' ? localStorage.getItem('FUNCTION') : localStorage.getItem('SOURCE_FUNCTION');
		if (payload && payload !== 'undefined') this.payload = JSON.parse(payload);
		if (updatedTransformFunction && updatedTransformFunction !== 'undefined' && !this.transformFunction) this.setFunction = updatedTransformFunction;
	}

	parseLog(log: string) {
		return JSON.parse(log);
	}
}
