import { Component, EventEmitter, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { MonacoComponent } from '../monaco/monaco.component';
import { DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { CreateSubscriptionService } from '../create-subscription/create-subscription.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-create-transform-function',
	standalone: true,
	imports: [CommonModule, CardComponent, ReactiveFormsModule, ButtonComponent, MonacoComponent, DialogHeaderComponent],
	templateUrl: './create-transform-function.component.html',
	styleUrls: ['./create-transform-function.component.scss']
})
export class CreateTransformFunctionComponent implements OnInit {
	@Output('close') close: EventEmitter<any> = new EventEmitter();
	@ViewChild('payloadEditor') payloadEditor!: MonacoComponent;
	@ViewChild('functionEditor') functionEditor!: MonacoComponent;
	@ViewChild('outputEditor') outputEditor!: MonacoComponent;
	@Output('subscriptionFunction') subscriptionFunction: EventEmitter<any> = new EventEmitter();
	transformForm: FormGroup = this.formBuilder.group({
		payload: [null],
		function: [null]
	});
	isTransformFunctionPassed = false;
	isTestingFunction = false;
	payload: any;
	setFunction: any;
	output: any;

	constructor(private createSubscriptionService: CreateSubscriptionService, private generalService: GeneralService, private formBuilder: FormBuilder) {}

	ngOnInit(): void {
		this.checkForExistingData();
	}

	async testTransformFunction() {
		this.isTransformFunctionPassed = false;
		this.isTestingFunction = true;
		this.transformForm.patchValue({
			payload: this.generalService.convertStringToJson(this.payloadEditor.getValue()),
			function: this.functionEditor.getValue()
		});

		try {
			const response = await this.createSubscriptionService.testTransformFunction(this.transformForm.value);
			this.output = response.data;
			this.isTransformFunctionPassed = true;
			this.isTestingFunction = false;
		} catch (error) {
			this.isTestingFunction = false;
			this.isTransformFunctionPassed = false;
			return error;
		}
	}

	async saveFunction() {
		await this.testTransformFunction();

		if (this.isTransformFunctionPassed) {
			if (this.payloadEditor?.getValue()) localStorage.setItem('PAYLOAD', this.payloadEditor.getValue());
			if (this.functionEditor?.getValue()) localStorage.setItem('FUNCTION', this.functionEditor.getValue());
			const subscriptionFunction = this.functionEditor.getValue();
			this.subscriptionFunction.emit(subscriptionFunction);
		}
	}

	checkForExistingData() {
		const payload = localStorage.getItem('PAYLOAD');
		const subscriptionFunction = localStorage.getItem('FUNCTION');
		if (payload && payload !== 'undefined') this.payload = JSON.parse(payload);
		if (subscriptionFunction && subscriptionFunction !== 'undefined') this.setFunction = subscriptionFunction;
	}
}
